package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// AppendCreationEntry 原子追加来源并移动头指针；调用方必须先停止活动运行并刷新会话。
// 本方法只保存准备材料，不将方向阶段标记为接受，也不改变 canonical 档案。
func (s *Store) AppendCreationEntry(ctx context.Context, expected creationdomain.Session, entry creationdomain.EntryRevision) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if entry.SessionID != expected.ID || expected.ActiveRunID != "" || expected.ActiveOperationID != "" {
		return creationdomain.ErrSessionVersionConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin creation entry revision", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE creation_sessions SET content_version=content_version+1,
  advance_mode='step_review',
  updated_at=CASE WHEN RTRIM(updated_at,'Z') < RTRIM(?,'Z') THEN ? ELSE updated_at END
  WHERE id=? AND project_id=? AND branch_id=? AND content_version=? AND run_slot_version=?
  AND active_run_id='' AND COALESCE(active_operation_id,'')=''`,
		formatTime(entry.CreatedAt), formatTime(entry.CreatedAt), expected.ID, expected.ProjectID, expected.BranchID, expected.ContentVersion, expected.RunSlotVersion)
	if err := creationSessionCASResult(result, err); err != nil {
		return err
	}
	var parentID string
	err = tx.QueryRowContext(ctx, `SELECT entry_id FROM creation_entry_heads WHERE session_id=?`, expected.ID).Scan(&parentID)
	if errors.Is(err, sql.ErrNoRows) {
		if entry.Version != 1 || entry.ParentID != "" {
			return creationdomain.ErrSessionVersionConflict
		}
	} else if err != nil {
		return databaseOperationError("read creation entry head", err)
	} else {
		parent, err := readCreationEntry(ctx, tx, expected.ID, parentID)
		if err != nil {
			return err
		}
		if entry.ParentID != parent.ID || entry.Version != parent.Version+1 || entry.CreatedAt.Before(parent.CreatedAt) {
			return creationdomain.ErrSessionVersionConflict
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creation_entry_revisions
  (id,session_id,parent_id,version,kind,material,material_sha256,created_at) VALUES (?,?,NULLIF(?,''),?,?,?,?,?)`,
		entry.ID, entry.SessionID, entry.ParentID, entry.Version, string(entry.Kind), entry.Material, entry.MaterialSHA256, formatTime(entry.CreatedAt))
	if err != nil {
		return databaseOperationError("insert creation entry revision", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creation_entry_heads(session_id,entry_id) VALUES(?,?)
  ON CONFLICT(session_id) DO UPDATE SET entry_id=excluded.entry_id`, entry.SessionID, entry.ID)
	if err != nil {
		return databaseOperationError("update creation entry head", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit creation entry revision", err)
	}
	return nil
}

// CreationEntry 按会话与来源 ID 读取准确历史版本，禁止跨会话读取。
func (s *Store) CreationEntry(ctx context.Context, sessionID, entryID string) (creationdomain.EntryRevision, error) {
	return readCreationEntry(ctx, s.db, sessionID, entryID)
}

// CurrentCreationEntry 读取当前材料；旧会话没有来源时返回 sql.ErrNoRows，不伪造历史。
func (s *Store) CurrentCreationEntry(ctx context.Context, sessionID string) (creationdomain.EntryRevision, error) {
	return readCreationEntry(ctx, s.db, sessionID, "")
}

func readCreationEntry(ctx context.Context, reader creationSessionReader, sessionID, entryID string) (creationdomain.EntryRevision, error) {
	var entry creationdomain.EntryRevision
	var created string
	err := reader.QueryRowContext(ctx, `SELECT id,session_id,COALESCE(parent_id,''),version,kind,material,material_sha256,created_at
  FROM creation_entry_revisions WHERE session_id=? AND id=CASE WHEN ?='' THEN
  (SELECT entry_id FROM creation_entry_heads WHERE session_id=?) ELSE ? END`, sessionID, entryID, sessionID, entryID).Scan(
		&entry.ID, &entry.SessionID, &entry.ParentID, &entry.Version, &entry.Kind, &entry.Material, &entry.MaterialSHA256, &created)
	if err != nil {
		return creationdomain.EntryRevision{}, fmt.Errorf("read creation entry revision: %w", err)
	}
	entry.CreatedAt, err = parseTime(created)
	if err != nil {
		return creationdomain.EntryRevision{}, fmt.Errorf("parse creation entry time: %w", err)
	}
	if err := entry.Validate(); err != nil {
		return creationdomain.EntryRevision{}, err
	}
	return entry, nil
}
