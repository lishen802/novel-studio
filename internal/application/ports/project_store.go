package ports

import (
	"context"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
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
	CreateOutlineNode(ctx context.Context, node storydomain.OutlineNode) error
	ListOutlineNodes(ctx context.Context) ([]storydomain.OutlineNode, error)
	ArchiveOutlineNode(ctx context.Context, id string, archivedAt time.Time) error
	SwapOutlineNodePositions(ctx context.Context, firstID, secondID string, updatedAt time.Time) error
	CreateStoryLine(ctx context.Context, line storydomain.StoryLine) error
	ListStoryLines(ctx context.Context) ([]storydomain.StoryLine, error)
	LinkStoryLine(ctx context.Context, projectID string, link storydomain.StoryLineLink) error
	ListStoryLineLinks(ctx context.Context) ([]storydomain.StoryLineLink, error)
	CreateSceneCard(ctx context.Context, card storydomain.SceneCard) error
	ListSceneCards(ctx context.Context, chapterID string) ([]storydomain.SceneCard, error)
	Close() error
}

type ProjectStoreFactory interface {
	Create(ctx context.Context, databasePath string, project projectdomain.Project) (ProjectStore, error)
	Open(ctx context.Context, databasePath string) (ProjectStore, error)
}
