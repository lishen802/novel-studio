package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func (s *Store) CreateChangeSet(ctx context.Context, changeSet revisiondomain.ChangeSet) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin change set transaction", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO change_sets(id, project_id, title, summary, created_by, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, changeSet.ID, changeSet.ProjectID, changeSet.Title, changeSet.Summary, changeSet.CreatedBy, changeSet.Status, formatTime(changeSet.CreatedAt), formatTime(changeSet.UpdatedAt))
	if err != nil {
		return databaseOperationError("create change set", err)
	}
	for _, operation := range changeSet.Operations {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO change_operations(
				id, change_set_id, kind, target_id, base_revision, before_value,
				after_value, reason, status, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, operation.ID, operation.ChangeSetID, operation.Kind, operation.TargetID, operation.BaseRevision, operation.BeforeValue, operation.AfterValue, operation.Reason, operation.Status, formatTime(operation.CreatedAt), formatTime(operation.UpdatedAt))
		if err != nil {
			return databaseOperationError("create change operation", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit change set transaction", err)
	}
	return nil
}

func (s *Store) ListChangeSets(ctx context.Context) ([]revisiondomain.ChangeSet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM change_sets ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, databaseOperationError("list change sets", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, databaseOperationError("scan change set id", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate change set ids", err)
	}
	if err := rows.Close(); err != nil {
		return nil, databaseOperationError("close change set id rows", err)
	}
	result := make([]revisiondomain.ChangeSet, 0, len(ids))
	for _, id := range ids {
		changeSet, err := loadChangeSet(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
		result = append(result, changeSet)
	}
	return result, nil
}

func (s *Store) ChangeSet(ctx context.Context, id string) (revisiondomain.ChangeSet, error) {
	return loadChangeSet(ctx, s.db, id)
}

func (s *Store) ApplyChangeSet(ctx context.Context, id string, appliedAt time.Time) (projectdomain.Chapter, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("begin apply change set transaction", err)
	}
	defer tx.Rollback()

	changeSet, err := loadChangeSet(ctx, tx, id)
	if err != nil {
		return projectdomain.Chapter{}, err
	}
	if len(changeSet.Operations) != 1 {
		return projectdomain.Chapter{}, revisiondomain.ErrInvalidTransition
	}
	operation := changeSet.Operations[0]
	if changeSet.Status == revisiondomain.ChangeSetApplied {
		return scanChapter(tx.QueryRowContext(ctx, `
			SELECT id, project_id, title, position, body, revision, created_at, updated_at
			FROM chapters WHERE id = ?
		`, operation.TargetID))
	}
	if changeSet.Status != revisiondomain.ChangeSetPending ||
		operation.Kind != revisiondomain.OperationReplaceChapterBody {
		return projectdomain.Chapter{}, revisiondomain.ErrInvalidTransition
	}

	current, err := scanChapter(tx.QueryRowContext(ctx, `
		SELECT id, project_id, title, position, body, revision, created_at, updated_at
		FROM chapters WHERE id = ?
	`, operation.TargetID))
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("read change target chapter", err)
	}
	if current.ProjectID != changeSet.ProjectID ||
		current.Revision != operation.BaseRevision ||
		current.Body != operation.BeforeValue {
		if err := setChangeSetStatus(ctx, tx, id, revisiondomain.ChangeSetConflicted, appliedAt); err != nil {
			return projectdomain.Chapter{}, err
		}
		if err := tx.Commit(); err != nil {
			return projectdomain.Chapter{}, databaseOperationError("commit conflicted change set", err)
		}
		return projectdomain.Chapter{}, projectdomain.ErrRevisionConflict
	}

	updated, err := current.Commit(operation.AfterValue, operation.BaseRevision, appliedAt)
	if err != nil {
		return projectdomain.Chapter{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO chapter_revisions(chapter_id, revision, title, body, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, current.ID, current.Revision, current.Title, current.Body, formatTime(updated.UpdatedAt))
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("archive chapter before change set", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE chapters SET body = ?, revision = ?, updated_at = ?
		WHERE id = ? AND revision = ?
	`, updated.Body, updated.Revision, formatTime(updated.UpdatedAt), updated.ID, current.Revision)
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("apply chapter change operation", err)
	}
	if err := requireOneRow(result, "chapter", updated.ID); err != nil {
		return projectdomain.Chapter{}, projectdomain.ErrRevisionConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM drafts WHERE chapter_id = ?`, updated.ID); err != nil {
		return projectdomain.Chapter{}, databaseOperationError("clear draft after change set", err)
	}
	if err := setChangeSetStatus(ctx, tx, id, revisiondomain.ChangeSetApplied, appliedAt); err != nil {
		return projectdomain.Chapter{}, err
	}
	if err := tx.Commit(); err != nil {
		return projectdomain.Chapter{}, databaseOperationError("commit applied change set", err)
	}
	return updated, nil
}

func (s *Store) RejectChangeSet(ctx context.Context, id string, rejectedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin reject change set transaction", err)
	}
	defer tx.Rollback()
	changeSet, err := loadChangeSet(ctx, tx, id)
	if err != nil {
		return err
	}
	if changeSet.Status == revisiondomain.ChangeSetRejected {
		return nil
	}
	if changeSet.Status != revisiondomain.ChangeSetPending &&
		changeSet.Status != revisiondomain.ChangeSetConflicted {
		return revisiondomain.ErrInvalidTransition
	}
	if err := setChangeSetStatus(ctx, tx, id, revisiondomain.ChangeSetRejected, rejectedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit rejected change set", err)
	}
	return nil
}

type changeSetQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadChangeSet(ctx context.Context, queryer changeSetQueryer, id string) (revisiondomain.ChangeSet, error) {
	var changeSet revisiondomain.ChangeSet
	var createdAtValue, updatedAtValue string
	err := queryer.QueryRowContext(ctx, `
		SELECT id, project_id, title, summary, created_by, status, created_at, updated_at
		FROM change_sets WHERE id = ?
	`, id).Scan(
		&changeSet.ID,
		&changeSet.ProjectID,
		&changeSet.Title,
		&changeSet.Summary,
		&changeSet.CreatedBy,
		&changeSet.Status,
		&createdAtValue,
		&updatedAtValue,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return revisiondomain.ChangeSet{}, fmt.Errorf("change set does not exist: %s", id)
	}
	if err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("read change set", err)
	}
	changeSet.CreatedAt, err = parseTime(createdAtValue)
	if err != nil {
		return revisiondomain.ChangeSet{}, fmt.Errorf("parse change set created_at: %w", err)
	}
	changeSet.UpdatedAt, err = parseTime(updatedAtValue)
	if err != nil {
		return revisiondomain.ChangeSet{}, fmt.Errorf("parse change set updated_at: %w", err)
	}

	rows, err := queryer.QueryContext(ctx, `
		SELECT id, change_set_id, kind, target_id, base_revision, before_value,
		       after_value, reason, status, created_at, updated_at
		FROM change_operations WHERE change_set_id = ?
		ORDER BY created_at, id
	`, id)
	if err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("list change operations", err)
	}
	defer rows.Close()
	changeSet.Operations = make([]revisiondomain.PatchOperation, 0)
	for rows.Next() {
		var operation revisiondomain.PatchOperation
		var operationCreatedAt, operationUpdatedAt string
		if err := rows.Scan(
			&operation.ID,
			&operation.ChangeSetID,
			&operation.Kind,
			&operation.TargetID,
			&operation.BaseRevision,
			&operation.BeforeValue,
			&operation.AfterValue,
			&operation.Reason,
			&operation.Status,
			&operationCreatedAt,
			&operationUpdatedAt,
		); err != nil {
			return revisiondomain.ChangeSet{}, databaseOperationError("scan change operation", err)
		}
		operation.CreatedAt, err = parseTime(operationCreatedAt)
		if err != nil {
			return revisiondomain.ChangeSet{}, fmt.Errorf("parse change operation created_at: %w", err)
		}
		operation.UpdatedAt, err = parseTime(operationUpdatedAt)
		if err != nil {
			return revisiondomain.ChangeSet{}, fmt.Errorf("parse change operation updated_at: %w", err)
		}
		changeSet.Operations = append(changeSet.Operations, operation)
	}
	if err := rows.Err(); err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("iterate change operations", err)
	}
	return changeSet, nil
}

func setChangeSetStatus(
	ctx context.Context,
	tx *sql.Tx,
	id string,
	status revisiondomain.ChangeSetStatus,
	updatedAt time.Time,
) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE change_sets SET status = ?, updated_at = ? WHERE id = ?
	`, status, formatTime(updatedAt), id); err != nil {
		return databaseOperationError("update change set status", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE change_operations SET status = ?, updated_at = ? WHERE change_set_id = ?
	`, status, formatTime(updatedAt), id); err != nil {
		return databaseOperationError("update change operation status", err)
	}
	return nil
}
