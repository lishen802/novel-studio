package sqlite

import (
	"context"
	"database/sql"

	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

// AppendChapterLoop 追加一条不可变的单章产品记录。
func (s *Store) AppendChapterLoop(ctx context.Context, record loopdomain.StoredRecord) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO chapter_loop_records(id, project_id, kind, subject_id, idempotency_key, payload_json) VALUES(?,?,?,?,?,?)`,
		record.ID, record.ProjectID, record.Kind, record.SubjectID, record.IdempotencyKey, record.Payload); err != nil {
		return databaseOperationError("insert chapter loop record", err)
	}
	return nil
}

// LatestChapterLoop 读取某对象最新一条记录。
func (s *Store) LatestChapterLoop(ctx context.Context, projectID, kind, subjectID string) (loopdomain.StoredRecord, error) {
	return scanChapterLoop(s.db.QueryRowContext(ctx, `SELECT id, project_id, kind, subject_id, idempotency_key, payload_json FROM chapter_loop_records WHERE project_id=? AND kind=? AND subject_id=? ORDER BY sequence DESC LIMIT 1`, projectID, kind, subjectID))
}

// GetChapterLoop 按记录标识读取。
func (s *Store) GetChapterLoop(ctx context.Context, projectID, id string) (loopdomain.StoredRecord, error) {
	return scanChapterLoop(s.db.QueryRowContext(ctx, `SELECT id, project_id, kind, subject_id, idempotency_key, payload_json FROM chapter_loop_records WHERE project_id=? AND id=?`, projectID, id))
}

// ListChapterLoop 按追加顺序返回一类记录。
func (s *Store) ListChapterLoop(ctx context.Context, projectID, kind string) ([]loopdomain.StoredRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, kind, subject_id, idempotency_key, payload_json FROM chapter_loop_records WHERE project_id=? AND kind=? ORDER BY sequence ASC`, projectID, kind)
	if err != nil {
		return nil, databaseOperationError("list chapter loop records", err)
	}
	defer rows.Close()
	records := []loopdomain.StoredRecord{}
	for rows.Next() {
		record, err := scanChapterLoop(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate chapter loop records", err)
	}
	return records, nil
}

type chapterLoopScanner interface {
	Scan(dest ...any) error
}

func scanChapterLoop(scanner chapterLoopScanner) (loopdomain.StoredRecord, error) {
	var record loopdomain.StoredRecord
	if err := scanner.Scan(&record.ID, &record.ProjectID, &record.Kind, &record.SubjectID, &record.IdempotencyKey, &record.Payload); err != nil {
		if err == sql.ErrNoRows {
			return loopdomain.StoredRecord{}, err
		}
		return loopdomain.StoredRecord{}, databaseOperationError("scan chapter loop record", err)
	}
	return record, nil
}
