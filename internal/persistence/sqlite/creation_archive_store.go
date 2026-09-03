package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// ApplyCreationArchive 在同一事务中写入 ChangeSet、正式资产、会话确认和工作台写作阶段。
func (s *Store) ApplyCreationArchive(
	ctx context.Context,
	changeSet revisiondomain.ChangeSet,
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	workbench projectdomain.WorkbenchState,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin creation archive transaction", err)
	}
	defer tx.Rollback()
	if err := insertChangeSet(ctx, tx, changeSet); err != nil {
		return err
	}
	appliedAt := workbench.UpdatedAt
	if appliedAt.IsZero() {
		appliedAt = time.Now().UTC()
	}
	prepared := make([]preparedOperation, 0, len(changeSet.Operations))
	for _, operation := range pendingOperations(changeSet) {
		value, err := prepareOperation(ctx, tx, changeSet, operation, appliedAt)
		if err != nil {
			return err
		}
		prepared = append(prepared, value)
	}
	for _, operation := range prepared {
		if _, err := operation.apply(); err != nil {
			return err
		}
	}
	changeSet, err = changeSet.Apply(appliedAt)
	if err != nil {
		return err
	}
	if err := persistChangeSetDecision(ctx, tx, changeSet); err != nil {
		return err
	}
	if err := persistCreationSessionTx(ctx, tx, session); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if err := persistCreationCandidateTx(ctx, tx, candidate); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_workbench_state (
			id, stage, canvas, ai_panel, pending_review_count,
			recoverable_job_count, archive_status, updated_at
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			stage = excluded.stage,
			canvas = excluded.canvas,
			ai_panel = excluded.ai_panel,
			pending_review_count = excluded.pending_review_count,
			recoverable_job_count = excluded.recoverable_job_count,
			archive_status = excluded.archive_status,
			updated_at = excluded.updated_at
	`,
		string(workbench.Stage), workbench.Canvas, workbench.AIPanel,
		workbench.PendingReviewCount, workbench.RecoverableJobCount,
		workbench.ArchiveStatus, formatTime(workbench.UpdatedAt),
	); err != nil {
		return databaseOperationError("save workbench after archive", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit creation archive", err)
	}
	return nil
}

func persistCreationSessionTx(ctx context.Context, tx *sql.Tx, session creationdomain.Session) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO creation_sessions (
			id, project_id, branch_id, stage, advance_mode, intent_text,
			selected_direction_candidate_id, blocking_reason, active_run_id,
			archive_digest, archive_idempotency_key, archive_change_set_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			id = excluded.id,
			branch_id = excluded.branch_id,
			stage = excluded.stage,
			advance_mode = excluded.advance_mode,
			intent_text = excluded.intent_text,
			selected_direction_candidate_id = excluded.selected_direction_candidate_id,
			blocking_reason = excluded.blocking_reason,
			active_run_id = excluded.active_run_id,
			archive_digest = excluded.archive_digest,
			archive_idempotency_key = excluded.archive_idempotency_key,
			archive_change_set_id = excluded.archive_change_set_id,
			updated_at = excluded.updated_at
	`,
		session.ID, session.ProjectID, session.BranchID, string(session.Stage), string(session.AdvanceMode),
		session.IntentText, session.SelectedDirectionCandidateID, string(session.BlockingReason), session.ActiveRunID,
		session.ArchiveDigest, session.ArchiveIdempotencyKey, session.ArchiveChangeSetID,
		formatTime(session.CreatedAt), formatTime(session.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save creation session after archive", err)
	}
	return nil
}

func persistCreationCandidateTx(ctx context.Context, tx *sql.Tx, candidate creationdomain.Candidate) error {
	locks, err := json.Marshal(candidate.LockMap)
	if err != nil {
		return err
	}
	if string(locks) == "null" {
		locks = []byte("{}")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE creation_candidates SET
			parent_candidate_id = NULLIF(?, ''),
			supersedes_candidate_id = NULLIF(?, ''),
			lock_map_json = ?,
			status = ?,
			updated_at = ?
		WHERE id = ?
	`, candidate.ParentCandidateID, candidate.SupersedesCandidateID, string(locks),
		string(candidate.Status), formatTime(candidate.UpdatedAt), candidate.ID)
	if err != nil {
		return databaseOperationError("update creation candidate after archive", err)
	}
	return requireOneRow(result, "creation candidate", candidate.ID)
}
