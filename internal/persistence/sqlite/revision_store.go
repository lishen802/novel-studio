package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

type outlineNodeValue struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Goal     string `json:"goal"`
	Conflict string `json:"conflict"`
	Outcome  string `json:"outcome"`
}

type characterValue struct {
	Name       string                      `json:"name"`
	Aliases    []string                    `json:"aliases"`
	Role       worlddomain.CharacterRole   `json:"role"`
	Summary    string                      `json:"summary"`
	Motivation string                      `json:"motivation"`
	Fear       string                      `json:"fear"`
	Secret     string                      `json:"secret"`
	Arc        string                      `json:"arc"`
	Status     worlddomain.CharacterStatus `json:"status"`
}

type factValue struct {
	Text              string                 `json:"text"`
	Category          string                 `json:"category"`
	Status            canondomain.FactStatus `json:"status"`
	Confidence        float64                `json:"confidence"`
	ValidFromEventID  string                 `json:"validFromEventId"`
	ValidUntilEventID string                 `json:"validUntilEventId"`
}

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
	for index, operation := range changeSet.Operations {
		evidenceJSON, err := encodeEvidence(operation.Evidence)
		if err != nil {
			return databaseOperationError("encode change operation evidence", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO change_operations(
				id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
				before_value, after_value, reason, evidence, status, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, operation.ID, operation.ChangeSetID, operation.Kind, index+1, operation.TargetID,
			operation.BaseRevision, operation.BaseFingerprint, operation.BeforeValue,
			operation.AfterValue, operation.Reason, evidenceJSON, operation.Status,
			formatTime(operation.CreatedAt), formatTime(operation.UpdatedAt))
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
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM change_sets ORDER BY created_at DESC, id DESC`)
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

func (s *Store) ApplyChangeSet(ctx context.Context, id string, appliedAt time.Time) (ports.ChangeSetApplyResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.ChangeSetApplyResult{}, databaseOperationError("begin apply change set transaction", err)
	}
	defer tx.Rollback()
	changeSet, err := loadChangeSet(ctx, tx, id)
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if changeSet.Status == revisiondomain.ChangeSetApplied && !hasPendingOperations(changeSet) {
		return appliedResult(ctx, tx, changeSet)
	}
	if changeSet.Status == revisiondomain.ChangeSetRejected ||
		changeSet.Status == revisiondomain.ChangeSetConflicted {
		return ports.ChangeSetApplyResult{}, revisiondomain.ErrInvalidTransition
	}

	pending := pendingOperations(changeSet)
	prepared := make([]preparedOperation, 0, len(pending))
	for _, operation := range pending {
		value, err := prepareOperation(ctx, tx, changeSet, operation, appliedAt)
		if errors.Is(err, projectdomain.ErrRevisionConflict) {
			return markOperationConflict(ctx, tx, changeSet, operation.ID, appliedAt)
		}
		if err != nil {
			return ports.ChangeSetApplyResult{}, err
		}
		prepared = append(prepared, value)
	}
	chapters := make([]projectdomain.Chapter, 0)
	for _, operation := range prepared {
		chapter, err := operation.apply()
		if err != nil {
			return ports.ChangeSetApplyResult{}, err
		}
		if chapter != nil {
			chapters = append(chapters, *chapter)
		}
	}
	changeSet, err = changeSet.Apply(appliedAt)
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.ChangeSetApplyResult{}, databaseOperationError("commit applied change set", err)
	}
	return ports.ChangeSetApplyResult{ChangeSet: changeSet, Chapters: chapters}, nil
}

func (s *Store) EditChangeOperation(
	ctx context.Context,
	changeSetID string,
	operationID string,
	afterValue string,
	editedAt time.Time,
) (revisiondomain.ChangeSet, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("begin edit change operation transaction", err)
	}
	defer tx.Rollback()
	changeSet, err := loadChangeSet(ctx, tx, changeSetID)
	if err != nil {
		return revisiondomain.ChangeSet{}, err
	}
	changeSet, err = changeSet.EditOperation(operationID, afterValue, editedAt)
	if err != nil {
		return revisiondomain.ChangeSet{}, err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return revisiondomain.ChangeSet{}, err
	}
	if err := tx.Commit(); err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("commit edited change operation", err)
	}
	return changeSet, nil
}

func (s *Store) ApplyChangeOperation(
	ctx context.Context,
	changeSetID string,
	operationID string,
	appliedAt time.Time,
) (ports.ChangeSetApplyResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.ChangeSetApplyResult{}, databaseOperationError("begin apply change operation transaction", err)
	}
	defer tx.Rollback()
	changeSet, err := loadChangeSet(ctx, tx, changeSetID)
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	operation, found := findOperation(changeSet, operationID)
	if !found {
		return ports.ChangeSetApplyResult{}, revisiondomain.ErrOperationMissing
	}
	if operation.Status == revisiondomain.ChangeSetApplied {
		return appliedResult(ctx, tx, changeSet)
	}
	if operation.Status != revisiondomain.ChangeSetPending {
		return ports.ChangeSetApplyResult{}, revisiondomain.ErrInvalidTransition
	}
	prepared, err := prepareOperation(ctx, tx, changeSet, operation, appliedAt)
	if errors.Is(err, projectdomain.ErrRevisionConflict) {
		return markOperationConflict(ctx, tx, changeSet, operation.ID, appliedAt)
	}
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	chapter, err := prepared.apply()
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	changeSet, err = changeSet.ApplyOperation(operationID, appliedAt)
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.ChangeSetApplyResult{}, databaseOperationError("commit applied change operation", err)
	}
	chapters := make([]projectdomain.Chapter, 0, 1)
	if chapter != nil {
		chapters = append(chapters, *chapter)
	}
	return ports.ChangeSetApplyResult{ChangeSet: changeSet, Chapters: chapters}, nil
}

func (s *Store) RejectChangeOperation(
	ctx context.Context,
	changeSetID string,
	operationID string,
	rejectedAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin reject change operation transaction", err)
	}
	defer tx.Rollback()
	changeSet, err := loadChangeSet(ctx, tx, changeSetID)
	if err != nil {
		return err
	}
	changeSet, err = changeSet.RejectOperation(operationID, rejectedAt)
	if err != nil {
		return err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit rejected change operation", err)
	}
	return nil
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
	changeSet, err = changeSet.Reject(rejectedAt)
	if err != nil {
		return err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit rejected change set", err)
	}
	return nil
}

type preparedOperation struct {
	apply func() (*projectdomain.Chapter, error)
}

func prepareOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operation revisiondomain.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	switch operation.Kind {
	case revisiondomain.OperationReplaceChapterBody:
		return prepareChapterOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationUpdateOutlineNode:
		return prepareOutlineOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationUpdateCharacter:
		return prepareCharacterOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationUpdateFact:
		return prepareFactOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateOutlineNode:
		return prepareCreateOutlineOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateCharacter:
		return prepareCreateCharacterOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateRelationship:
		return prepareCreateRelationshipOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateWorldRule:
		return prepareCreateWorldRuleOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateStoryEvent:
		return prepareCreateStoryEventOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateFact:
		return prepareCreateFactOperation(ctx, tx, changeSet, operation, appliedAt)
	case revisiondomain.OperationCreateForeshadowingHook:
		return prepareCreateForeshadowingHookOperation(ctx, tx, changeSet, operation, appliedAt)
	default:
		return preparedOperation{}, revisiondomain.ErrInvalidOperation
	}
}

func prepareChapterOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operation revisiondomain.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	current, err := scanChapter(tx.QueryRowContext(ctx, `
		SELECT id, project_id, title, position, body, revision, created_at, updated_at
		FROM chapters WHERE id = ?
	`, operation.TargetID))
	if err != nil || current.ProjectID != changeSet.ProjectID ||
		current.Revision != operation.BaseRevision || current.Body != operation.BeforeValue {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	updated, err := current.Commit(operation.AfterValue, operation.BaseRevision, appliedAt)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		var source, summary string
		if err := tx.QueryRowContext(ctx, `
			SELECT revision_source, revision_summary FROM chapters WHERE id = ?
		`, current.ID).Scan(&source, &summary); err != nil {
			return nil, databaseOperationError("read change target revision metadata", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chapter_revisions(chapter_id, revision, title, body, source, summary, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, current.ID, current.Revision, current.Title, current.Body, source, summary, formatTime(updated.UpdatedAt)); err != nil {
			return nil, databaseOperationError("archive chapter before change set", err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE chapters SET body = ?, revision = ?, revision_source = ?, revision_summary = ?, updated_at = ?
			WHERE id = ? AND revision = ?
		`, updated.Body, updated.Revision, "agent", changeSet.Title, formatTime(updated.UpdatedAt), updated.ID, current.Revision)
		if err != nil {
			return nil, databaseOperationError("apply chapter change operation", err)
		}
		if err := requireOneRow(result, "chapter", updated.ID); err != nil {
			return nil, projectdomain.ErrRevisionConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM drafts WHERE chapter_id = ?`, updated.ID); err != nil {
			return nil, databaseOperationError("clear draft after change set", err)
		}
		return &updated, nil
	}}, nil
}

func prepareOutlineOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operation revisiondomain.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	current, err := scanOutlineNode(tx.QueryRowContext(ctx, `
		SELECT id, project_id, COALESCE(parent_id, ''), level, title, summary, goal,
			conflict, outcome, position, COALESCE(chapter_id, ''), created_at, updated_at
		FROM outline_nodes WHERE id = ? AND deleted_at IS NULL
	`, operation.TargetID))
	if err != nil || current.ProjectID != changeSet.ProjectID ||
		!matchesBaseline(operation, outlineNodeValue{
			Title: current.Title, Summary: current.Summary, Goal: current.Goal,
			Conflict: current.Conflict, Outcome: current.Outcome,
		}) {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	var proposed outlineNodeValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	updated, err := current.UpdateContent(
		proposed.Title, proposed.Summary, proposed.Goal, proposed.Conflict, proposed.Outcome, appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			UPDATE outline_nodes SET title = ?, summary = ?, goal = ?, conflict = ?, outcome = ?, updated_at = ?
			WHERE id = ? AND project_id = ? AND deleted_at IS NULL
		`, updated.Title, updated.Summary, updated.Goal, updated.Conflict, updated.Outcome,
			formatTime(updated.UpdatedAt), updated.ID, updated.ProjectID)
		if err != nil {
			return nil, databaseOperationError("apply outline change operation", err)
		}
		return nil, requireOneRow(result, "outline node", updated.ID)
	}}, nil
}

func prepareCharacterOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operation revisiondomain.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	current, err := scanCharacter(tx.QueryRowContext(ctx, `
		SELECT id, project_id, name, aliases_json, role, summary, motivation, fear,
			secret, arc, status, created_at, updated_at
		FROM characters WHERE id = ? AND deleted_at IS NULL
	`, operation.TargetID))
	if err != nil || current.ProjectID != changeSet.ProjectID ||
		!matchesBaseline(operation, characterValue{
			Name: current.Name, Aliases: current.Aliases, Role: current.Role,
			Summary: current.Summary, Motivation: current.Motivation, Fear: current.Fear,
			Secret: current.Secret, Arc: current.Arc, Status: current.Status,
		}) {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	var proposed characterValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	updated, err := current.UpdateProfile(
		proposed.Name, proposed.Aliases, proposed.Role, proposed.Summary,
		proposed.Motivation, proposed.Fear, proposed.Secret, proposed.Arc,
		proposed.Status, appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	aliases, err := json.Marshal(updated.Aliases)
	if err != nil {
		return preparedOperation{}, fmt.Errorf("encode character aliases: %w", err)
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			UPDATE characters SET name = ?, aliases_json = ?, role = ?, summary = ?,
				motivation = ?, fear = ?, secret = ?, arc = ?, status = ?, updated_at = ?
			WHERE id = ? AND project_id = ? AND deleted_at IS NULL
		`, updated.Name, string(aliases), updated.Role, updated.Summary, updated.Motivation,
			updated.Fear, updated.Secret, updated.Arc, updated.Status, formatTime(updated.UpdatedAt),
			updated.ID, updated.ProjectID)
		if err != nil {
			return nil, databaseOperationError("apply character change operation", err)
		}
		return nil, requireOneRow(result, "character", updated.ID)
	}}, nil
}

func prepareFactOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operation revisiondomain.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	current, err := scanFact(tx.QueryRowContext(ctx, `
		SELECT id, project_id, fact_text, category, origin, status, confidence,
			valid_from_event_id, valid_until_event_id, created_by, confirmed_by, created_at, updated_at
		FROM canon_facts WHERE id = ? AND deleted_at IS NULL
	`, operation.TargetID))
	if err != nil || current.ProjectID != changeSet.ProjectID ||
		!matchesBaseline(operation, factValue{
			Text: current.Text, Category: current.Category, Status: current.Status,
			Confidence: current.Confidence, ValidFromEventID: current.ValidFromEventID,
			ValidUntilEventID: current.ValidUntilEventID,
		}) {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	var proposed factValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	updated, err := current.Update(
		proposed.Text, proposed.Category, proposed.Status, proposed.Confidence,
		proposed.ValidFromEventID, proposed.ValidUntilEventID, appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			UPDATE canon_facts SET fact_text = ?, category = ?, status = ?, confidence = ?,
				valid_from_event_id = ?, valid_until_event_id = ?, confirmed_by = ?, updated_at = ?
			WHERE id = ? AND project_id = ? AND deleted_at IS NULL
		`, updated.Text, updated.Category, updated.Status, updated.Confidence,
			nullString(updated.ValidFromEventID), nullString(updated.ValidUntilEventID),
			nullString(updated.ConfirmedBy), formatTime(updated.UpdatedAt), updated.ID, updated.ProjectID)
		if err != nil {
			return nil, databaseOperationError("apply fact change operation", err)
		}
		return nil, requireOneRow(result, "canon fact", updated.ID)
	}}, nil
}

func markOperationConflict(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revisiondomain.ChangeSet,
	operationID string,
	at time.Time,
) (ports.ChangeSetApplyResult, error) {
	conflicted, err := changeSet.ConflictOperation(operationID, at)
	if err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := persistChangeSetDecision(ctx, tx, conflicted); err != nil {
		return ports.ChangeSetApplyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.ChangeSetApplyResult{}, databaseOperationError("commit conflicted change operation", err)
	}
	return ports.ChangeSetApplyResult{}, projectdomain.ErrRevisionConflict
}

func appliedResult(
	ctx context.Context,
	queryer changeSetQueryer,
	changeSet revisiondomain.ChangeSet,
) (ports.ChangeSetApplyResult, error) {
	chapters := make([]projectdomain.Chapter, 0)
	for _, operation := range changeSet.Operations {
		if operation.Kind != revisiondomain.OperationReplaceChapterBody ||
			operation.Status != revisiondomain.ChangeSetApplied {
			continue
		}
		chapter, err := scanChapter(queryer.QueryRowContext(ctx, `
			SELECT id, project_id, title, position, body, revision, created_at, updated_at
			FROM chapters WHERE id = ?
		`, operation.TargetID))
		if err != nil {
			return ports.ChangeSetApplyResult{}, err
		}
		chapters = append(chapters, chapter)
	}
	return ports.ChangeSetApplyResult{ChangeSet: changeSet, Chapters: chapters}, nil
}

func persistChangeSetDecision(ctx context.Context, tx *sql.Tx, changeSet revisiondomain.ChangeSet) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE change_sets SET status = ?, updated_at = ? WHERE id = ?
	`, changeSet.Status, formatTime(changeSet.UpdatedAt), changeSet.ID); err != nil {
		return databaseOperationError("update change set status", err)
	}
	for _, operation := range changeSet.Operations {
		if _, err := tx.ExecContext(ctx, `
			UPDATE change_operations
			SET after_value = ?, status = ?, updated_at = ?
			WHERE id = ? AND change_set_id = ?
		`, operation.AfterValue, operation.Status, formatTime(operation.UpdatedAt),
			operation.ID, changeSet.ID); err != nil {
			return databaseOperationError("update change operation", err)
		}
	}
	return nil
}

func pendingOperations(changeSet revisiondomain.ChangeSet) []revisiondomain.PatchOperation {
	result := make([]revisiondomain.PatchOperation, 0, len(changeSet.Operations))
	for _, operation := range changeSet.Operations {
		if operation.Status == revisiondomain.ChangeSetPending {
			result = append(result, operation)
		}
	}
	return result
}

func hasPendingOperations(changeSet revisiondomain.ChangeSet) bool {
	return len(pendingOperations(changeSet)) > 0
}

func findOperation(changeSet revisiondomain.ChangeSet, id string) (revisiondomain.PatchOperation, bool) {
	for _, operation := range changeSet.Operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return revisiondomain.PatchOperation{}, false
}

func matchesBaseline(operation revisiondomain.PatchOperation, current any) bool {
	value, err := canonicalJSON(current)
	if err != nil || value != operation.BeforeValue {
		return false
	}
	sum := sha256.Sum256([]byte(value))
	return operation.BaseFingerprint == hex.EncodeToString(sum[:])
}

func canonicalJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(generic)
	if err != nil {
		return "", err
	}
	return string(canonical), nil
}

func decodeOperationValue(raw string, destination any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: %v", revisiondomain.ErrInvalidOperation, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return revisiondomain.ErrInvalidOperation
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
		&changeSet.ID, &changeSet.ProjectID, &changeSet.Title, &changeSet.Summary,
		&changeSet.CreatedBy, &changeSet.Status, &createdAtValue, &updatedAtValue,
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
		SELECT id, change_set_id, kind, target_id, base_revision, base_fingerprint,
			before_value, after_value, reason, evidence, status, created_at, updated_at
		FROM change_operations WHERE change_set_id = ?
		ORDER BY position, id
	`, id)
	if err != nil {
		return revisiondomain.ChangeSet{}, databaseOperationError("list change operations", err)
	}
	defer rows.Close()
	changeSet.Operations = make([]revisiondomain.PatchOperation, 0)
	for rows.Next() {
		var operation revisiondomain.PatchOperation
		var operationCreatedAt, operationUpdatedAt, evidenceJSON string
		if err := rows.Scan(
			&operation.ID, &operation.ChangeSetID, &operation.Kind, &operation.TargetID,
			&operation.BaseRevision, &operation.BaseFingerprint, &operation.BeforeValue,
			&operation.AfterValue, &operation.Reason, &evidenceJSON, &operation.Status,
			&operationCreatedAt, &operationUpdatedAt,
		); err != nil {
			return revisiondomain.ChangeSet{}, databaseOperationError("scan change operation", err)
		}
		operation.Evidence, err = decodeEvidence(evidenceJSON)
		if err != nil {
			return revisiondomain.ChangeSet{}, databaseOperationError("decode change operation evidence", err)
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
