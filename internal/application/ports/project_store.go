package ports

import (
	"context"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
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
	CreateChangeSet(ctx context.Context, changeSet revisiondomain.ChangeSet) error
	ListChangeSets(ctx context.Context) ([]revisiondomain.ChangeSet, error)
	ChangeSet(ctx context.Context, id string) (revisiondomain.ChangeSet, error)
	ApplyChangeSet(ctx context.Context, id string, appliedAt time.Time) (projectdomain.Chapter, error)
	RejectChangeSet(ctx context.Context, id string, rejectedAt time.Time) error
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
	CreateWorldRegion(ctx context.Context, region worlddomain.WorldRegion) error
	UpdateWorldRegion(ctx context.Context, region worlddomain.WorldRegion) error
	ListWorldRegions(ctx context.Context) ([]worlddomain.WorldRegion, error)
	WorldRegion(ctx context.Context, id string) (worlddomain.WorldRegion, error)
	ArchiveWorldRegion(ctx context.Context, id string, archivedAt time.Time) error
	UpsertRegionAdjacency(ctx context.Context, adjacency worlddomain.RegionAdjacency) error
	ListRegionAdjacencies(ctx context.Context) ([]worlddomain.RegionAdjacency, error)
	DeleteRegionAdjacency(ctx context.Context, firstRegionID, secondRegionID string) error
	CreateLocation(ctx context.Context, location worlddomain.Location) error
	UpdateLocation(ctx context.Context, location worlddomain.Location) error
	ListLocations(ctx context.Context) ([]worlddomain.Location, error)
	Location(ctx context.Context, id string) (worlddomain.Location, error)
	ArchiveLocation(ctx context.Context, id string, archivedAt time.Time) error
	UpsertLocationLink(ctx context.Context, link worlddomain.LocationLink) error
	ListLocationLinks(ctx context.Context) ([]worlddomain.LocationLink, error)
	DeleteLocationLink(ctx context.Context, locationID string, targetType worlddomain.LocationTargetType, targetID string) error
	CreateItem(ctx context.Context, item worlddomain.Item) error
	UpdateItem(ctx context.Context, item worlddomain.Item) error
	ListItems(ctx context.Context) ([]worlddomain.Item, error)
	Item(ctx context.Context, id string) (worlddomain.Item, error)
	TransferItem(ctx context.Context, item worlddomain.Item, transfer worlddomain.ItemTransfer) error
	ListItemTransfers(ctx context.Context, itemID string) ([]worlddomain.ItemTransfer, error)
	ArchiveItem(ctx context.Context, id string, archivedAt time.Time) error
	CreateWorldRule(ctx context.Context, rule worlddomain.WorldRule) error
	UpdateWorldRule(ctx context.Context, rule worlddomain.WorldRule) error
	ListWorldRules(ctx context.Context) ([]worlddomain.WorldRule, error)
	WorldRule(ctx context.Context, id string) (worlddomain.WorldRule, error)
	ArchiveWorldRule(ctx context.Context, id string, archivedAt time.Time) error
	CreateForeshadowingHook(ctx context.Context, hook worlddomain.ForeshadowingHook) error
	UpdateForeshadowingHook(ctx context.Context, hook worlddomain.ForeshadowingHook) error
	ListForeshadowingHooks(ctx context.Context) ([]worlddomain.ForeshadowingHook, error)
	ForeshadowingHook(ctx context.Context, id string) (worlddomain.ForeshadowingHook, error)
	ArchiveForeshadowingHook(ctx context.Context, id string, archivedAt time.Time) error
	CreateStoryEvent(ctx context.Context, event timelinedomain.StoryEvent) error
	UpdateStoryEvent(ctx context.Context, event timelinedomain.StoryEvent) error
	ListStoryEvents(ctx context.Context) ([]timelinedomain.StoryEvent, error)
	StoryEvent(ctx context.Context, id string) (timelinedomain.StoryEvent, error)
	ArchiveStoryEvent(ctx context.Context, id string, archivedAt time.Time) error
	UpsertStoryEventLink(ctx context.Context, link timelinedomain.EventLink) error
	ListStoryEventLinks(ctx context.Context) ([]timelinedomain.EventLink, error)
	DeleteStoryEventLink(ctx context.Context, eventID string, targetType timelinedomain.EventTargetType, targetID string) error
	CreateFact(ctx context.Context, fact canondomain.Fact) error
	UpdateFact(ctx context.Context, fact canondomain.Fact) error
	ListFacts(ctx context.Context) ([]canondomain.Fact, error)
	Fact(ctx context.Context, id string) (canondomain.Fact, error)
	ArchiveFact(ctx context.Context, id string, archivedAt time.Time) error
	CreateCharacterKnowledge(ctx context.Context, knowledge canondomain.CharacterKnowledge) error
	ListCharacterKnowledge(ctx context.Context) ([]canondomain.CharacterKnowledge, error)
	ArchiveCharacterKnowledge(ctx context.Context, id string, archivedAt time.Time) error
	CreateFactEvidence(ctx context.Context, evidence canondomain.Evidence) error
	ListFactEvidence(ctx context.Context) ([]canondomain.Evidence, error)
	DeleteFactEvidence(ctx context.Context, id string) error
	CreateFactScope(ctx context.Context, scope canondomain.Scope) error
	ListFactScopes(ctx context.Context) ([]canondomain.Scope, error)
	DeleteFactScope(ctx context.Context, id string) error
	CreateFactConflict(ctx context.Context, conflict canondomain.Conflict) error
	UpdateFactConflict(ctx context.Context, conflict canondomain.Conflict) error
	ListFactConflicts(ctx context.Context) ([]canondomain.Conflict, error)
	DeleteFactConflict(ctx context.Context, id string) error
	Close() error
}

type ProjectStoreFactory interface {
	Create(ctx context.Context, databasePath string, project projectdomain.Project) (ProjectStore, error)
	Open(ctx context.Context, databasePath string) (ProjectStore, error)
}
