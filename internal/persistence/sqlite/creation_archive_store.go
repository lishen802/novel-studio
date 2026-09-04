package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// ApplyCreationArchive 在同一事务中写入 ChangeSet、正式资产、会话确认和工作台写作阶段。
// verify 是确认前置条件的一部分但不改为 confirmed 的候选（如选中的方向），事务内按快照校验其未被替代/废弃/改锁/改质量。
func (s *Store) ApplyCreationArchive(
	ctx context.Context,
	changeSet revisiondomain.ChangeSet,
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	verify []creationdomain.Candidate,
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
	if err := updateCreationSession(ctx, tx, session); err != nil {
		return err
	}
	if err := updateCreationRunSlot(ctx, tx, session); err != nil {
		return err
	}
	for _, candidate := range verify {
		if err := verifyCreationCandidateTx(ctx, tx, candidate); err != nil {
			return err
		}
	}
	for _, candidate := range candidates {
		if err := persistCreationCandidateTx(ctx, tx, candidate, appliedAt); err != nil {
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

// verifyCreationCandidateTx 只校验不迁移：确认前置依赖候选（如选中的方向）在入档前快照未变，
// 被替代/废弃、改锁、改质量、依赖或影响处理变化都会命中 0 行并整事务回滚。
func verifyCreationCandidateTx(ctx context.Context, tx *sql.Tx, candidate creationdomain.Candidate) error {
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
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM creation_candidates
		WHERE id = ? AND content_sha256 = ? AND quality_status = ? AND lock_map_json = ? AND status = ?
		AND upstream_digest = ? AND excepted_upstream_digest = ? AND impact_severity = ? AND impact_refs_json = ?
	`, candidate.ID, candidate.ContentSHA256, string(candidate.QualityStatus), string(locks), string(candidate.Status),
		candidate.UpstreamDigest, candidate.ExceptedUpstreamDigest, string(candidate.ImpactSeverity), string(refs)).Scan(&count); err != nil {
		return databaseOperationError("verify creation candidate after archive", err)
	}
	if count != 1 {
		return fmt.Errorf("%w: dependency candidate %s changed since preview", creationdomain.ErrConfirmationStale, candidate.ID)
	}
	return nil
}

// persistCreationCandidateTx 把候选确认成正式版本，同时以入档前快照为条件更新。
// 内容哈希、质量结论、锁与状态任一与预览时不一致（改锁、改质量、被替代、新版本），
// 都命中 0 行并整事务回滚，避免把已过期的候选写成 confirmed。
func persistCreationCandidateTx(ctx context.Context, tx *sql.Tx, candidate creationdomain.Candidate, confirmedAt time.Time) error {
	if err := verifyCreationCandidateTx(ctx, tx, candidate); err != nil {
		return err
	}
	locks, err := json.Marshal(candidate.LockMap)
	if err != nil {
		return err
	}
	if string(locks) == "null" {
		locks = []byte("{}")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE creation_candidates SET status = ?, updated_at = ?
		WHERE id = ? AND content_sha256 = ? AND quality_status = ? AND lock_map_json = ? AND status = ?
		AND NOT EXISTS (
			SELECT 1 FROM creation_candidates AS newer
			WHERE newer.session_id = creation_candidates.session_id
			AND newer.stage = creation_candidates.stage
			AND newer.status NOT IN ('superseded', 'discarded')
			AND newer.version >= creation_candidates.version
			AND newer.id != creation_candidates.id
		)
	`, string(creationdomain.StatusConfirmed), formatTime(confirmedAt),
		candidate.ID, candidate.ContentSHA256, string(candidate.QualityStatus), string(locks), string(candidate.Status))
	if err != nil {
		return databaseOperationError("confirm creation candidate after archive", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read creation candidate confirm result", err)
	}
	if changed != 1 {
		return fmt.Errorf("%w: candidate %s changed since preview", creationdomain.ErrConfirmationStale, candidate.ID)
	}
	return nil
}
