package ports

import (
	"context"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type ProjectStore interface {
	Project(ctx context.Context) (projectdomain.Project, error)
	CreateChapter(ctx context.Context, chapter projectdomain.Chapter) error
	ListChapters(ctx context.Context) ([]projectdomain.Chapter, error)
	Chapter(ctx context.Context, id string) (projectdomain.Chapter, error)
	SaveDraft(ctx context.Context, chapterID, body string, baseRevision int64) error
	Draft(ctx context.Context, chapterID string) (body string, baseRevision int64, updatedAt time.Time, found bool, err error)
	DeleteDraft(ctx context.Context, chapterID string, baseRevision int64) error
	CommitChapter(ctx context.Context, chapter projectdomain.Chapter, previous projectdomain.Chapter) error
	Close() error
}

type ProjectStoreFactory interface {
	Create(ctx context.Context, databasePath string, project projectdomain.Project) (ProjectStore, error)
	Open(ctx context.Context, databasePath string) (ProjectStore, error)
}
