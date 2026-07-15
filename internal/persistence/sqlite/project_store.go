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
	ErrProjectCount   = errors.New("project database must contain exactly one project")
	ErrChapterMissing = errors.New("chapter does not exist")
)

type Factory struct{}

func (Factory) Create(ctx context.Context, databasePath string, project projectdomain.Project) (ports.ProjectStore, error) {
	db, err := OpenDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}

	_, err = db.ExecContext(ctx, `
		INSERT INTO projects (id, title, genre, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		project.ID,
		project.Title,
		project.Genre,
		project.Summary,
		formatTime(project.CreatedAt),
		formatTime(project.UpdatedAt),
	)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("create project record: %w", err)
	}
	return store, nil
}

func (Factory) Open(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	db, err := OpenDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("count projects: %w", err)
	}
	if count != 1 {
		_ = store.Close()
		return nil, fmt.Errorf("%w: found %d", ErrProjectCount, count)
	}
	return store, nil
}

type Store struct {
	db *sql.DB
}

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
		return fmt.Errorf("begin draft transaction: %w", err)
	}
	defer tx.Rollback()

	var currentRevision int64
	err = tx.QueryRowContext(ctx, "SELECT revision FROM chapters WHERE id = ?", chapterID).Scan(&currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrChapterMissing, chapterID)
	}
	if err != nil {
		return fmt.Errorf("read chapter revision: %w", err)
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
		return fmt.Errorf("save chapter draft: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chapter draft: %w", err)
	}
	return nil
}

func (s *Store) Draft(ctx context.Context, chapterID string) (string, int64, bool, error) {
	var body string
	var baseRevision int64
	err := s.db.QueryRowContext(ctx, `
		SELECT body, base_revision
		FROM drafts
		WHERE chapter_id = ?
	`, chapterID).Scan(&body, &baseRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, fmt.Errorf("read chapter draft: %w", err)
	}
	return body, baseRevision, true, nil
}

func (s *Store) CommitChapter(ctx context.Context, chapter, previous projectdomain.Chapter) error {
	if chapter.ID != previous.ID || chapter.ProjectID != previous.ProjectID || chapter.Revision != previous.Revision+1 {
		return projectdomain.ErrRevisionConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin chapter transaction: %w", err)
	}
	defer tx.Rollback()

	var currentRevision int64
	err = tx.QueryRowContext(ctx, "SELECT revision FROM chapters WHERE id = ?", previous.ID).Scan(&currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrChapterMissing, previous.ID)
	}
	if err != nil {
		return fmt.Errorf("read chapter revision: %w", err)
	}
	if currentRevision != previous.Revision {
		return projectdomain.ErrRevisionConflict
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO chapter_revisions (chapter_id, revision, title, body, created_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		previous.ID,
		previous.Revision,
		previous.Title,
		previous.Body,
		formatTime(chapter.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("archive chapter revision: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE chapters
		SET title = ?, body = ?, revision = ?, updated_at = ?
		WHERE id = ? AND revision = ?
	`,
		chapter.Title,
		chapter.Body,
		chapter.Revision,
		formatTime(chapter.UpdatedAt),
		chapter.ID,
		previous.Revision,
	)
	if err != nil {
		return fmt.Errorf("update chapter: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated chapter count: %w", err)
	}
	if rowsAffected != 1 {
		return projectdomain.ErrRevisionConflict
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM drafts WHERE chapter_id = ?", chapter.ID); err != nil {
		return fmt.Errorf("clear chapter draft: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chapter transaction: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

var _ ports.ProjectStoreFactory = Factory{}
var _ ports.ProjectStore = (*Store)(nil)

type scanner interface {
	Scan(dest ...any) error
}

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

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
