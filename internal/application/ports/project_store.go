package ports

import (
	"context"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
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
	CreateCharacter(ctx context.Context, character worlddomain.Character) error
	UpdateCharacter(ctx context.Context, character worlddomain.Character) error
	ListCharacters(ctx context.Context) ([]worlddomain.Character, error)
	Character(ctx context.Context, id string) (worlddomain.Character, error)
	ArchiveCharacter(ctx context.Context, id string, archivedAt time.Time) error
	CreateFaction(ctx context.Context, faction worlddomain.Faction) error
	UpdateFaction(ctx context.Context, faction worlddomain.Faction) error
	ListFactions(ctx context.Context) ([]worlddomain.Faction, error)
	Faction(ctx context.Context, id string) (worlddomain.Faction, error)
	ArchiveFaction(ctx context.Context, id string, archivedAt time.Time) error
	UpsertMembership(ctx context.Context, membership worlddomain.Membership) error
	ListMemberships(ctx context.Context) ([]worlddomain.Membership, error)
	DeleteMembership(ctx context.Context, characterID, factionID string) error
	CreateRelationship(ctx context.Context, relationship worlddomain.Relationship) error
	ListRelationships(ctx context.Context) ([]worlddomain.Relationship, error)
	ArchiveRelationship(ctx context.Context, id string, archivedAt time.Time) error
	AddChapterAppearance(ctx context.Context, appearance worlddomain.ChapterAppearance) error
	ListChapterAppearances(ctx context.Context) ([]worlddomain.ChapterAppearance, error)
	RemoveChapterAppearance(ctx context.Context, characterID, chapterID string) error
	AddSceneAppearance(ctx context.Context, appearance worlddomain.SceneAppearance) error
	ListSceneAppearances(ctx context.Context) ([]worlddomain.SceneAppearance, error)
	RemoveSceneAppearance(ctx context.Context, characterID, sceneCardID string) error
	Close() error
}

type ProjectStoreFactory interface {
	Create(ctx context.Context, databasePath string, project projectdomain.Project) (ProjectStore, error)
	Open(ctx context.Context, databasePath string) (ProjectStore, error)
}
