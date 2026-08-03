package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func (s *Store) CreateImportJob(
	ctx context.Context,
	job importing.ImportJob,
	chapters []importing.ImportChapter,
) error {
	if job.Status != importing.JobStatusPrepared ||
		job.ChapterCount != len(chapters) ||
		job.SelectedChapterCount != 0 ||
		job.AppliedAt != nil {
		return ports.ErrImportChapterMappingMismatch
	}
	seenPositions := make(map[int]struct{}, len(chapters))
	for _, chapter := range chapters {
		bodyHash := sha256.Sum256([]byte(chapter.Body))
		if chapter.JobID != job.ID ||
			chapter.Selected ||
			chapter.CreatedChapterID != "" ||
			chapter.ContentHash != hex.EncodeToString(bodyHash[:]) {
			return ports.ErrImportChapterMappingMismatch
		}
		if _, exists := seenPositions[chapter.Position]; exists {
			return ports.ErrImportChapterMappingMismatch
		}
		seenPositions[chapter.Position] = struct{}{}
	}

	tx, err := beginImmediate(ctx, s.db)
	if err != nil {
		return safeImportStoreError("begin create import job", err)
	}
	defer tx.Rollback()

	var existingSourceID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM import_jobs
		WHERE project_id = ? AND source_hash = ?
	`, job.ProjectID, job.SourceHash).Scan(&existingSourceID)
	if err == nil {
		return ports.ErrDuplicateImportSource
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return safeImportStoreError("check duplicate import source", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO import_jobs(
			id, project_id, source_name, source_format, source_encoding,
			source_hash, source_bytes, archived_relative_path, status,
			chapter_count, selected_chapter_count, error_code, error_message,
			created_at, updated_at, applied_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.ID,
		job.ProjectID,
		job.SourceName,
		job.SourceFormat,
		job.SourceEncoding,
		job.SourceHash,
		job.SourceBytes,
		job.ArchivedRelativePath,
		job.Status,
		job.ChapterCount,
		job.SelectedChapterCount,
		job.ErrorCode,
		job.ErrorMessage,
		formatTime(job.CreatedAt),
		formatTime(job.UpdatedAt),
		formatOptionalTime(job.AppliedAt),
	)
	if err != nil {
		return safeImportStoreError("create import job", err)
	}

	for _, chapter := range chapters {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO import_chapters(
				id, job_id, volume_title, title, body, position,
				source_start_line, source_end_line, content_hash,
				selected, created_chapter_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			chapter.ID,
			chapter.JobID,
			chapter.VolumeTitle,
			chapter.Title,
			chapter.Body,
			chapter.Position,
			chapter.SourceStartLine,
			chapter.SourceEndLine,
			chapter.ContentHash,
			boolToSQLite(chapter.Selected),
			nil,
		)
		if err != nil {
			return safeImportStoreError("create import chapter", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return safeImportStoreError("commit create import job", err)
	}
	return nil
}

func (s *Store) CreateFailedImportJob(ctx context.Context, job importing.ImportJob) error {
	if job.Status != importing.JobStatusFailed ||
		job.ChapterCount != 0 ||
		job.SelectedChapterCount != 0 ||
		strings.TrimSpace(job.ErrorCode) == "" ||
		strings.TrimSpace(job.ErrorMessage) == "" ||
		job.AppliedAt != nil {
		return ports.ErrImportChapterMappingMismatch
	}

	tx, err := beginImmediate(ctx, s.db)
	if err != nil {
		return safeImportStoreError("begin create failed import job", err)
	}
	defer tx.Rollback()

	var existingSourceID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM import_jobs
		WHERE project_id = ? AND source_hash = ?
	`, job.ProjectID, job.SourceHash).Scan(&existingSourceID)
	if err == nil {
		return ports.ErrDuplicateImportSource
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return safeImportStoreError("check duplicate failed import source", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO import_jobs(
			id, project_id, source_name, source_format, source_encoding,
			source_hash, source_bytes, archived_relative_path, status,
			chapter_count, selected_chapter_count, error_code, error_message,
			created_at, updated_at, applied_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.ID,
		job.ProjectID,
		job.SourceName,
		job.SourceFormat,
		job.SourceEncoding,
		job.SourceHash,
		job.SourceBytes,
		job.ArchivedRelativePath,
		job.Status,
		job.ChapterCount,
		job.SelectedChapterCount,
		job.ErrorCode,
		job.ErrorMessage,
		formatTime(job.CreatedAt),
		formatTime(job.UpdatedAt),
		nil,
	)
	if err != nil {
		return safeImportStoreError("create failed import job", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return safeImportStoreError("commit create failed import job", err)
	}
	return nil
}

func (s *Store) ListImportJobs(
	ctx context.Context,
	projectID string,
) ([]importing.ImportJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, project_id, source_name, source_format, source_encoding,
			source_hash, source_bytes, archived_relative_path, status,
			chapter_count, selected_chapter_count, error_code, error_message,
			created_at, updated_at, applied_at
		FROM import_jobs
		WHERE project_id = ?
		ORDER BY created_at DESC, id ASC
	`, projectID)
	if err != nil {
		return nil, safeImportStoreError("list import jobs", err)
	}
	defer rows.Close()

	jobs := make([]importing.ImportJob, 0)
	for rows.Next() {
		job, err := scanImportJob(rows)
		if err != nil {
			return nil, safeImportStoreError("scan import job", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, safeImportStoreError("iterate import jobs", err)
	}
	return jobs, nil
}

func (s *Store) ImportJob(
	ctx context.Context,
	id string,
) (ports.ImportJobDetails, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ports.ImportJobDetails{}, safeImportStoreError("begin read import job", err)
	}
	defer tx.Rollback()

	job, err := loadImportJob(ctx, tx, id)
	if err != nil {
		return ports.ImportJobDetails{}, err
	}
	if s.importJobReadHook != nil {
		s.importJobReadHook()
	}
	chapters, err := loadImportChapters(ctx, tx, id)
	if err != nil {
		return ports.ImportJobDetails{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.ImportJobDetails{}, safeImportStoreError("commit read import job", err)
	}
	return ports.ImportJobDetails{Job: job, Chapters: chapters}, nil
}

func (s *Store) DeleteImportJob(ctx context.Context, id string) error {
	tx, err := beginImmediate(ctx, s.db)
	if err != nil {
		return safeImportStoreError("begin delete import job", err)
	}
	defer tx.Rollback()

	job, err := loadImportJob(ctx, tx, id)
	if err != nil {
		return err
	}
	switch job.Status {
	case importing.JobStatusApplied:
		return ports.ErrImportJobAlreadyApplied
	case importing.JobStatusPrepared, importing.JobStatusFailed:
	default:
		return ports.ErrImportJobNotPrepared
	}
	result, err := tx.ExecContext(ctx, `
		DELETE FROM import_jobs
		WHERE id = ? AND status IN ('prepared', 'failed')
	`, id)
	if err != nil {
		return safeImportStoreError("delete import job", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return safeImportStoreError("read deleted import job count", err)
	}
	if deleted != 1 {
		return ports.ErrImportJobNotPrepared
	}
	if err := tx.Commit(ctx); err != nil {
		return safeImportStoreError("commit delete import job", err)
	}
	return nil
}

// DeletePreparedImportJob is retained for callers compiled against the
// pre-import-use-case concrete Store API. New code should use DeleteImportJob.
func (s *Store) DeletePreparedImportJob(ctx context.Context, id string) error {
	return s.DeleteImportJob(ctx, id)
}

func (s *Store) ApplyImportJob(
	ctx context.Context,
	jobID string,
	selectedImportChapterIDs []string,
	chapters []projectdomain.Chapter,
	appliedAt time.Time,
) (ports.ImportApplyResult, error) {
	if len(selectedImportChapterIDs) == 0 {
		return ports.ImportApplyResult{}, ports.ErrNoImportChaptersSelected
	}
	if len(selectedImportChapterIDs) != len(chapters) {
		return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
	}

	chapterByCandidateID := make(map[string]projectdomain.Chapter, len(chapters))
	createdChapterIDs := make(map[string]struct{}, len(chapters))
	for index, candidateID := range selectedImportChapterIDs {
		candidateID = strings.TrimSpace(candidateID)
		chapterID := strings.TrimSpace(chapters[index].ID)
		if candidateID == "" || chapterID == "" {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		if _, exists := chapterByCandidateID[candidateID]; exists {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		if _, exists := createdChapterIDs[chapterID]; exists {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		chapterByCandidateID[candidateID] = chapters[index]
		createdChapterIDs[chapterID] = struct{}{}
	}

	tx, err := beginImmediate(ctx, s.db)
	if err != nil {
		return ports.ImportApplyResult{}, safeImportStoreError("begin apply import job", err)
	}
	defer tx.Rollback()

	job, err := loadImportJob(ctx, tx, jobID)
	if err != nil {
		return ports.ImportApplyResult{}, err
	}
	switch job.Status {
	case importing.JobStatusApplied:
		return ports.ImportApplyResult{}, ports.ErrImportJobAlreadyApplied
	case importing.JobStatusPrepared:
	default:
		return ports.ImportApplyResult{}, ports.ErrImportJobNotPrepared
	}

	candidates, err := loadImportChapters(ctx, tx, jobID)
	if err != nil {
		return ports.ImportApplyResult{}, err
	}
	selectedCandidates := make([]importing.ImportChapter, 0, len(chapters))
	for _, candidate := range candidates {
		if _, selected := chapterByCandidateID[candidate.ID]; selected {
			if candidate.Selected || candidate.CreatedChapterID != "" {
				return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
			}
			selectedCandidates = append(selectedCandidates, candidate)
		}
	}
	if len(selectedCandidates) != len(chapterByCandidateID) {
		return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
	}

	var maxPosition int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), 0)
		FROM chapters
		WHERE project_id = ?
	`, job.ProjectID).Scan(&maxPosition); err != nil {
		return ports.ImportApplyResult{}, safeImportStoreError("read maximum chapter position", err)
	}
	if s.importApplyAfterMaxPositionHook != nil {
		s.importApplyAfterMaxPositionHook()
	}

	appliedAt = appliedAt.UTC()
	created := make([]projectdomain.Chapter, 0, len(selectedCandidates))
	for index, candidate := range selectedCandidates {
		requestedChapter := chapterByCandidateID[candidate.ID]
		position := maxPosition + index + 1
		if requestedChapter.ProjectID != job.ProjectID ||
			requestedChapter.Title != candidate.Title ||
			requestedChapter.Body != candidate.Body ||
			requestedChapter.Revision != 1 ||
			(requestedChapter.Position != 0 && requestedChapter.Position != position) {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		chapter, err := projectdomain.NewChapter(
			requestedChapter.ID,
			job.ProjectID,
			candidate.Title,
			position,
			appliedAt,
		)
		if err != nil {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		chapter.Body = candidate.Body

		_, err = tx.ExecContext(ctx, `
			INSERT INTO chapters(
				id, project_id, title, position, body, revision,
				revision_source, revision_summary, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, 1, 'initial', ?, ?, ?)
		`,
			chapter.ID,
			chapter.ProjectID,
			chapter.Title,
			chapter.Position,
			chapter.Body,
			importRevisionSummary(job.SourceName),
			formatTime(chapter.CreatedAt),
			formatTime(chapter.UpdatedAt),
		)
		if err != nil {
			return ports.ImportApplyResult{}, safeImportStoreError("create imported chapter", err)
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE import_chapters
			SET selected = 1, created_chapter_id = ?
			WHERE id = ? AND job_id = ? AND selected = 0 AND created_chapter_id IS NULL
		`, chapter.ID, candidate.ID, job.ID)
		if err != nil {
			return ports.ImportApplyResult{}, safeImportStoreError("map imported chapter", err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return ports.ImportApplyResult{}, safeImportStoreError("read mapped import chapter count", err)
		}
		if updated != 1 {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		created = append(created, chapter)
	}

	appliedJob, err := job.MarkApplied(len(created), appliedAt)
	if err != nil {
		return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE import_jobs
		SET status = ?, selected_chapter_count = ?, error_code = '', error_message = '',
			updated_at = ?, applied_at = ?
		WHERE id = ? AND status = 'prepared'
	`,
		appliedJob.Status,
		appliedJob.SelectedChapterCount,
		formatTime(appliedJob.UpdatedAt),
		formatOptionalTime(appliedJob.AppliedAt),
		appliedJob.ID,
	)
	if err != nil {
		return ports.ImportApplyResult{}, safeImportStoreError("mark import job applied", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return ports.ImportApplyResult{}, safeImportStoreError("read applied import job count", err)
	}
	if updated != 1 {
		return ports.ImportApplyResult{}, ports.ErrImportJobAlreadyApplied
	}

	if err := tx.Commit(ctx); err != nil {
		return ports.ImportApplyResult{}, safeImportStoreError("commit apply import job", err)
	}
	return ports.ImportApplyResult{Job: appliedJob, Chapters: created}, nil
}

type immediateTx struct {
	conn      *sql.Conn
	completed bool
}

var errImportWriteLockTimeout = errors.New("import write lock wait timed out")

const (
	importWriteLockRetryInterval = 10 * time.Millisecond
	importWriteLockWaitLimit     = 5 * time.Second
	importSQLiteBusyTimeoutMS    = 5000
)

func beginImmediate(ctx context.Context, db *sql.DB) (*immediateTx, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	restoreAndClose := func() {
		restoreSQLiteBusyTimeout(conn)
		_ = conn.Close()
	}

	waitLimit := time.NewTimer(importWriteLockWaitLimit)
	defer waitLimit.Stop()
	for {
		_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err == nil {
			if err := restoreSQLiteBusyTimeout(conn); err != nil {
				_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
				restoreAndClose()
				return nil, err
			}
			return &immediateTx{conn: conn}, nil
		}
		if !isSQLiteBusy(err) {
			restoreAndClose()
			return nil, err
		}

		retry := time.NewTimer(importWriteLockRetryInterval)
		select {
		case <-ctx.Done():
			if !retry.Stop() {
				<-retry.C
			}
			restoreAndClose()
			return nil, ctx.Err()
		case <-waitLimit.C:
			if !retry.Stop() {
				<-retry.C
			}
			restoreAndClose()
			return nil, errImportWriteLockTimeout
		case <-retry.C:
		}
	}
}

func (tx *immediateTx) ExecContext(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	return tx.conn.ExecContext(ctx, query, args...)
}

func (tx *immediateTx) QueryContext(
	ctx context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	return tx.conn.QueryContext(ctx, query, args...)
}

func (tx *immediateTx) QueryRowContext(
	ctx context.Context,
	query string,
	args ...any,
) *sql.Row {
	return tx.conn.QueryRowContext(ctx, query, args...)
}

func (tx *immediateTx) Commit(ctx context.Context) error {
	if tx.completed {
		return sql.ErrTxDone
	}
	if _, err := tx.conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	tx.completed = true
	return tx.conn.Close()
}

func (tx *immediateTx) Rollback() {
	if tx == nil || tx.completed {
		return
	}
	rollbackCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = tx.conn.ExecContext(rollbackCtx, "ROLLBACK")
	tx.completed = true
	_ = tx.conn.Close()
}

func restoreSQLiteBusyTimeout(conn *sql.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := conn.ExecContext(ctx, fmt.Sprintf(
		"PRAGMA busy_timeout = %d",
		importSQLiteBusyTimeoutMS,
	))
	return err
}

func isSQLiteBusy(err error) bool {
	var sqliteError interface{ Code() int }
	if !errors.As(err, &sqliteError) {
		return false
	}
	const (
		sqliteBusy   = 5
		sqliteLocked = 6
	)
	primaryCode := sqliteError.Code() & 0xff
	return primaryCode == sqliteBusy || primaryCode == sqliteLocked
}

type importStoreQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadImportJob(
	ctx context.Context,
	query importStoreQuery,
	id string,
) (importing.ImportJob, error) {
	job, err := scanImportJob(query.QueryRowContext(ctx, `
		SELECT
			id, project_id, source_name, source_format, source_encoding,
			source_hash, source_bytes, archived_relative_path, status,
			chapter_count, selected_chapter_count, error_code, error_message,
			created_at, updated_at, applied_at
		FROM import_jobs
		WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return importing.ImportJob{}, ports.ErrImportJobMissing
	}
	if err != nil {
		return importing.ImportJob{}, safeImportStoreError("read import job", err)
	}
	return job, nil
}

func loadImportChapters(
	ctx context.Context,
	query importStoreQuery,
	jobID string,
) ([]importing.ImportChapter, error) {
	rows, err := query.QueryContext(ctx, `
		SELECT
			id, job_id, volume_title, title, body, position,
			source_start_line, source_end_line, content_hash,
			selected, created_chapter_id
		FROM import_chapters
		WHERE job_id = ?
		ORDER BY position ASC, id ASC
	`, jobID)
	if err != nil {
		return nil, safeImportStoreError("list import chapters", err)
	}
	defer rows.Close()

	chapters := make([]importing.ImportChapter, 0)
	for rows.Next() {
		chapter, err := scanImportChapter(rows)
		if err != nil {
			return nil, safeImportStoreError("scan import chapter", err)
		}
		chapters = append(chapters, chapter)
	}
	if err := rows.Err(); err != nil {
		return nil, safeImportStoreError("iterate import chapters", err)
	}
	return chapters, nil
}

func scanImportJob(row scanner) (importing.ImportJob, error) {
	var job importing.ImportJob
	var sourceFormat, status string
	var createdAt, updatedAt string
	var appliedAt sql.NullString
	err := row.Scan(
		&job.ID,
		&job.ProjectID,
		&job.SourceName,
		&sourceFormat,
		&job.SourceEncoding,
		&job.SourceHash,
		&job.SourceBytes,
		&job.ArchivedRelativePath,
		&status,
		&job.ChapterCount,
		&job.SelectedChapterCount,
		&job.ErrorCode,
		&job.ErrorMessage,
		&createdAt,
		&updatedAt,
		&appliedAt,
	)
	if err != nil {
		return importing.ImportJob{}, err
	}
	job.SourceFormat = importing.SourceFormat(sourceFormat)
	job.Status = importing.JobStatus(status)
	job.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return importing.ImportJob{}, err
	}
	job.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return importing.ImportJob{}, err
	}
	if appliedAt.Valid {
		value, err := parseTime(appliedAt.String)
		if err != nil {
			return importing.ImportJob{}, err
		}
		job.AppliedAt = &value
	}
	return job, nil
}

func scanImportChapter(row scanner) (importing.ImportChapter, error) {
	var chapter importing.ImportChapter
	var selected int
	var createdChapterID sql.NullString
	err := row.Scan(
		&chapter.ID,
		&chapter.JobID,
		&chapter.VolumeTitle,
		&chapter.Title,
		&chapter.Body,
		&chapter.Position,
		&chapter.SourceStartLine,
		&chapter.SourceEndLine,
		&chapter.ContentHash,
		&selected,
		&createdChapterID,
	)
	if err != nil {
		return importing.ImportChapter{}, err
	}
	chapter.Selected = selected == 1
	if createdChapterID.Valid {
		chapter.CreatedChapterID = createdChapterID.String
	}
	return chapter, nil
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func boolToSQLite(value bool) int {
	if value {
		return 1
	}
	return 0
}

func safeImportStoreError(operation string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", operation, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, context.DeadlineExceeded)
	}
	classified := classifyDatabaseError(err)
	for _, stable := range []error{
		ErrDatabasePermission,
		ErrDatabaseCorrupt,
		ErrDatabaseWrite,
		ErrDatabaseMissing,
	} {
		if errors.Is(classified, stable) {
			return fmt.Errorf("%s: %w", operation, stable)
		}
	}
	return fmt.Errorf("%s: %w", operation, ports.ErrImportPersistence)
}

func importRevisionSummary(sourceName string) string {
	return "从导入文件 " + sourceName + " 创建"
}

var _ ports.ImportStore = (*Store)(nil)
