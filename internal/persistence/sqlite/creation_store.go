package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// CreateCreationSession 原子创建或返回已有会话，重复初始化不替换会话身份及作者内容。
func (s *Store) CreateCreationSession(ctx context.Context, session creationdomain.Session) (creationdomain.Session, error) {
	kept, err := encodeKeptDirections(session.KeptDirectionIDs)
	if err != nil {
		return creationdomain.Session{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return creationdomain.Session{}, databaseOperationError("begin creation session", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO creation_sessions (
			id, project_id, branch_id, stage, advance_mode, intent_text,
			selected_direction_candidate_id, selected_direction_card_id, kept_direction_ids_json,
			blocking_reason, active_operation_id, active_run_id,
			archive_digest, archive_idempotency_key, archive_change_set_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO NOTHING
	`,
		session.ID, session.ProjectID, session.BranchID, string(session.Stage), string(session.AdvanceMode),
		session.IntentText, session.SelectedDirectionCandidateID, session.SelectedDirectionCardID, string(kept),
		string(session.BlockingReason), session.ActiveOperationID, session.ActiveRunID,
		session.ArchiveDigest, session.ArchiveIdempotencyKey, session.ArchiveChangeSetID,
		formatTime(session.CreatedAt), formatTime(session.UpdatedAt),
	)
	if err != nil {
		return creationdomain.Session{}, databaseOperationError("create creation session", err)
	}
	existing, err := readCreationSession(ctx, tx, session.ProjectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if existing.BranchID != session.BranchID {
		return creationdomain.Session{}, creationdomain.ErrSessionVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return creationdomain.Session{}, databaseOperationError("commit creation session", err)
	}
	return existing, nil
}

func (s *Store) CreationSession(ctx context.Context, projectID string) (creationdomain.Session, error) {
	return readCreationSession(ctx, s.db, projectID)
}

type creationSessionReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readCreationSession(ctx context.Context, reader creationSessionReader, projectID string) (creationdomain.Session, error) {
	var session creationdomain.Session
	var selected, selectedCard, activeOperation sql.NullString
	var keptJSON, createdAt, updatedAt string
	err := reader.QueryRowContext(ctx, `
		SELECT id, project_id, branch_id, stage, advance_mode, intent_text,
			selected_direction_candidate_id, selected_direction_card_id, kept_direction_ids_json,
			blocking_reason, active_operation_id, active_run_id,
			archive_digest, archive_idempotency_key, archive_change_set_id, created_at, updated_at,
			content_version, run_slot_version
		FROM creation_sessions
		WHERE project_id = ?
	`, strings.TrimSpace(projectID)).Scan(
		&session.ID, &session.ProjectID, &session.BranchID, &session.Stage, &session.AdvanceMode,
		&session.IntentText, &selected, &selectedCard, &keptJSON,
		&session.BlockingReason, &activeOperation, &session.ActiveRunID,
		&session.ArchiveDigest, &session.ArchiveIdempotencyKey, &session.ArchiveChangeSetID, &createdAt, &updatedAt,
		&session.ContentVersion, &session.RunSlotVersion,
	)
	if err != nil {
		return creationdomain.Session{}, fmt.Errorf("read creation session: %w", err)
	}
	if selected.Valid {
		session.SelectedDirectionCandidateID = selected.String
	}
	if selectedCard.Valid {
		session.SelectedDirectionCardID = selectedCard.String
	}
	if activeOperation.Valid {
		session.ActiveOperationID = activeOperation.String
	}
	if err := decodeKeptDirections(keptJSON, &session.KeptDirectionIDs); err != nil {
		return creationdomain.Session{}, err
	}
	session.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return creationdomain.Session{}, fmt.Errorf("parse creation session created_at: %w", err)
	}
	session.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return creationdomain.Session{}, fmt.Errorf("parse creation session updated_at: %w", err)
	}
	return session, nil
}

type creationSessionWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// SaveCreationSession 仅按内容版本更新作者字段，不带回旧快照里的运行槽和暂停原因。
func (s *Store) SaveCreationSession(ctx context.Context, session creationdomain.Session) error {
	return updateCreationSession(ctx, s.db, session)
}

// updateCreationSession 供普通保存与入档事务共用，避免事务路径绕过内容 CAS。
func updateCreationSession(ctx context.Context, writer creationSessionWriter, session creationdomain.Session) error {
	kept, err := encodeKeptDirections(session.KeptDirectionIDs)
	if err != nil {
		return err
	}
	result, err := writer.ExecContext(ctx, `
		UPDATE creation_sessions SET stage = ?, advance_mode = ?, intent_text = ?,
			selected_direction_candidate_id = ?, selected_direction_card_id = ?, kept_direction_ids_json = ?,
			archive_digest = ?, archive_idempotency_key = ?, archive_change_set_id = ?,
			content_version = content_version + 1,
			updated_at = CASE WHEN RTRIM(updated_at, 'Z') < RTRIM(?, 'Z') THEN ? ELSE updated_at END
		WHERE id = ? AND project_id = ? AND branch_id = ? AND content_version = ?
	`, string(session.Stage), string(session.AdvanceMode), session.IntentText,
		session.SelectedDirectionCandidateID, session.SelectedDirectionCardID, string(kept),
		session.ArchiveDigest, session.ArchiveIdempotencyKey, session.ArchiveChangeSetID,
		formatTime(session.UpdatedAt), formatTime(session.UpdatedAt), session.ID, session.ProjectID, session.BranchID, session.ContentVersion)
	return creationSessionCASResult(result, err)
}

// SaveCreationRunSlot 独立比较运行槽版本；旧完成回调无法恢复已清空的槽或清除较新的运行。
func (s *Store) SaveCreationRunSlot(ctx context.Context, session creationdomain.Session) error {
	return updateCreationRunSlot(ctx, s.db, session)
}

// updateCreationRunSlot 允许入档事务同时校验并清除原暂停原因，不覆盖更新后的运行状态。
func updateCreationRunSlot(ctx context.Context, writer creationSessionWriter, session creationdomain.Session) error {
	result, err := writer.ExecContext(ctx, `
		UPDATE creation_sessions SET active_operation_id = ?, active_run_id = ?, blocking_reason = ?,
			run_slot_version = run_slot_version + 1,
			updated_at = CASE WHEN RTRIM(updated_at, 'Z') < RTRIM(?, 'Z') THEN ? ELSE updated_at END
		WHERE id = ? AND project_id = ? AND branch_id = ? AND run_slot_version = ?
	`, session.ActiveOperationID, session.ActiveRunID, string(session.BlockingReason),
		formatTime(session.UpdatedAt), formatTime(session.UpdatedAt), session.ID, session.ProjectID, session.BranchID, session.RunSlotVersion)
	return creationSessionCASResult(result, err)
}

func creationSessionCASResult(result sql.Result, err error) error {
	if err != nil {
		return databaseOperationError("update creation session", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read creation session update result", err)
	}
	if count != 1 {
		return creationdomain.ErrSessionVersionConflict
	}
	return nil
}

// SaveCreationCandidate 写入新的候选版本，不覆盖已有行。
func (s *Store) SaveCreationCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return insertCreationCandidate(ctx, s.db, candidate)
}

func insertCreationCandidate(ctx context.Context, exec txExecutor, candidate creationdomain.Candidate) error {
	locks, err := json.Marshal(candidate.LockMap)
	if err != nil {
		return fmt.Errorf("encode creation candidate locks: %w", err)
	}
	if string(locks) == "null" {
		locks = []byte("{}")
	}
	refs, err := encodeImpactRefs(candidate.ImpactRefs)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `
		INSERT INTO creation_candidates (
			id, session_id, stage, kind, version, parent_candidate_id, supersedes_candidate_id,
			run_id, artifact_id, payload_json, payload_schema_id, payload_schema_version,
			content_sha256, lock_map_json, status, quality_status, origin, stale_reason, impact_refs_json,
			upstream_digest, excepted_upstream_digest, impact_severity, result_key, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(result_key) WHERE result_key <> '' DO NOTHING
	`,
		candidate.ID, candidate.SessionID, string(candidate.Stage), string(candidate.Kind), candidate.Version,
		candidate.ParentCandidateID, candidate.SupersedesCandidateID, candidate.RunID, candidate.ArtifactID,
		candidate.PayloadJSON, candidate.PayloadSchemaID, candidate.PayloadSchemaVersion, candidate.ContentSHA256,
		string(locks), string(candidate.Status), string(candidate.QualityStatus), string(candidate.Origin),
		candidate.StaleReason, string(refs),
		candidate.UpstreamDigest, candidate.ExceptedUpstreamDigest, string(candidate.ImpactSeverity),
		candidateResultKey(candidate),
		formatTime(candidate.CreatedAt), formatTime(candidate.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save creation candidate", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read creation candidate insert result", err)
	}
	if affected == 0 {
		return creationdomain.ErrCandidateResultExists
	}
	return nil
}

func candidateResultKey(candidate creationdomain.Candidate) string {
	if strings.TrimSpace(candidate.RunID) == "" || strings.TrimSpace(candidate.ArtifactID) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(candidate.RunID + "\x00" + candidate.ArtifactID + "\x00" + string(candidate.Kind)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) CreationCandidate(ctx context.Context, id string) (creationdomain.Candidate, error) {
	var candidate creationdomain.Candidate
	var parent, supersedes, runID, artifactID sql.NullString
	var lockJSON, refsJSON, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, stage, kind, version, parent_candidate_id, supersedes_candidate_id,
			run_id, artifact_id, payload_json, payload_schema_id, payload_schema_version,
			content_sha256, lock_map_json, status, quality_status, origin, stale_reason, impact_refs_json,
			upstream_digest, excepted_upstream_digest, impact_severity, created_at, updated_at
		FROM creation_candidates
		WHERE id = ?
	`, strings.TrimSpace(id)).Scan(
		&candidate.ID, &candidate.SessionID, &candidate.Stage, &candidate.Kind, &candidate.Version,
		&parent, &supersedes, &runID, &artifactID, &candidate.PayloadJSON, &candidate.PayloadSchemaID,
		&candidate.PayloadSchemaVersion, &candidate.ContentSHA256, &lockJSON, &candidate.Status,
		&candidate.QualityStatus, &candidate.Origin, &candidate.StaleReason, &refsJSON, &candidate.UpstreamDigest,
		&candidate.ExceptedUpstreamDigest, &candidate.ImpactSeverity, &createdAt, &updatedAt,
	)
	if err != nil {
		return creationdomain.Candidate{}, fmt.Errorf("read creation candidate: %w", err)
	}
	candidate.ParentCandidateID = parent.String
	candidate.SupersedesCandidateID = supersedes.String
	candidate.RunID = runID.String
	candidate.ArtifactID = artifactID.String
	if err := json.Unmarshal([]byte(lockJSON), &candidate.LockMap); err != nil {
		return creationdomain.Candidate{}, fmt.Errorf("decode creation candidate locks: %w", err)
	}
	if candidate.LockMap == nil {
		candidate.LockMap = map[string]bool{}
	}
	if strings.TrimSpace(refsJSON) == "" {
		refsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(refsJSON), &candidate.ImpactRefs); err != nil {
		return creationdomain.Candidate{}, fmt.Errorf("decode creation candidate impact refs: %w", err)
	}
	if candidate.ImpactRefs == nil {
		candidate.ImpactRefs = []string{}
	}
	candidate.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return creationdomain.Candidate{}, fmt.Errorf("parse creation candidate created_at: %w", err)
	}
	candidate.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return creationdomain.Candidate{}, fmt.Errorf("parse creation candidate updated_at: %w", err)
	}
	return candidate, nil
}

// ListCreationCandidates 按版本倒序列出会话内全部候选。
func (s *Store) ListCreationCandidates(ctx context.Context, sessionID string) ([]creationdomain.Candidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM creation_candidates
		WHERE session_id = ?
		ORDER BY version DESC, id
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, databaseOperationError("list creation candidates", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]creationdomain.Candidate, 0, len(ids))
	for _, id := range ids {
		candidate, err := s.CreationCandidate(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, nil
}

// UpdateCreationCandidate 保存状态、锁和谱系，不改 payload。
func (s *Store) UpdateCreationCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return updateCreationCandidate(ctx, s.db, candidate)
}

func updateCreationCandidate(ctx context.Context, exec txExecutor, candidate creationdomain.Candidate) error {
	locks, err := json.Marshal(candidate.LockMap)
	if err != nil {
		return fmt.Errorf("encode creation candidate locks: %w", err)
	}
	if string(locks) == "null" {
		locks = []byte("{}")
	}
	refs, err := encodeImpactRefs(candidate.ImpactRefs)
	if err != nil {
		return err
	}
	result, err := exec.ExecContext(ctx, `
		UPDATE creation_candidates SET
			parent_candidate_id = NULLIF(?, ''),
			supersedes_candidate_id = NULLIF(?, ''),
			lock_map_json = ?,
			status = ?,
			quality_status = ?,
			stale_reason = ?,
			impact_refs_json = ?,
			upstream_digest = ?,
			excepted_upstream_digest = ?,
			impact_severity = ?,
			updated_at = ?
		WHERE id = ?
	`, candidate.ParentCandidateID, candidate.SupersedesCandidateID, string(locks),
		string(candidate.Status), string(candidate.QualityStatus), candidate.StaleReason, string(refs), candidate.UpstreamDigest,
		candidate.ExceptedUpstreamDigest, string(candidate.ImpactSeverity),
		formatTime(candidate.UpdatedAt), candidate.ID)
	if err != nil {
		return databaseOperationError("update creation candidate", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read creation candidate update result", err)
	}
	if count != 1 {
		return creationdomain.ErrCandidateVersionConflict
	}
	return nil
}

// ApplyCreationCandidateChanges 在写锁事务中先校验全部旧快照，再原子提交候选状态与新版本。
func (s *Store) ApplyCreationCandidateChanges(
	ctx context.Context,
	expected, updates, inserts []creationdomain.Candidate,
	guards ...creationdomain.CandidateCommitGuard,
) error {
	tx, err := beginImmediate(ctx, s.db)
	if err != nil {
		return databaseOperationError("begin creation candidate transaction", err)
	}
	defer tx.Rollback()
	for _, guard := range guards {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation_sessions s JOIN projects p ON p.id=s.project_id WHERE s.id=? AND s.content_version=? AND p.id=? AND p.updated_at=?`, guard.SessionID, guard.ContentVersion, guard.ProjectID, formatTime(guard.ProjectUpdatedAt)).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return creationdomain.ErrSessionVersionConflict
		}
	}
	for _, candidate := range expected {
		if err := verifyCreationCandidateSnapshot(ctx, tx, candidate); err != nil {
			return err
		}
	}
	for _, candidate := range updates {
		if err := updateCreationCandidate(ctx, tx, candidate); err != nil {
			return err
		}
	}
	for _, candidate := range inserts {
		if err := insertCreationCandidate(ctx, tx, candidate); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return databaseOperationError("commit creation candidate transaction", err)
	}
	return nil
}

func verifyCreationCandidateSnapshot(ctx context.Context, reader creationSessionReader, candidate creationdomain.Candidate) error {
	locks, err := json.Marshal(candidate.LockMap)
	if err != nil {
		return err
	}
	if string(locks) == "null" {
		locks = []byte("{}")
	}
	refs, err := encodeImpactRefs(candidate.ImpactRefs)
	if err != nil {
		return err
	}
	var count int
	err = reader.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM creation_candidates
		WHERE id = ? AND session_id = ? AND stage = ? AND kind = ? AND version = ?
		AND COALESCE(parent_candidate_id, '') = ? AND COALESCE(supersedes_candidate_id, '') = ?
		AND COALESCE(run_id, '') = ? AND COALESCE(artifact_id, '') = ?
		AND payload_json = ? AND payload_schema_id = ? AND payload_schema_version = ? AND content_sha256 = ?
		AND lock_map_json = ? AND status = ? AND quality_status = ? AND origin = ?
		AND stale_reason = ? AND impact_refs_json = ? AND upstream_digest = ?
		AND excepted_upstream_digest = ? AND impact_severity = ? AND created_at = ? AND updated_at = ?
	`, candidate.ID, candidate.SessionID, string(candidate.Stage), string(candidate.Kind), candidate.Version,
		candidate.ParentCandidateID, candidate.SupersedesCandidateID, candidate.RunID, candidate.ArtifactID,
		candidate.PayloadJSON, candidate.PayloadSchemaID, candidate.PayloadSchemaVersion, candidate.ContentSHA256,
		string(locks), string(candidate.Status), string(candidate.QualityStatus), string(candidate.Origin),
		candidate.StaleReason, string(refs), candidate.UpstreamDigest, candidate.ExceptedUpstreamDigest,
		string(candidate.ImpactSeverity), formatTime(candidate.CreatedAt), formatTime(candidate.UpdatedAt),
	).Scan(&count)
	if err != nil {
		return databaseOperationError("verify creation candidate snapshot", err)
	}
	if count != 1 {
		return creationdomain.ErrCandidateVersionConflict
	}
	return nil
}

// ReserveCreationOperation 以唯一键原子预留创作操作回执；键已存在时返回既有回执与 false。
func (s *Store) ReserveCreationOperation(ctx context.Context, operation creationdomain.Operation) (creationdomain.Operation, bool, error) {
	trimmed := normalizeOperation(operation)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO creation_operations (
			id, project_id, branch_id, command, idempotency_key, arguments_hash, frozen_input_digest,
			status, run_id, candidate_id, error_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, branch_id, command, idempotency_key) DO NOTHING
	`,
		trimmed.ID, trimmed.ProjectID, trimmed.BranchID, trimmed.Command, trimmed.IdempotencyKey,
		trimmed.ArgumentsHash, trimmed.FrozenInputDigest, string(trimmed.Status), trimmed.RunID, trimmed.CandidateID,
		trimmed.ErrorCode, formatTime(trimmed.CreatedAt), formatTime(trimmed.UpdatedAt),
	)
	if err != nil {
		return creationdomain.Operation{}, false, databaseOperationError("reserve creation operation", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return creationdomain.Operation{}, false, databaseOperationError("reserve creation operation rows", err)
	}
	if affected == 0 {
		existing, loadErr := s.CreationOperationByIdempotency(ctx, trimmed.ProjectID, trimmed.BranchID, trimmed.Command, trimmed.IdempotencyKey)
		if loadErr != nil {
			return creationdomain.Operation{}, false, loadErr
		}
		return existing, false, nil
	}
	return trimmed, true, nil
}

// CreationOperation 按操作身份读取恢复所需的冻结输入证据。
func (s *Store) CreationOperation(ctx context.Context, id string) (creationdomain.Operation, error) {
	operation, err := scanCreationOperation(s.db.QueryRowContext(ctx, creationOperationSelect+`
		WHERE id = ?
	`, strings.TrimSpace(id)))
	if err != nil {
		return creationdomain.Operation{}, fmt.Errorf("read creation operation: %w", err)
	}
	return operation, nil
}

func (s *Store) CreationOperationByIdempotency(
	ctx context.Context, projectID, branchID, command, idempotencyKey string,
) (creationdomain.Operation, error) {
	operation, err := scanCreationOperation(s.db.QueryRowContext(ctx, creationOperationSelect+`
		WHERE project_id = ? AND branch_id = ? AND command = ? AND idempotency_key = ?
	`, strings.TrimSpace(projectID), strings.TrimSpace(branchID), strings.TrimSpace(command), strings.TrimSpace(idempotencyKey)))
	if err != nil {
		return creationdomain.Operation{}, fmt.Errorf("read creation operation: %w", err)
	}
	return operation, nil
}

// UpdateCreationOperation 只更新执行状态、当前子运行的冻结摘要与结果引用，不改操作身份和参数摘要。
func (s *Store) UpdateCreationOperation(ctx context.Context, operation creationdomain.Operation) error {
	trimmed := normalizeOperation(operation)
	result, err := s.db.ExecContext(ctx, `
		UPDATE creation_operations SET
			status = ?, run_id = ?, frozen_input_digest = ?, candidate_id = ?, error_code = ?, updated_at = ?
		WHERE id = ?
	`, string(trimmed.Status), trimmed.RunID, trimmed.FrozenInputDigest, trimmed.CandidateID, trimmed.ErrorCode,
		formatTime(trimmed.UpdatedAt), trimmed.ID)
	if err != nil {
		return databaseOperationError("update creation operation", err)
	}
	return requireOneRow(result, "creation operation", trimmed.ID)
}

const creationOperationSelect = `
	SELECT id, project_id, branch_id, command, idempotency_key, arguments_hash, frozen_input_digest,
		status, run_id, candidate_id, error_code, created_at, updated_at
	FROM creation_operations
`

func scanCreationOperation(row *sql.Row) (creationdomain.Operation, error) {
	var operation creationdomain.Operation
	var createdAt, updatedAt string
	err := row.Scan(
		&operation.ID, &operation.ProjectID, &operation.BranchID, &operation.Command,
		&operation.IdempotencyKey, &operation.ArgumentsHash, &operation.FrozenInputDigest, &operation.Status,
		&operation.RunID, &operation.CandidateID, &operation.ErrorCode, &createdAt, &updatedAt,
	)
	if err != nil {
		return creationdomain.Operation{}, err
	}
	if operation.CreatedAt, err = parseTime(createdAt); err != nil {
		return creationdomain.Operation{}, fmt.Errorf("parse creation operation created_at: %w", err)
	}
	if operation.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return creationdomain.Operation{}, fmt.Errorf("parse creation operation updated_at: %w", err)
	}
	return operation, nil
}

func normalizeOperation(operation creationdomain.Operation) creationdomain.Operation {
	operation.ID = strings.TrimSpace(operation.ID)
	operation.ProjectID = strings.TrimSpace(operation.ProjectID)
	operation.BranchID = strings.TrimSpace(operation.BranchID)
	operation.Command = strings.TrimSpace(operation.Command)
	operation.IdempotencyKey = strings.TrimSpace(operation.IdempotencyKey)
	operation.FrozenInputDigest = strings.TrimSpace(operation.FrozenInputDigest)
	operation.RunID = strings.TrimSpace(operation.RunID)
	operation.CandidateID = strings.TrimSpace(operation.CandidateID)
	return operation
}

func encodeKeptDirections(values []string) ([]byte, error) {
	if values == nil {
		return []byte("[]"), nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode kept direction ids: %w", err)
	}
	if string(encoded) == "null" {
		return []byte("[]"), nil
	}
	return encoded, nil
}

func decodeKeptDirections(encoded string, target *[]string) error {
	if strings.TrimSpace(encoded) == "" {
		*target = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(encoded), target); err != nil {
		return fmt.Errorf("decode kept direction ids: %w", err)
	}
	if *target == nil {
		*target = []string{}
	}
	return nil
}

func (s *Store) PendingAssistantActionCount(ctx context.Context, projectID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM assistant_actions
		WHERE project_id = ? AND state = 'pending_confirmation'
	`, strings.TrimSpace(projectID)).Scan(&count)
	if err != nil {
		return 0, databaseOperationError("count pending assistant actions", err)
	}
	return count, nil
}

// SaveStoryAsset 写入确认后的资产信封。
func (s *Store) SaveStoryAsset(ctx context.Context, asset storyassetdomain.Asset) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO story_assets (
			id, project_id, kind, status, current_revision, origin, source_candidate_id,
			source_run_id, source_artifact_id, typed_table, typed_id, locked, stale,
			stale_reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			current_revision = excluded.current_revision,
			locked = excluded.locked,
			stale = excluded.stale,
			stale_reason = excluded.stale_reason,
			updated_at = excluded.updated_at
	`,
		asset.ID, asset.ProjectID, string(asset.Kind), string(asset.Status), asset.CurrentRevision,
		string(asset.Origin), asset.SourceCandidateID, asset.SourceRunID, asset.SourceArtifactID,
		asset.TypedTable, asset.TypedID, boolToInt(asset.Locked), boolToInt(asset.Stale),
		asset.StaleReason, formatTime(asset.CreatedAt), formatTime(asset.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save story asset", err)
	}
	return nil
}

func (s *Store) StoryAsset(ctx context.Context, id string) (storyassetdomain.Asset, error) {
	var asset storyassetdomain.Asset
	var sourceCandidate, sourceRun, sourceArtifact sql.NullString
	var locked, stale int
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, kind, status, current_revision, origin, source_candidate_id,
			source_run_id, source_artifact_id, typed_table, typed_id, locked, stale,
			stale_reason, created_at, updated_at
		FROM story_assets
		WHERE id = ?
	`, strings.TrimSpace(id)).Scan(
		&asset.ID, &asset.ProjectID, &asset.Kind, &asset.Status, &asset.CurrentRevision, &asset.Origin,
		&sourceCandidate, &sourceRun, &sourceArtifact, &asset.TypedTable, &asset.TypedID,
		&locked, &stale, &asset.StaleReason, &createdAt, &updatedAt,
	)
	if err != nil {
		return storyassetdomain.Asset{}, fmt.Errorf("read story asset: %w", err)
	}
	asset.Locked = locked == 1
	asset.Stale = stale == 1
	asset.SourceCandidateID = sourceCandidate.String
	asset.SourceRunID = sourceRun.String
	asset.SourceArtifactID = sourceArtifact.String
	asset.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return storyassetdomain.Asset{}, fmt.Errorf("parse story asset created_at: %w", err)
	}
	asset.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return storyassetdomain.Asset{}, fmt.Errorf("parse story asset updated_at: %w", err)
	}
	return asset, nil
}

// StoryAssetRevision 读取正式资产指定版本的完整载荷与审计信息。
func (s *Store) StoryAssetRevision(ctx context.Context, assetID string, revision int64) (storyassetdomain.Revision, error) {
	var result storyassetdomain.Revision
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT asset_id, revision, payload_json, content_sha256, created_by, created_at
		FROM story_asset_revisions
		WHERE asset_id = ? AND revision = ?
	`, strings.TrimSpace(assetID), revision).Scan(
		&result.AssetID, &result.Revision, &result.PayloadJSON, &result.ContentSHA256,
		&result.CreatedBy, &createdAt,
	)
	if err != nil {
		return storyassetdomain.Revision{}, fmt.Errorf("read story asset revision: %w", err)
	}
	result.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return storyassetdomain.Revision{}, fmt.Errorf("parse story asset revision created_at: %w", err)
	}
	return result, nil
}

// ListStoryAnchors 返回正式锚点及其已解析的稳定作用域。
func (s *Store) ListStoryAnchors(ctx context.Context) ([]storyassetdomain.Anchor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, asset_id, project_id, strength, scope_kind, scope_id, statement, created_at
		FROM story_anchors
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, databaseOperationError("list story anchors", err)
	}
	defer rows.Close()
	anchors := make([]storyassetdomain.Anchor, 0)
	for rows.Next() {
		var anchor storyassetdomain.Anchor
		var createdAt string
		if err := rows.Scan(
			&anchor.ID, &anchor.AssetID, &anchor.ProjectID, &anchor.Strength,
			&anchor.ScopeKind, &anchor.ScopeID, &anchor.Statement, &createdAt,
		); err != nil {
			return nil, databaseOperationError("scan story anchor", err)
		}
		anchor.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse story anchor created_at: %w", err)
		}
		anchors = append(anchors, anchor)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate story anchors", err)
	}
	return anchors, nil
}

// SaveAssistantAction 要求会话已存在，用于保存待确认或状态变化后的 Action。
func (s *Store) SaveAssistantSession(ctx context.Context, id, projectID, branchID string, now string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO assistant_sessions (id, project_id, branch_id, kind, created_at, updated_at)
		VALUES (?, ?, ?, 'creation', ?, ?)
		ON CONFLICT(project_id, branch_id, kind) DO UPDATE SET updated_at = excluded.updated_at
	`, id, projectID, branchID, now, now)
	if err != nil {
		return databaseOperationError("save assistant session", err)
	}
	return nil
}

func (s *Store) SaveAssistantAction(ctx context.Context, action assistantdomain.Action) error {
	targets, err := json.Marshal(action.TargetRefs)
	if err != nil {
		return fmt.Errorf("encode assistant action targets: %w", err)
	}
	versions, err := json.Marshal(action.BaseVersions)
	if err != nil {
		return fmt.Errorf("encode assistant action base versions: %w", err)
	}
	results, err := json.Marshal(action.ResultRefs)
	if err != nil {
		return fmt.Errorf("encode assistant action result refs: %w", err)
	}
	if string(results) == "null" {
		results = []byte("[]")
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO assistant_actions (
			id, session_id, project_id, branch_id, operation, risk, target_refs_json,
			base_versions_json, arguments, arguments_hash, idempotency_key, estimated_cost,
			state, error_code, result_refs_json, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			error_code = excluded.error_code,
			result_refs_json = excluded.result_refs_json,
			updated_at = excluded.updated_at
	`,
		action.ID, action.SessionID, action.ProjectID, action.BranchID, string(action.Operation),
		string(action.Risk), string(targets), string(versions), action.Arguments, action.ArgumentsHash,
		action.IdempotencyKey, action.EstimatedCost, string(action.State), action.ErrorCode, string(results),
		formatTime(action.ExpiresAt), formatTime(action.CreatedAt), formatTime(action.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save assistant action", err)
	}
	return nil
}

func (s *Store) AssistantAction(ctx context.Context, id string) (assistantdomain.Action, error) {
	return s.scanAssistantAction(s.db.QueryRowContext(ctx, assistantActionSelect+` WHERE id = ?`, strings.TrimSpace(id)))
}

func (s *Store) AssistantActionByIdempotency(ctx context.Context, sessionID, key string) (assistantdomain.Action, error) {
	return s.scanAssistantAction(s.db.QueryRowContext(ctx, assistantActionSelect+`
		WHERE session_id = ? AND idempotency_key = ?
	`, strings.TrimSpace(sessionID), strings.TrimSpace(key)))
}

func (s *Store) ListAssistantActions(ctx context.Context, sessionID string) ([]assistantdomain.Action, error) {
	rows, err := s.db.QueryContext(ctx, assistantActionSelect+`
		WHERE session_id = ?
		ORDER BY created_at, id
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, databaseOperationError("list assistant actions", err)
	}
	defer rows.Close()
	actions := make([]assistantdomain.Action, 0)
	for rows.Next() {
		action, err := scanAssistantActionRow(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate assistant actions", err)
	}
	return actions, nil
}

func (s *Store) AssistantSession(ctx context.Context, projectID, branchID string) (assistantdomain.Session, error) {
	var session assistantdomain.Session
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, branch_id, kind, created_at, updated_at
		FROM assistant_sessions
		WHERE project_id = ? AND branch_id = ? AND kind = 'creation'
	`, strings.TrimSpace(projectID), strings.TrimSpace(branchID)).Scan(
		&session.ID, &session.ProjectID, &session.BranchID, &session.Kind, &createdAt, &updatedAt,
	)
	if err != nil {
		return assistantdomain.Session{}, fmt.Errorf("read assistant session: %w", err)
	}
	session.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return assistantdomain.Session{}, fmt.Errorf("parse assistant session created_at: %w", err)
	}
	session.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return assistantdomain.Session{}, fmt.Errorf("parse assistant session updated_at: %w", err)
	}
	return session, nil
}

func (s *Store) SaveAssistantSessionRecord(ctx context.Context, session assistantdomain.Session) error {
	return s.SaveAssistantSession(ctx, session.ID, session.ProjectID, session.BranchID, formatTime(session.UpdatedAt))
}

const assistantActionSelect = `
		SELECT id, session_id, project_id, branch_id, operation, risk, target_refs_json,
			base_versions_json, arguments, arguments_hash, idempotency_key, estimated_cost,
			state, error_code, result_refs_json, expires_at, created_at, updated_at
		FROM assistant_actions
`

type assistantActionRow interface {
	Scan(dest ...any) error
}

func (s *Store) scanAssistantAction(row assistantActionRow) (assistantdomain.Action, error) {
	action, err := scanAssistantActionRow(row)
	if err != nil {
		return assistantdomain.Action{}, fmt.Errorf("read assistant action: %w", err)
	}
	return action, nil
}

func scanAssistantActionRow(row assistantActionRow) (assistantdomain.Action, error) {
	var action assistantdomain.Action
	var targets, versions, results, expiresAt, createdAt, updatedAt string
	err := row.Scan(
		&action.ID, &action.SessionID, &action.ProjectID, &action.BranchID, &action.Operation,
		&action.Risk, &targets, &versions, &action.Arguments, &action.ArgumentsHash,
		&action.IdempotencyKey, &action.EstimatedCost, &action.State, &action.ErrorCode, &results,
		&expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if err := json.Unmarshal([]byte(targets), &action.TargetRefs); err != nil {
		return assistantdomain.Action{}, fmt.Errorf("decode assistant action targets: %w", err)
	}
	if err := json.Unmarshal([]byte(versions), &action.BaseVersions); err != nil {
		return assistantdomain.Action{}, fmt.Errorf("decode assistant action base versions: %w", err)
	}
	if strings.TrimSpace(results) == "" {
		results = "[]"
	}
	if err := json.Unmarshal([]byte(results), &action.ResultRefs); err != nil {
		return assistantdomain.Action{}, fmt.Errorf("decode assistant action result refs: %w", err)
	}
	action.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return assistantdomain.Action{}, fmt.Errorf("parse assistant action expires_at: %w", err)
	}
	action.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return assistantdomain.Action{}, fmt.Errorf("parse assistant action created_at: %w", err)
	}
	action.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return assistantdomain.Action{}, fmt.Errorf("parse assistant action updated_at: %w", err)
	}
	return action, nil
}

func encodeImpactRefs(values []string) ([]byte, error) {
	if values == nil {
		return []byte("[]"), nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode creation candidate impact refs: %w", err)
	}
	if string(encoded) == "null" {
		return []byte("[]"), nil
	}
	return encoded, nil
}
