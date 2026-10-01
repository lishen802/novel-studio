package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func (s *Store) ListChapterRevisions(
	ctx context.Context,
	chapterID string,
) ([]revisiondomain.ChapterRevision, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chapter_id, revision, title, body, source, summary, created_at
		FROM chapter_revisions
		WHERE chapter_id = ?
		UNION ALL
		SELECT id, revision, title, body, revision_source, revision_summary, updated_at
		FROM chapters
		WHERE id = ?
		ORDER BY revision DESC
	`, chapterID, chapterID)
	if err != nil {
		return nil, databaseOperationError("list chapter revisions", err)
	}
	defer rows.Close()

	result := make([]revisiondomain.ChapterRevision, 0)
	for rows.Next() {
		var item revisiondomain.ChapterRevision
		var createdAt string
		if err := rows.Scan(
			&item.ChapterID,
			&item.Revision,
			&item.Title,
			&item.Body,
			&item.Source,
			&item.Summary,
			&createdAt,
		); err != nil {
			return nil, databaseOperationError("scan chapter revision", err)
		}
		item.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse chapter revision created_at: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate chapter revisions", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrChapterMissing, chapterID)
	}
	return result, nil
}

func (s *Store) RestoreChapterRevision(
	ctx context.Context,
	chapterID string,
	targetRevision int64,
	expectedRevision int64,
	restoredAt time.Time,
) (projectdomain.Chapter, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("begin chapter restore transaction", err)
	}
	defer tx.Rollback()

	current, err := scanChapter(tx.QueryRowContext(ctx, `
		SELECT id, project_id, title, position, body, revision, created_at, updated_at
		FROM chapters WHERE id = ?
	`, chapterID))
	if errors.Is(err, sql.ErrNoRows) {
		return projectdomain.Chapter{}, fmt.Errorf("%w: %s", ErrChapterMissing, chapterID)
	}
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("read chapter before restore", err)
	}
	if current.Revision != expectedRevision {
		return projectdomain.Chapter{}, projectdomain.ErrRevisionConflict
	}

	var targetTitle, targetBody string
	if targetRevision == current.Revision {
		targetTitle, targetBody = current.Title, current.Body
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT title, body
			FROM chapter_revisions
			WHERE chapter_id = ? AND revision = ?
		`, chapterID, targetRevision).Scan(&targetTitle, &targetBody)
		if errors.Is(err, sql.ErrNoRows) {
			return projectdomain.Chapter{}, fmt.Errorf("chapter revision does not exist: %s@%d", chapterID, targetRevision)
		}
		if err != nil {
			return projectdomain.Chapter{}, databaseOperationError("read chapter restore target", err)
		}
	}

	if err := requireCleanDraft(ctx, tx, current); err != nil {
		return projectdomain.Chapter{}, err
	}
	var currentSource, currentSummary string
	if err := tx.QueryRowContext(ctx, `
		SELECT revision_source, revision_summary FROM chapters WHERE id = ?
	`, chapterID).Scan(&currentSource, &currentSummary); err != nil {
		return projectdomain.Chapter{}, databaseOperationError("read current revision metadata", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO chapter_revisions(
			chapter_id, revision, title, body, source, summary, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, current.ID, current.Revision, current.Title, current.Body, currentSource, currentSummary, formatTime(restoredAt))
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("archive chapter before restore", err)
	}

	updated := current
	updated.Title = targetTitle
	updated.Body = targetBody
	updated.Revision++
	updated.UpdatedAt = restoredAt.UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE chapters
		SET title = ?, body = ?, revision = ?, revision_source = ?, revision_summary = ?, updated_at = ?
		WHERE id = ? AND revision = ?
	`, updated.Title, updated.Body, updated.Revision, revisiondomain.RevisionRestore,
		fmt.Sprintf("恢复自 Revision %d", targetRevision), formatTime(updated.UpdatedAt),
		updated.ID, expectedRevision)
	if err != nil {
		return projectdomain.Chapter{}, databaseOperationError("restore chapter revision", err)
	}
	if err := requireOneRow(result, "chapter", chapterID); err != nil {
		return projectdomain.Chapter{}, projectdomain.ErrRevisionConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM drafts WHERE chapter_id = ?`, chapterID); err != nil {
		return projectdomain.Chapter{}, databaseOperationError("clear draft after chapter restore", err)
	}
	if err := tx.Commit(); err != nil {
		return projectdomain.Chapter{}, databaseOperationError("commit chapter restore", err)
	}
	return updated, nil
}

func (s *Store) Backup(ctx context.Context, targetPath string) error {
	targetPath = filepath.Clean(targetPath)
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("snapshot target already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect snapshot target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	return backupDatabase(ctx, s.db, targetPath)
}
