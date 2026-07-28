package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
)

func TestCreateProjectChecksLicenseBeforeCreatingPackage(t *testing.T) {
	denied := errors.New("license denied")
	events := make([]string, 0)
	service := NewService(
		&fakePackageManager{events: &events},
		&fakeStoreFactory{events: &events},
		&fakeLicense{events: &events, err: denied},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-1"}},
	)

	_, err := service.CreateProject(context.Background(), "/books", "长夜织境", "奇幻", "简介")
	if !errors.Is(err, denied) {
		t.Fatalf("error = %v, want license denied", err)
	}
	if !reflect.DeepEqual(events, []string{"license"}) {
		t.Fatalf("events = %v, want [license]", events)
	}
}

func TestCreateProjectLeavesNoActiveWorkspaceWhenFactoryFails(t *testing.T) {
	factoryFailure := errors.New("database unavailable")
	service := NewService(
		&fakePackageManager{createPackage: testPackage("/books/长夜织境.zjproject")},
		&fakeStoreFactory{createErr: factoryFailure},
		&fakeLicense{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-1"}},
	)

	_, err := service.CreateProject(context.Background(), "/books", "长夜织境", "奇幻", "简介")
	if !errors.Is(err, factoryFailure) {
		t.Fatalf("error = %v, want factory failure", err)
	}
	_, err = service.ListChapters(context.Background())
	if !errors.Is(err, ErrNoActiveProject) {
		t.Fatalf("ListChapters error = %v, want ErrNoActiveProject", err)
	}
}

func TestOpenProjectClosesPreviousStoreAfterNewStoreIsReady(t *testing.T) {
	events := make([]string, 0)
	firstProject := mustProject(t, "project-1", "第一本书")
	secondProject := mustProject(t, "project-2", "第二本书")
	firstStore := newFakeStore(firstProject)
	firstStore.closeEvent = "close-old"
	firstStore.events = &events
	secondStore := newFakeStore(secondProject)
	packages := &fakePackageManager{
		createPackage: testPackage("/books/first.zjproject"),
		openPackage:   testPackage("/books/second.zjproject"),
		events:        &events,
	}
	factory := &fakeStoreFactory{
		createStore: firstStore,
		openStore:   secondStore,
		events:      &events,
	}
	service := NewService(
		packages,
		factory,
		&fakeLicense{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-1"}},
	)
	if _, err := service.CreateProject(context.Background(), "/books", "第一本书", "", ""); err != nil {
		t.Fatal(err)
	}
	events = events[:0]

	view, err := service.OpenProject(context.Background(), "/books/second.zjproject")
	if err != nil {
		t.Fatal(err)
	}
	if view.ID != secondProject.ID || view.Path != packages.openPackage.Root {
		t.Fatalf("view = %#v", view)
	}
	if !reflect.DeepEqual(events, []string{"open-package", "open-store", "close-old"}) {
		t.Fatalf("events = %v", events)
	}
}

func TestOpenProjectFailureKeepsCurrentWorkspaceActive(t *testing.T) {
	firstProject := mustProject(t, "project-1", "第一本书")
	firstStore := newFakeStore(firstProject)
	chapter := mustChapter(t, "chapter-1", firstProject.ID, "第一章", 1)
	firstStore.chapters[chapter.ID] = chapter
	service := NewService(
		&fakePackageManager{
			createPackage: testPackage("/books/first.zjproject"),
			openPackage:   testPackage("/books/broken.zjproject"),
		},
		&fakeStoreFactory{
			createStore: firstStore,
			openErr:     sqlite.ErrDatabaseCorrupt,
		},
		&fakeLicense{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-1"}},
	)
	if _, err := service.CreateProject(context.Background(), "/books", "第一本书", "", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := service.OpenProject(context.Background(), "/books/broken.zjproject"); !errors.Is(err, sqlite.ErrDatabaseCorrupt) {
		t.Fatalf("error = %v, want ErrDatabaseCorrupt", err)
	}
	chapters, err := service.ListChapters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 || chapters[0].ID != chapter.ID {
		t.Fatalf("active workspace changed after failed open: %#v", chapters)
	}
}

func TestCreateChapterAssignsNextPosition(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	store.chapters["chapter-1"] = mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters["chapter-3"] = mustChapter(t, "chapter-3", project.ID, "第三章", 3)
	service := openTestService(t, store, &sequenceIDs{ids: []string{"chapter-4"}})

	view, err := service.CreateChapter(context.Background(), "  第四章  ")
	if err != nil {
		t.Fatal(err)
	}
	if view.ID != "chapter-4" || view.Position != 4 || view.Title != "第四章" {
		t.Fatalf("chapter view = %#v", view)
	}
	if got := store.chapters[view.ID]; got.Position != 4 {
		t.Fatalf("stored chapter = %#v", got)
	}
}

func TestSaveDraftDoesNotAdvanceRevision(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{})

	if err := service.SaveDraft(context.Background(), chapter.ID, "未提交正文", chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if store.savedDraft.body != "未提交正文" || store.savedDraft.baseRevision != 1 {
		t.Fatalf("saved draft = %#v", store.savedDraft)
	}
	if got := store.chapters[chapter.ID]; got.Revision != chapter.Revision || got.Body != chapter.Body {
		t.Fatalf("committed chapter changed: %#v", got)
	}
}

func TestCommitChapterReturnsConflictForStaleEditor(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter, err := chapter.Commit("较新的正文", chapter.Revision, time.Unix(110, 0))
	if err != nil {
		t.Fatal(err)
	}
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{})

	_, err = service.CommitChapter(context.Background(), chapter.ID, "旧窗口正文", 1)
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}
	if store.commitCalls != 0 {
		t.Fatalf("CommitChapter called %d times", store.commitCalls)
	}
}

func TestOpenChapterReturnsRecoverableDraft(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	store.draft = draftRecord{chapterID: chapter.ID, body: "恢复草稿", baseRevision: 1, found: true}
	service := openTestService(t, store, &sequenceIDs{})

	view, err := service.OpenChapter(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.HasRecoveryDraft || view.RecoveryBody != "恢复草稿" {
		t.Fatalf("chapter view = %#v", view)
	}
	if view.RecoveryUpdatedAt != time.Unix(150, 0).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("recovery updated at = %q", view.RecoveryUpdatedAt)
	}
}

func TestDiscardDraftRequiresEditCapabilityAndKeepsChapter(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	store.draft = draftRecord{chapterID: chapter.ID, body: "恢复草稿", baseRevision: 1, found: true}
	service := openTestService(t, store, &sequenceIDs{})

	if err := service.DiscardDraft(context.Background(), chapter.ID, chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if store.deleteDraftID != chapter.ID {
		t.Fatalf("deleted draft id = %q", store.deleteDraftID)
	}
	if store.deleteDraftRevision != chapter.Revision {
		t.Fatalf("deleted draft revision = %d", store.deleteDraftRevision)
	}
	if got := store.chapters[chapter.ID]; got != chapter {
		t.Fatalf("committed chapter changed: %#v", got)
	}
}

func openTestService(t *testing.T, store *fakeStore, ids *sequenceIDs) *Service {
	t.Helper()
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		&fakeStoreFactory{openStore: store},
		&fakeLicense{},
		fixedClock{now: time.Unix(200, 0)},
		ids,
	)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}
	return service
}

func mustProject(t *testing.T, id, title string) projectdomain.Project {
	t.Helper()
	project, err := projectdomain.NewProject(id, title, "奇幻", "简介", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func mustChapter(t *testing.T, id, projectID, title string, position int) projectdomain.Chapter {
	t.Helper()
	chapter, err := projectdomain.NewChapter(id, projectID, title, position, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	return chapter
}

func testPackage(root string) ports.ProjectPackage {
	return ports.ProjectPackage{
		Root:        root,
		Database:    root + "/project.db",
		Attachments: root + "/attachments",
		Exports:     root + "/exports",
		Snapshots:   root + "/snapshots",
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type sequenceIDs struct {
	ids []string
}

func (s *sequenceIDs) NewID() string {
	if len(s.ids) == 0 {
		return "unused-id"
	}
	id := s.ids[0]
	s.ids = s.ids[1:]
	return id
}

type fakeLicense struct {
	events         *[]string
	err            error
	lastCapability license.Capability
}

func (f *fakeLicense) Require(_ context.Context, capability license.Capability) error {
	appendEvent(f.events, "license")
	f.lastCapability = capability
	return f.err
}

type fakePackageManager struct {
	createPackage ports.ProjectPackage
	createErr     error
	openPackage   ports.ProjectPackage
	openErr       error
	events        *[]string
}

func (f *fakePackageManager) Create(string, string) (ports.ProjectPackage, error) {
	appendEvent(f.events, "create-package")
	return f.createPackage, f.createErr
}

func (f *fakePackageManager) Open(string) (ports.ProjectPackage, error) {
	appendEvent(f.events, "open-package")
	return f.openPackage, f.openErr
}

type fakeStoreFactory struct {
	createStore ports.ProjectStore
	createErr   error
	openStore   ports.ProjectStore
	openErr     error
	events      *[]string
}

func (f *fakeStoreFactory) Create(context.Context, string, projectdomain.Project) (ports.ProjectStore, error) {
	appendEvent(f.events, "create-store")
	return f.createStore, f.createErr
}

func (f *fakeStoreFactory) Open(context.Context, string) (ports.ProjectStore, error) {
	appendEvent(f.events, "open-store")
	return f.openStore, f.openErr
}

type draftRecord struct {
	chapterID    string
	body         string
	baseRevision int64
	found        bool
}

type fakeStore struct {
	project             projectdomain.Project
	chapters            map[string]projectdomain.Chapter
	draft               draftRecord
	savedDraft          draftRecord
	commitCalls         int
	deleteDraftID       string
	deleteDraftRevision int64
	closeEvent          string
	events              *[]string
	outlineNodes        map[string]storydomain.OutlineNode
	storyLines          map[string]storydomain.StoryLine
	storyLinks          []storydomain.StoryLineLink
	sceneCards          map[string]storydomain.SceneCard
	characters          map[string]worlddomain.Character
	factions            map[string]worlddomain.Faction
	memberships         map[string]worlddomain.Membership
	relationships       map[string]worlddomain.Relationship
	chapterAppearances  map[string]worlddomain.ChapterAppearance
	sceneAppearances    map[string]worlddomain.SceneAppearance
	worldRegions        map[string]worlddomain.WorldRegion
	regionAdjacencies   map[string]worlddomain.RegionAdjacency
	locations           map[string]worlddomain.Location
	locationLinks       map[string]worlddomain.LocationLink
	items               map[string]worlddomain.Item
	itemTransfers       []worlddomain.ItemTransfer
	worldRules          map[string]worlddomain.WorldRule
	foreshadowingHooks  map[string]worlddomain.ForeshadowingHook
	storyEvents         map[string]timelinedomain.StoryEvent
	storyEventLinks     map[string]timelinedomain.EventLink
	facts               map[string]canondomain.Fact
	factEvidence        map[string]canondomain.Evidence
	factScopes          map[string]canondomain.Scope
	factConflicts       map[string]canondomain.Conflict
	characterKnowledge  map[string]canondomain.CharacterKnowledge
	changeSets          map[string]revisiondomain.ChangeSet
}

func newFakeStore(project projectdomain.Project) *fakeStore {
	return &fakeStore{
		project:            project,
		chapters:           make(map[string]projectdomain.Chapter),
		outlineNodes:       make(map[string]storydomain.OutlineNode),
		storyLines:         make(map[string]storydomain.StoryLine),
		sceneCards:         make(map[string]storydomain.SceneCard),
		characters:         make(map[string]worlddomain.Character),
		factions:           make(map[string]worlddomain.Faction),
		memberships:        make(map[string]worlddomain.Membership),
		relationships:      make(map[string]worlddomain.Relationship),
		chapterAppearances: make(map[string]worlddomain.ChapterAppearance),
		sceneAppearances:   make(map[string]worlddomain.SceneAppearance),
		worldRegions:       make(map[string]worlddomain.WorldRegion),
		regionAdjacencies:  make(map[string]worlddomain.RegionAdjacency),
		locations:          make(map[string]worlddomain.Location),
		locationLinks:      make(map[string]worlddomain.LocationLink),
		items:              make(map[string]worlddomain.Item),
		worldRules:         make(map[string]worlddomain.WorldRule),
		foreshadowingHooks: make(map[string]worlddomain.ForeshadowingHook),
		storyEvents:        make(map[string]timelinedomain.StoryEvent),
		storyEventLinks:    make(map[string]timelinedomain.EventLink),
		facts:              make(map[string]canondomain.Fact),
		factEvidence:       make(map[string]canondomain.Evidence),
		factScopes:         make(map[string]canondomain.Scope),
		factConflicts:      make(map[string]canondomain.Conflict),
		characterKnowledge: make(map[string]canondomain.CharacterKnowledge),
		changeSets:         make(map[string]revisiondomain.ChangeSet),
	}
}

func (f *fakeStore) Project(context.Context) (projectdomain.Project, error) {
	return f.project, nil
}

func (f *fakeStore) CreateChapter(_ context.Context, chapter projectdomain.Chapter) error {
	f.chapters[chapter.ID] = chapter
	return nil
}

func (f *fakeStore) ListChapters(context.Context) ([]projectdomain.Chapter, error) {
	chapters := make([]projectdomain.Chapter, 0, len(f.chapters))
	for _, chapter := range f.chapters {
		chapters = append(chapters, chapter)
	}
	return chapters, nil
}

func (f *fakeStore) Chapter(_ context.Context, id string) (projectdomain.Chapter, error) {
	chapter, ok := f.chapters[id]
	if !ok {
		return projectdomain.Chapter{}, errors.New("chapter missing")
	}
	return chapter, nil
}

func (f *fakeStore) SaveDraft(_ context.Context, chapterID, body string, baseRevision int64) error {
	f.savedDraft = draftRecord{chapterID: chapterID, body: body, baseRevision: baseRevision, found: true}
	return nil
}

func (f *fakeStore) Draft(_ context.Context, chapterID string) (string, int64, time.Time, bool, error) {
	if !f.draft.found || f.draft.chapterID != chapterID {
		return "", 0, time.Time{}, false, nil
	}
	return f.draft.body, f.draft.baseRevision, time.Unix(150, 0).UTC(), true, nil
}

func (f *fakeStore) DeleteDraft(_ context.Context, chapterID string, baseRevision int64) error {
	f.deleteDraftID = chapterID
	f.deleteDraftRevision = baseRevision
	f.draft = draftRecord{}
	return nil
}

func (f *fakeStore) CommitChapter(_ context.Context, chapter, _ projectdomain.Chapter) error {
	f.commitCalls++
	f.chapters[chapter.ID] = chapter
	return nil
}

func (f *fakeStore) CreateChangeSet(_ context.Context, changeSet revisiondomain.ChangeSet) error {
	f.changeSets[changeSet.ID] = changeSet
	return nil
}

func (f *fakeStore) ListChangeSets(context.Context) ([]revisiondomain.ChangeSet, error) {
	result := make([]revisiondomain.ChangeSet, 0, len(f.changeSets))
	for _, changeSet := range f.changeSets {
		result = append(result, changeSet)
	}
	return result, nil
}

func (f *fakeStore) ChangeSet(_ context.Context, id string) (revisiondomain.ChangeSet, error) {
	changeSet, ok := f.changeSets[id]
	if !ok {
		return revisiondomain.ChangeSet{}, errors.New("change set missing")
	}
	return changeSet, nil
}

func (f *fakeStore) ApplyChangeSet(_ context.Context, id string, appliedAt time.Time) (projectdomain.Chapter, error) {
	changeSet, ok := f.changeSets[id]
	if !ok {
		return projectdomain.Chapter{}, errors.New("change set missing")
	}
	if changeSet.Status == revisiondomain.ChangeSetApplied {
		return f.chapters[changeSet.Operations[0].TargetID], nil
	}
	applied, err := changeSet.Apply(appliedAt)
	if err != nil {
		return projectdomain.Chapter{}, err
	}
	operation := applied.Operations[0]
	current := f.chapters[operation.TargetID]
	updated, err := current.Commit(operation.AfterValue, operation.BaseRevision, appliedAt)
	if err != nil {
		return projectdomain.Chapter{}, err
	}
	f.chapters[updated.ID] = updated
	f.changeSets[id] = applied
	return updated, nil
}

func (f *fakeStore) RejectChangeSet(_ context.Context, id string, rejectedAt time.Time) error {
	changeSet, ok := f.changeSets[id]
	if !ok {
		return errors.New("change set missing")
	}
	rejected, err := changeSet.Reject(rejectedAt)
	if err != nil {
		return err
	}
	f.changeSets[id] = rejected
	return nil
}

func (f *fakeStore) CreateOutlineNode(_ context.Context, node storydomain.OutlineNode) error {
	f.outlineNodes[node.ID] = node
	return nil
}
func (f *fakeStore) ListOutlineNodes(context.Context) ([]storydomain.OutlineNode, error) {
	nodes := make([]storydomain.OutlineNode, 0, len(f.outlineNodes))
	for _, node := range f.outlineNodes {
		if node.DeletedAt == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}
func (f *fakeStore) ArchiveOutlineNode(_ context.Context, id string, archivedAt time.Time) error {
	node, ok := f.outlineNodes[id]
	if !ok {
		return errors.New("outline missing")
	}
	node.DeletedAt = &archivedAt
	f.outlineNodes[id] = node
	return nil
}
func (f *fakeStore) SwapOutlineNodePositions(_ context.Context, firstID, secondID string, _ time.Time) error {
	first, firstOK := f.outlineNodes[firstID]
	second, secondOK := f.outlineNodes[secondID]
	if !firstOK || !secondOK {
		return errors.New("outline missing")
	}
	first.Position, second.Position = second.Position, first.Position
	f.outlineNodes[firstID], f.outlineNodes[secondID] = first, second
	return nil
}
func (f *fakeStore) CreateStoryLine(_ context.Context, line storydomain.StoryLine) error {
	f.storyLines[line.ID] = line
	return nil
}
func (f *fakeStore) ListStoryLines(context.Context) ([]storydomain.StoryLine, error) {
	lines := make([]storydomain.StoryLine, 0, len(f.storyLines))
	for _, line := range f.storyLines {
		lines = append(lines, line)
	}
	return lines, nil
}
func (f *fakeStore) LinkStoryLine(_ context.Context, _ string, link storydomain.StoryLineLink) error {
	f.storyLinks = append(f.storyLinks, link)
	return nil
}
func (f *fakeStore) ListStoryLineLinks(context.Context) ([]storydomain.StoryLineLink, error) {
	return append([]storydomain.StoryLineLink(nil), f.storyLinks...), nil
}
func (f *fakeStore) CreateSceneCard(_ context.Context, card storydomain.SceneCard) error {
	f.sceneCards[card.ID] = card
	return nil
}
func (f *fakeStore) ListSceneCards(_ context.Context, chapterID string) ([]storydomain.SceneCard, error) {
	cards := make([]storydomain.SceneCard, 0)
	for _, card := range f.sceneCards {
		if card.ChapterID == chapterID {
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func (f *fakeStore) CreateCharacter(_ context.Context, character worlddomain.Character) error {
	f.characters[character.ID] = character
	return nil
}
func (f *fakeStore) UpdateCharacter(_ context.Context, character worlddomain.Character) error {
	f.characters[character.ID] = character
	return nil
}
func (f *fakeStore) ListCharacters(context.Context) ([]worlddomain.Character, error) {
	characters := make([]worlddomain.Character, 0, len(f.characters))
	for _, character := range f.characters {
		if character.DeletedAt == nil {
			characters = append(characters, character)
		}
	}
	return characters, nil
}
func (f *fakeStore) Character(_ context.Context, id string) (worlddomain.Character, error) {
	character, ok := f.characters[id]
	if !ok || character.DeletedAt != nil {
		return worlddomain.Character{}, errors.New("character missing")
	}
	return character, nil
}
func (f *fakeStore) ArchiveCharacter(_ context.Context, id string, archivedAt time.Time) error {
	character, ok := f.characters[id]
	if !ok {
		return errors.New("character missing")
	}
	character.DeletedAt = &archivedAt
	f.characters[id] = character
	return nil
}
func (f *fakeStore) CreateFaction(_ context.Context, faction worlddomain.Faction) error {
	f.factions[faction.ID] = faction
	return nil
}
func (f *fakeStore) UpdateFaction(_ context.Context, faction worlddomain.Faction) error {
	f.factions[faction.ID] = faction
	return nil
}
func (f *fakeStore) ListFactions(context.Context) ([]worlddomain.Faction, error) {
	factions := make([]worlddomain.Faction, 0, len(f.factions))
	for _, faction := range f.factions {
		if faction.DeletedAt == nil {
			factions = append(factions, faction)
		}
	}
	return factions, nil
}
func (f *fakeStore) Faction(_ context.Context, id string) (worlddomain.Faction, error) {
	faction, ok := f.factions[id]
	if !ok || faction.DeletedAt != nil {
		return worlddomain.Faction{}, errors.New("faction missing")
	}
	return faction, nil
}
func (f *fakeStore) ArchiveFaction(_ context.Context, id string, archivedAt time.Time) error {
	faction, ok := f.factions[id]
	if !ok {
		return errors.New("faction missing")
	}
	faction.DeletedAt = &archivedAt
	f.factions[id] = faction
	return nil
}
func (f *fakeStore) UpsertMembership(_ context.Context, membership worlddomain.Membership) error {
	f.memberships[membership.CharacterID+"\x00"+membership.FactionID] = membership
	return nil
}
func (f *fakeStore) ListMemberships(context.Context) ([]worlddomain.Membership, error) {
	memberships := make([]worlddomain.Membership, 0, len(f.memberships))
	for _, membership := range f.memberships {
		memberships = append(memberships, membership)
	}
	return memberships, nil
}
func (f *fakeStore) DeleteMembership(_ context.Context, characterID, factionID string) error {
	delete(f.memberships, characterID+"\x00"+factionID)
	return nil
}
func (f *fakeStore) CreateRelationship(_ context.Context, relationship worlddomain.Relationship) error {
	f.relationships[relationship.ID] = relationship
	return nil
}
func (f *fakeStore) ListRelationships(context.Context) ([]worlddomain.Relationship, error) {
	relationships := make([]worlddomain.Relationship, 0, len(f.relationships))
	for _, relationship := range f.relationships {
		if relationship.DeletedAt == nil {
			relationships = append(relationships, relationship)
		}
	}
	return relationships, nil
}
func (f *fakeStore) ArchiveRelationship(_ context.Context, id string, archivedAt time.Time) error {
	relationship, ok := f.relationships[id]
	if !ok {
		return errors.New("relationship missing")
	}
	relationship.DeletedAt = &archivedAt
	f.relationships[id] = relationship
	return nil
}
func (f *fakeStore) AddChapterAppearance(_ context.Context, appearance worlddomain.ChapterAppearance) error {
	f.chapterAppearances[appearance.ChapterID+"\x00"+appearance.CharacterID] = appearance
	return nil
}
func (f *fakeStore) ListChapterAppearances(context.Context) ([]worlddomain.ChapterAppearance, error) {
	appearances := make([]worlddomain.ChapterAppearance, 0, len(f.chapterAppearances))
	for _, appearance := range f.chapterAppearances {
		appearances = append(appearances, appearance)
	}
	return appearances, nil
}
func (f *fakeStore) RemoveChapterAppearance(_ context.Context, characterID, chapterID string) error {
	delete(f.chapterAppearances, chapterID+"\x00"+characterID)
	return nil
}
func (f *fakeStore) AddSceneAppearance(_ context.Context, appearance worlddomain.SceneAppearance) error {
	f.sceneAppearances[appearance.SceneCardID+"\x00"+appearance.CharacterID] = appearance
	return nil
}
func (f *fakeStore) ListSceneAppearances(context.Context) ([]worlddomain.SceneAppearance, error) {
	appearances := make([]worlddomain.SceneAppearance, 0, len(f.sceneAppearances))
	for _, appearance := range f.sceneAppearances {
		appearances = append(appearances, appearance)
	}
	return appearances, nil
}
func (f *fakeStore) RemoveSceneAppearance(_ context.Context, characterID, sceneCardID string) error {
	delete(f.sceneAppearances, sceneCardID+"\x00"+characterID)
	return nil
}

func (f *fakeStore) CreateWorldRegion(_ context.Context, region worlddomain.WorldRegion) error {
	f.worldRegions[region.ID] = region
	return nil
}
func (f *fakeStore) UpdateWorldRegion(_ context.Context, region worlddomain.WorldRegion) error {
	f.worldRegions[region.ID] = region
	return nil
}
func (f *fakeStore) ListWorldRegions(context.Context) ([]worlddomain.WorldRegion, error) {
	regions := make([]worlddomain.WorldRegion, 0, len(f.worldRegions))
	for _, region := range f.worldRegions {
		if region.DeletedAt == nil {
			regions = append(regions, region)
		}
	}
	return regions, nil
}
func (f *fakeStore) WorldRegion(_ context.Context, id string) (worlddomain.WorldRegion, error) {
	region, ok := f.worldRegions[id]
	if !ok || region.DeletedAt != nil {
		return worlddomain.WorldRegion{}, errors.New("world region missing")
	}
	return region, nil
}
func (f *fakeStore) ArchiveWorldRegion(_ context.Context, id string, archivedAt time.Time) error {
	region, ok := f.worldRegions[id]
	if !ok {
		return errors.New("world region missing")
	}
	region.DeletedAt = &archivedAt
	f.worldRegions[id] = region
	return nil
}
func (f *fakeStore) UpsertRegionAdjacency(_ context.Context, adjacency worlddomain.RegionAdjacency) error {
	f.regionAdjacencies[adjacency.FirstRegionID+"\x00"+adjacency.SecondRegionID] = adjacency
	return nil
}
func (f *fakeStore) ListRegionAdjacencies(context.Context) ([]worlddomain.RegionAdjacency, error) {
	adjacencies := make([]worlddomain.RegionAdjacency, 0, len(f.regionAdjacencies))
	for _, adjacency := range f.regionAdjacencies {
		adjacencies = append(adjacencies, adjacency)
	}
	return adjacencies, nil
}
func (f *fakeStore) DeleteRegionAdjacency(_ context.Context, firstRegionID, secondRegionID string) error {
	if secondRegionID < firstRegionID {
		firstRegionID, secondRegionID = secondRegionID, firstRegionID
	}
	delete(f.regionAdjacencies, firstRegionID+"\x00"+secondRegionID)
	return nil
}
func (f *fakeStore) CreateLocation(_ context.Context, location worlddomain.Location) error {
	f.locations[location.ID] = location
	return nil
}
func (f *fakeStore) UpdateLocation(_ context.Context, location worlddomain.Location) error {
	f.locations[location.ID] = location
	return nil
}
func (f *fakeStore) ListLocations(context.Context) ([]worlddomain.Location, error) {
	locations := make([]worlddomain.Location, 0, len(f.locations))
	for _, location := range f.locations {
		if location.DeletedAt == nil {
			locations = append(locations, location)
		}
	}
	return locations, nil
}
func (f *fakeStore) Location(_ context.Context, id string) (worlddomain.Location, error) {
	location, ok := f.locations[id]
	if !ok || location.DeletedAt != nil {
		return worlddomain.Location{}, errors.New("location missing")
	}
	return location, nil
}
func (f *fakeStore) ArchiveLocation(_ context.Context, id string, archivedAt time.Time) error {
	location, ok := f.locations[id]
	if !ok {
		return errors.New("location missing")
	}
	location.DeletedAt = &archivedAt
	f.locations[id] = location
	return nil
}
func (f *fakeStore) UpsertLocationLink(_ context.Context, link worlddomain.LocationLink) error {
	f.locationLinks[link.LocationID+"\x00"+string(link.TargetType)+"\x00"+link.TargetID] = link
	return nil
}
func (f *fakeStore) ListLocationLinks(context.Context) ([]worlddomain.LocationLink, error) {
	links := make([]worlddomain.LocationLink, 0, len(f.locationLinks))
	for _, link := range f.locationLinks {
		links = append(links, link)
	}
	return links, nil
}
func (f *fakeStore) DeleteLocationLink(_ context.Context, locationID string, targetType worlddomain.LocationTargetType, targetID string) error {
	delete(f.locationLinks, locationID+"\x00"+string(targetType)+"\x00"+targetID)
	return nil
}
func (f *fakeStore) CreateItem(_ context.Context, item worlddomain.Item) error {
	f.items[item.ID] = item
	return nil
}
func (f *fakeStore) UpdateItem(_ context.Context, item worlddomain.Item) error {
	f.items[item.ID] = item
	return nil
}
func (f *fakeStore) ListItems(context.Context) ([]worlddomain.Item, error) {
	items := make([]worlddomain.Item, 0, len(f.items))
	for _, item := range f.items {
		if item.DeletedAt == nil {
			items = append(items, item)
		}
	}
	return items, nil
}
func (f *fakeStore) Item(_ context.Context, id string) (worlddomain.Item, error) {
	item, ok := f.items[id]
	if !ok || item.DeletedAt != nil {
		return worlddomain.Item{}, errors.New("item missing")
	}
	return item, nil
}
func (f *fakeStore) TransferItem(_ context.Context, item worlddomain.Item, transfer worlddomain.ItemTransfer) error {
	f.items[item.ID] = item
	f.itemTransfers = append(f.itemTransfers, transfer)
	return nil
}
func (f *fakeStore) ListItemTransfers(_ context.Context, itemID string) ([]worlddomain.ItemTransfer, error) {
	transfers := make([]worlddomain.ItemTransfer, 0)
	for _, transfer := range f.itemTransfers {
		if transfer.ItemID == itemID {
			transfers = append(transfers, transfer)
		}
	}
	return transfers, nil
}
func (f *fakeStore) ArchiveItem(_ context.Context, id string, archivedAt time.Time) error {
	item, ok := f.items[id]
	if !ok {
		return errors.New("item missing")
	}
	item.DeletedAt = &archivedAt
	f.items[id] = item
	return nil
}
func (f *fakeStore) CreateWorldRule(_ context.Context, rule worlddomain.WorldRule) error {
	f.worldRules[rule.ID] = rule
	return nil
}
func (f *fakeStore) UpdateWorldRule(_ context.Context, rule worlddomain.WorldRule) error {
	f.worldRules[rule.ID] = rule
	return nil
}
func (f *fakeStore) ListWorldRules(context.Context) ([]worlddomain.WorldRule, error) {
	rules := make([]worlddomain.WorldRule, 0, len(f.worldRules))
	for _, rule := range f.worldRules {
		if rule.DeletedAt == nil {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}
func (f *fakeStore) WorldRule(_ context.Context, id string) (worlddomain.WorldRule, error) {
	rule, ok := f.worldRules[id]
	if !ok || rule.DeletedAt != nil {
		return worlddomain.WorldRule{}, errors.New("world rule missing")
	}
	return rule, nil
}
func (f *fakeStore) ArchiveWorldRule(_ context.Context, id string, archivedAt time.Time) error {
	rule, ok := f.worldRules[id]
	if !ok {
		return errors.New("world rule missing")
	}
	rule.DeletedAt = &archivedAt
	f.worldRules[id] = rule
	return nil
}
func (f *fakeStore) CreateForeshadowingHook(_ context.Context, hook worlddomain.ForeshadowingHook) error {
	f.foreshadowingHooks[hook.ID] = hook
	return nil
}
func (f *fakeStore) UpdateForeshadowingHook(_ context.Context, hook worlddomain.ForeshadowingHook) error {
	f.foreshadowingHooks[hook.ID] = hook
	return nil
}
func (f *fakeStore) ListForeshadowingHooks(context.Context) ([]worlddomain.ForeshadowingHook, error) {
	hooks := make([]worlddomain.ForeshadowingHook, 0, len(f.foreshadowingHooks))
	for _, hook := range f.foreshadowingHooks {
		if hook.DeletedAt == nil {
			hooks = append(hooks, hook)
		}
	}
	return hooks, nil
}
func (f *fakeStore) ForeshadowingHook(_ context.Context, id string) (worlddomain.ForeshadowingHook, error) {
	hook, ok := f.foreshadowingHooks[id]
	if !ok || hook.DeletedAt != nil {
		return worlddomain.ForeshadowingHook{}, errors.New("foreshadowing hook missing")
	}
	return hook, nil
}
func (f *fakeStore) ArchiveForeshadowingHook(_ context.Context, id string, archivedAt time.Time) error {
	hook, ok := f.foreshadowingHooks[id]
	if !ok {
		return errors.New("foreshadowing hook missing")
	}
	hook.DeletedAt = &archivedAt
	f.foreshadowingHooks[id] = hook
	return nil
}

func (f *fakeStore) CreateStoryEvent(_ context.Context, event timelinedomain.StoryEvent) error {
	f.storyEvents[event.ID] = event
	return nil
}
func (f *fakeStore) UpdateStoryEvent(_ context.Context, event timelinedomain.StoryEvent) error {
	f.storyEvents[event.ID] = event
	return nil
}
func (f *fakeStore) ListStoryEvents(context.Context) ([]timelinedomain.StoryEvent, error) {
	result := make([]timelinedomain.StoryEvent, 0)
	for _, value := range f.storyEvents {
		if value.DeletedAt == nil {
			result = append(result, value)
		}
	}
	return result, nil
}
func (f *fakeStore) StoryEvent(_ context.Context, id string) (timelinedomain.StoryEvent, error) {
	value, ok := f.storyEvents[id]
	if !ok || value.DeletedAt != nil {
		return timelinedomain.StoryEvent{}, errors.New("story event missing")
	}
	return value, nil
}
func (f *fakeStore) ArchiveStoryEvent(_ context.Context, id string, at time.Time) error {
	value, ok := f.storyEvents[id]
	if !ok {
		return errors.New("story event missing")
	}
	value.DeletedAt = &at
	f.storyEvents[id] = value
	return nil
}
func (f *fakeStore) UpsertStoryEventLink(_ context.Context, link timelinedomain.EventLink) error {
	f.storyEventLinks[link.EventID+"\x00"+string(link.TargetType)+"\x00"+link.TargetID] = link
	return nil
}
func (f *fakeStore) ListStoryEventLinks(context.Context) ([]timelinedomain.EventLink, error) {
	result := make([]timelinedomain.EventLink, 0, len(f.storyEventLinks))
	for _, value := range f.storyEventLinks {
		result = append(result, value)
	}
	return result, nil
}
func (f *fakeStore) DeleteStoryEventLink(_ context.Context, eventID string, targetType timelinedomain.EventTargetType, targetID string) error {
	delete(f.storyEventLinks, eventID+"\x00"+string(targetType)+"\x00"+targetID)
	return nil
}
func (f *fakeStore) CreateFact(_ context.Context, fact canondomain.Fact) error {
	f.facts[fact.ID] = fact
	return nil
}
func (f *fakeStore) UpdateFact(_ context.Context, fact canondomain.Fact) error {
	f.facts[fact.ID] = fact
	return nil
}
func (f *fakeStore) ListFacts(context.Context) ([]canondomain.Fact, error) {
	result := make([]canondomain.Fact, 0)
	for _, value := range f.facts {
		if value.DeletedAt == nil {
			result = append(result, value)
		}
	}
	return result, nil
}
func (f *fakeStore) Fact(_ context.Context, id string) (canondomain.Fact, error) {
	value, ok := f.facts[id]
	if !ok || value.DeletedAt != nil {
		return canondomain.Fact{}, errors.New("fact missing")
	}
	return value, nil
}
func (f *fakeStore) ArchiveFact(_ context.Context, id string, at time.Time) error {
	value, ok := f.facts[id]
	if !ok {
		return errors.New("fact missing")
	}
	value.DeletedAt = &at
	f.facts[id] = value
	return nil
}
func (f *fakeStore) CreateCharacterKnowledge(_ context.Context, value canondomain.CharacterKnowledge) error {
	f.characterKnowledge[value.ID] = value
	return nil
}
func (f *fakeStore) ListCharacterKnowledge(context.Context) ([]canondomain.CharacterKnowledge, error) {
	result := make([]canondomain.CharacterKnowledge, 0, len(f.characterKnowledge))
	for _, value := range f.characterKnowledge {
		if value.DeletedAt == nil {
			result = append(result, value)
		}
	}
	return result, nil
}
func (f *fakeStore) ArchiveCharacterKnowledge(_ context.Context, id string, at time.Time) error {
	value, ok := f.characterKnowledge[id]
	if !ok {
		return errors.New("character knowledge missing")
	}
	value.DeletedAt = &at
	f.characterKnowledge[id] = value
	return nil
}
func (f *fakeStore) CreateFactEvidence(_ context.Context, value canondomain.Evidence) error {
	f.factEvidence[value.ID] = value
	return nil
}
func (f *fakeStore) ListFactEvidence(context.Context) ([]canondomain.Evidence, error) {
	result := make([]canondomain.Evidence, 0, len(f.factEvidence))
	for _, value := range f.factEvidence {
		result = append(result, value)
	}
	return result, nil
}
func (f *fakeStore) DeleteFactEvidence(_ context.Context, id string) error {
	delete(f.factEvidence, id)
	return nil
}
func (f *fakeStore) CreateFactScope(_ context.Context, value canondomain.Scope) error {
	f.factScopes[value.ID] = value
	return nil
}
func (f *fakeStore) ListFactScopes(context.Context) ([]canondomain.Scope, error) {
	result := make([]canondomain.Scope, 0, len(f.factScopes))
	for _, value := range f.factScopes {
		result = append(result, value)
	}
	return result, nil
}
func (f *fakeStore) DeleteFactScope(_ context.Context, id string) error {
	delete(f.factScopes, id)
	return nil
}
func (f *fakeStore) CreateFactConflict(_ context.Context, value canondomain.Conflict) error {
	f.factConflicts[value.ID] = value
	return nil
}
func (f *fakeStore) UpdateFactConflict(_ context.Context, value canondomain.Conflict) error {
	f.factConflicts[value.ID] = value
	return nil
}
func (f *fakeStore) ListFactConflicts(context.Context) ([]canondomain.Conflict, error) {
	result := make([]canondomain.Conflict, 0, len(f.factConflicts))
	for _, value := range f.factConflicts {
		result = append(result, value)
	}
	return result, nil
}
func (f *fakeStore) DeleteFactConflict(_ context.Context, id string) error {
	delete(f.factConflicts, id)
	return nil
}

func (f *fakeStore) Close() error {
	appendEvent(f.events, f.closeEvent)
	return nil
}

func appendEvent(events *[]string, event string) {
	if events == nil || event == "" {
		return
	}
	*events = append(*events, event)
}
