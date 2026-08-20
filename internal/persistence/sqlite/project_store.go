package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

var (
	ErrProjectCount     = errors.New("project database must contain exactly one project")
	ErrChapterMissing   = errors.New("chapter does not exist")
	ErrWorkbenchMissing = fmt.Errorf("%w: workbench state or profile not set", sql.ErrNoRows)
)

func (s *Store) Project(ctx context.Context) (projectdomain.Project, error) {
	return scanProject(s.db.QueryRowContext(ctx, `
		SELECT id, title, genre, summary, created_at, updated_at
		FROM projects
		LIMIT 1
	`))
}

func (s *Store) CreateChapter(ctx context.Context, chapter projectdomain.Chapter) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chapters (
			id, project_id, title, position, body, revision, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		chapter.ID,
		chapter.ProjectID,
		chapter.Title,
		chapter.Position,
		chapter.Body,
		chapter.Revision,
		formatTime(chapter.CreatedAt),
		formatTime(chapter.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create chapter: %w", err)
	}
	return nil
}

func (s *Store) ListChapters(ctx context.Context) ([]projectdomain.Chapter, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, position, body, revision, created_at, updated_at
		FROM chapters
		ORDER BY position ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()

	chapters := make([]projectdomain.Chapter, 0)
	for rows.Next() {
		chapter, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chapters: %w", err)
	}
	return chapters, nil
}

func (s *Store) Chapter(ctx context.Context, id string) (projectdomain.Chapter, error) {
	chapter, err := scanChapter(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, position, body, revision, created_at, updated_at
		FROM chapters
		WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return projectdomain.Chapter{}, fmt.Errorf("%w: %s", ErrChapterMissing, id)
	}
	return chapter, err
}

func (s *Store) SaveDraft(ctx context.Context, chapterID, body string, baseRevision int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin draft transaction", err)
	}
	defer tx.Rollback()

	var currentRevision int64
	err = tx.QueryRowContext(ctx, "SELECT revision FROM chapters WHERE id = ?", chapterID).Scan(&currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrChapterMissing, chapterID)
	}
	if err != nil {
		return databaseOperationError("read chapter revision", err)
	}
	if currentRevision != baseRevision {
		return projectdomain.ErrRevisionConflict
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO drafts (chapter_id, body, base_revision, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(chapter_id) DO UPDATE SET
			body = excluded.body,
			base_revision = excluded.base_revision,
			updated_at = excluded.updated_at
	`, chapterID, body, baseRevision, formatTime(time.Now()))
	if err != nil {
		return databaseOperationError("save chapter draft", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit chapter draft", err)
	}
	return nil
}

func (s *Store) Draft(ctx context.Context, chapterID string) (string, int64, time.Time, bool, error) {
	var body string
	var baseRevision int64
	var updatedAtValue string
	err := s.db.QueryRowContext(ctx, `
		SELECT body, base_revision, updated_at
		FROM drafts
		WHERE chapter_id = ?
	`, chapterID).Scan(&body, &baseRevision, &updatedAtValue)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, time.Time{}, false, nil
	}
	if err != nil {
		return "", 0, time.Time{}, false, fmt.Errorf("read chapter draft: %w", err)
	}
	updatedAt, err := parseTime(updatedAtValue)
	if err != nil {
		return "", 0, time.Time{}, false, fmt.Errorf("parse draft updated_at: %w", err)
	}
	return body, baseRevision, updatedAt, true, nil
}

func (s *Store) DeleteDraft(ctx context.Context, chapterID string, baseRevision int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM drafts
		WHERE chapter_id = ? AND base_revision = ?
	`, chapterID, baseRevision)
	if err != nil {
		return databaseOperationError("delete chapter draft", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read deleted draft count", err)
	}
	if deleted == 1 {
		return nil
	}
	var currentRevision int64
	err = s.db.QueryRowContext(ctx, "SELECT base_revision FROM drafts WHERE chapter_id = ?", chapterID).Scan(&currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return databaseOperationError("read recovery draft revision", err)
	}
	return projectdomain.ErrRevisionConflict
}

func (s *Store) SaveProfileSelection(ctx context.Context, selection projectdomain.ProfileSelection, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_creative_profile (id, profile_id, profile_version, role_overrides_json, updated_at)
		VALUES (1, ?, ?, '{}', ?)
		ON CONFLICT(id) DO UPDATE SET
			profile_id = excluded.profile_id,
			profile_version = excluded.profile_version,
			role_overrides_json = excluded.role_overrides_json,
			updated_at = excluded.updated_at
	`, selection.ProfileID, selection.Version, formatTime(updatedAt))
	if err != nil {
		return databaseOperationError("save project profile selection", err)
	}
	return nil
}

func (s *Store) ProfileSelection(ctx context.Context) (projectdomain.ProfileSelection, error) {
	var selection projectdomain.ProfileSelection
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT profile_id, profile_version, updated_at
		FROM project_creative_profile
		WHERE id = 1
	`).Scan(&selection.ProfileID, &selection.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return projectdomain.ProfileSelection{}, ErrWorkbenchMissing
	}
	if err != nil {
		return projectdomain.ProfileSelection{}, fmt.Errorf("read project profile selection: %w", err)
	}
	return selection, nil
}

func (s *Store) SaveWorkbenchState(ctx context.Context, state projectdomain.WorkbenchState) error {
	_, err := s.db.ExecContext(ctx, `
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
		string(state.Stage),
		state.Canvas,
		state.AIPanel,
		state.PendingReviewCount,
		state.RecoverableJobCount,
		state.ArchiveStatus,
		formatTime(state.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save project workbench state", err)
	}
	return nil
}

func (s *Store) WorkbenchState(ctx context.Context) (projectdomain.WorkbenchState, error) {
	var state projectdomain.WorkbenchState
	var stage string
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT stage, canvas, ai_panel, pending_review_count,
			recoverable_job_count, archive_status, updated_at
		FROM project_workbench_state
		WHERE id = 1
	`).Scan(
		&stage,
		&state.Canvas,
		&state.AIPanel,
		&state.PendingReviewCount,
		&state.RecoverableJobCount,
		&state.ArchiveStatus,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return projectdomain.WorkbenchState{}, ErrWorkbenchMissing
	}
	if err != nil {
		return projectdomain.WorkbenchState{}, fmt.Errorf("read project workbench state: %w", err)
	}
	state.Stage = projectdomain.WorkbenchStage(stage)
	state.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return projectdomain.WorkbenchState{}, fmt.Errorf("parse workbench updated_at: %w", err)
	}
	return state, nil
}

func (s *Store) CommitChapter(ctx context.Context, chapter, previous projectdomain.Chapter) error {
	if chapter.ID != previous.ID || chapter.ProjectID != previous.ProjectID || chapter.Revision != previous.Revision+1 {
		return projectdomain.ErrRevisionConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin chapter transaction", err)
	}
	defer tx.Rollback()

	var currentRevision int64
	var currentSource, currentSummary string
	err = tx.QueryRowContext(ctx, `
		SELECT revision, revision_source, revision_summary
		FROM chapters WHERE id = ?
	`, previous.ID).Scan(&currentRevision, &currentSource, &currentSummary)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrChapterMissing, previous.ID)
	}
	if err != nil {
		return databaseOperationError("read chapter revision", err)
	}
	if currentRevision != previous.Revision {
		return projectdomain.ErrRevisionConflict
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO chapter_revisions (
			chapter_id, revision, title, body, source, summary, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		previous.ID,
		previous.Revision,
		previous.Title,
		previous.Body,
		currentSource,
		currentSummary,
		formatTime(chapter.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("archive chapter revision", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE chapters
		SET title = ?, body = ?, revision = ?, revision_source = ?, revision_summary = ?, updated_at = ?
		WHERE id = ? AND revision = ?
	`,
		chapter.Title,
		chapter.Body,
		chapter.Revision,
		"author",
		"",
		formatTime(chapter.UpdatedAt),
		chapter.ID,
		previous.Revision,
	)
	if err != nil {
		return databaseOperationError("update chapter", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read updated chapter count", err)
	}
	if rowsAffected != 1 {
		return projectdomain.ErrRevisionConflict
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM drafts WHERE chapter_id = ?", chapter.ID); err != nil {
		return databaseOperationError("clear chapter draft", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit chapter transaction", err)
	}
	return nil
}

var _ ports.ProjectStore = (*Store)(nil)

func scanProject(row scanner) (projectdomain.Project, error) {
	var project projectdomain.Project
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&project.ID,
		&project.Title,
		&project.Genre,
		&project.Summary,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return projectdomain.Project{}, err
	}
	project.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return projectdomain.Project{}, fmt.Errorf("parse project created_at: %w", err)
	}
	project.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return projectdomain.Project{}, fmt.Errorf("parse project updated_at: %w", err)
	}
	return project, nil
}

func scanChapter(row scanner) (projectdomain.Chapter, error) {
	var chapter projectdomain.Chapter
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&chapter.ID,
		&chapter.ProjectID,
		&chapter.Title,
		&chapter.Position,
		&chapter.Body,
		&chapter.Revision,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return projectdomain.Chapter{}, err
	}
	chapter.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return projectdomain.Chapter{}, fmt.Errorf("parse chapter created_at: %w", err)
	}
	chapter.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return projectdomain.Chapter{}, fmt.Errorf("parse chapter updated_at: %w", err)
	}
	return chapter, nil
}
