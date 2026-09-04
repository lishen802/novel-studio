package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// SaveCreationSession 以项目为唯一键保存或更新原创构建会话。
func (s *Store) SaveCreationSession(ctx context.Context, session creationdomain.Session) error {
	kept, err := encodeKeptDirections(session.KeptDirectionIDs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO creation_sessions (
			id, project_id, branch_id, stage, advance_mode, intent_text,
			selected_direction_candidate_id, selected_direction_card_id, kept_direction_ids_json,
			blocking_reason, active_operation_id, active_run_id,
			archive_digest, archive_idempotency_key, archive_change_set_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			id = excluded.id,
			branch_id = excluded.branch_id,
			stage = excluded.stage,
			advance_mode = excluded.advance_mode,
			intent_text = excluded.intent_text,
			selected_direction_candidate_id = excluded.selected_direction_candidate_id,
			selected_direction_card_id = excluded.selected_direction_card_id,
			kept_direction_ids_json = excluded.kept_direction_ids_json,
			blocking_reason = excluded.blocking_reason,
			active_operation_id = excluded.active_operation_id,
			active_run_id = excluded.active_run_id,
			archive_digest = excluded.archive_digest,
			archive_idempotency_key = excluded.archive_idempotency_key,
			archive_change_set_id = excluded.archive_change_set_id,
			updated_at = excluded.updated_at
	`,
		session.ID, session.ProjectID, session.BranchID, string(session.Stage), string(session.AdvanceMode),
		session.IntentText, session.SelectedDirectionCandidateID, session.SelectedDirectionCardID, string(kept),
		string(session.BlockingReason), session.ActiveOperationID, session.ActiveRunID,
		session.ArchiveDigest, session.ArchiveIdempotencyKey, session.ArchiveChangeSetID,
		formatTime(session.CreatedAt), formatTime(session.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save creation session", err)
	}
	return nil
}

func (s *Store) CreationSession(ctx context.Context, projectID string) (creationdomain.Session, error) {
	var session creationdomain.Session
	var selected, selectedCard, activeOperation sql.NullString
	var keptJSON, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, branch_id, stage, advance_mode, intent_text,
			selected_direction_candidate_id, selected_direction_card_id, kept_direction_ids_json,
			blocking_reason, active_operation_id, active_run_id,
			archive_digest, archive_idempotency_key, archive_change_set_id, created_at, updated_at
		FROM creation_sessions
		WHERE project_id = ?
	`, strings.TrimSpace(projectID)).Scan(
		&session.ID, &session.ProjectID, &session.BranchID, &session.Stage, &session.AdvanceMode,
		&session.IntentText, &selected, &selectedCard, &keptJSON,
		&session.BlockingReason, &activeOperation, &session.ActiveRunID,
		&session.ArchiveDigest, &session.ArchiveIdempotencyKey, &session.ArchiveChangeSetID, &createdAt, &updatedAt,
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

// SaveCreationCandidate 写入新的候选版本，不覆盖已有行。
func (s *Store) SaveCreationCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
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
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO creation_candidates (
			id, session_id, stage, kind, version, parent_candidate_id, supersedes_candidate_id,
			run_id, artifact_id, payload_json, payload_schema_id, payload_schema_version,
			content_sha256, lock_map_json, status, quality_status, origin, stale_reason, impact_refs_json,
			upstream_digest, excepted_upstream_digest, impact_severity, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		candidate.ID, candidate.SessionID, string(candidate.Stage), string(candidate.Kind), candidate.Version,
		candidate.ParentCandidateID, candidate.SupersedesCandidateID, candidate.RunID, candidate.ArtifactID,
		candidate.PayloadJSON, candidate.PayloadSchemaID, candidate.PayloadSchemaVersion, candidate.ContentSHA256,
		string(locks), string(candidate.Status), string(candidate.QualityStatus), string(candidate.Origin),
		candidate.StaleReason, string(refs),
		candidate.UpstreamDigest, candidate.ExceptedUpstreamDigest, string(candidate.ImpactSeverity),
		formatTime(candidate.CreatedAt), formatTime(candidate.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save creation candidate", err)
	}
	return nil
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
	_, err = s.db.ExecContext(ctx, `
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
	return nil
}

// ReserveCreationOperation 以唯一键原子预留创作操作回执；键已存在时返回既有回执与 false。
func (s *Store) ReserveCreationOperation(ctx context.Context, operation creationdomain.Operation) (creationdomain.Operation, bool, error) {
	trimmed := normalizeOperation(operation)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO creation_operations (
			id, project_id, branch_id, command, idempotency_key, arguments_hash,
			status, run_id, candidate_id, error_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, branch_id, command, idempotency_key) DO NOTHING
	`,
		trimmed.ID, trimmed.ProjectID, trimmed.BranchID, trimmed.Command, trimmed.IdempotencyKey,
		trimmed.ArgumentsHash, string(trimmed.Status), trimmed.RunID, trimmed.CandidateID,
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

// UpdateCreationOperation 只更新状态与结果引用，不改身份与参数摘要。
func (s *Store) UpdateCreationOperation(ctx context.Context, operation creationdomain.Operation) error {
	trimmed := normalizeOperation(operation)
	result, err := s.db.ExecContext(ctx, `
		UPDATE creation_operations SET
			status = ?, run_id = ?, candidate_id = ?, error_code = ?, updated_at = ?
		WHERE id = ?
	`, string(trimmed.Status), trimmed.RunID, trimmed.CandidateID, trimmed.ErrorCode,
		formatTime(trimmed.UpdatedAt), trimmed.ID)
	if err != nil {
		return databaseOperationError("update creation operation", err)
	}
	return requireOneRow(result, "creation operation", trimmed.ID)
}

const creationOperationSelect = `
	SELECT id, project_id, branch_id, command, idempotency_key, arguments_hash,
		status, run_id, candidate_id, error_code, created_at, updated_at
	FROM creation_operations
`

func scanCreationOperation(row *sql.Row) (creationdomain.Operation, error) {
	var operation creationdomain.Operation
	var createdAt, updatedAt string
	err := row.Scan(
		&operation.ID, &operation.ProjectID, &operation.BranchID, &operation.Command,
		&operation.IdempotencyKey, &operation.ArgumentsHash, &operation.Status,
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
