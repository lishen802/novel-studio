package sqlite

import (
	"context"
	"encoding/json"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// SaveCreationSample 比较会话与精确方向快照，原子追加试写准备；失败不移动任何状态。
func (s *Store) SaveCreationSample(ctx context.Context, expected creationdomain.Session, direction creationdomain.Candidate, handoff creationdomain.SampleHandoff) error {
	if err := handoff.Validate(direction); err != nil {
		return err
	}
	if handoff.SessionID != expected.ID || expected.SelectedDirectionCandidateID != direction.ID || expected.SelectedDirectionCardID != handoff.DirectionCardID || expected.ActiveRunID != "" || expected.ActiveOperationID != "" {
		return creationdomain.ErrSessionVersionConflict
	}
	raw, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin sample handoff", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE creation_sessions SET content_version=content_version+1 WHERE id=? AND project_id=? AND branch_id=? AND content_version=? AND run_slot_version=? AND active_run_id='' AND COALESCE(active_operation_id,'')=''`, expected.ID, expected.ProjectID, expected.BranchID, expected.ContentVersion, expected.RunSlotVersion)
	if err := creationSessionCASResult(result, err); err != nil {
		return err
	}
	var newer int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation_candidates WHERE session_id=? AND stage='directions' AND version>? AND status NOT IN ('superseded','discarded')`, expected.ID, direction.Version).Scan(&newer); err != nil {
		return databaseOperationError("verify latest sample direction", err)
	}
	if newer != 0 {
		return creationdomain.ErrConfirmationStale
	}
	if err := verifyCreationCandidateTx(ctx, tx, direction); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO creation_sample_handoffs(id,session_id,direction_candidate_id,payload_json) VALUES(?,?,?,?)`, handoff.ID, handoff.SessionID, direction.ID, string(raw)); err != nil {
		return databaseOperationError("insert sample handoff", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit sample handoff", err)
	}
	return nil
}

// LatestCreationSample 读取最近的不可变准备，旧会话没有记录时保留 sql.ErrNoRows。
func (s *Store) LatestCreationSample(ctx context.Context, sessionID string) (creationdomain.SampleHandoff, error) {
	var raw string
	var h creationdomain.SampleHandoff
	if err := s.db.QueryRowContext(ctx, `SELECT payload_json FROM creation_sample_handoffs WHERE session_id=? ORDER BY sequence DESC LIMIT 1`, sessionID).Scan(&raw); err != nil {
		return h, err
	}
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		return h, databaseOperationError("decode sample handoff", err)
	}
	return h, nil
}
