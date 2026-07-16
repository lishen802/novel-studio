package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/workspace"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
)

func TestDesktopAPICreateProjectForwardsInput(t *testing.T) {
	want := workspace.ProjectView{ID: "project-1", Title: "长夜织境", Path: "/books/长夜织境.zjproject"}
	workspaces := &fakeWorkspace{createResult: want}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())
	input := CreateProjectInput{
		ParentDirectory: "/books",
		Title:           " 长夜织境 ",
		Genre:           "东方奇幻",
		Summary:         "简介",
	}

	got, err := api.CreateProject(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
	if workspaces.createInput != input {
		t.Fatalf("forwarded input = %#v, want %#v", workspaces.createInput, input)
	}
}

func TestDesktopAPIForwardsChapterOperations(t *testing.T) {
	chapter := workspace.ChapterView{ID: "chapter-1", Title: "第一章", Revision: 1}
	committed := workspace.ChapterView{ID: "chapter-1", Title: "第一章", Body: "正文", Revision: 2}
	workspaces := &fakeWorkspace{
		listResult:          []workspace.ChapterView{chapter},
		createChapterResult: chapter,
		openChapterResult:   chapter,
		commitResult:        committed,
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	listed, err := api.ListChapters()
	if err != nil || !reflect.DeepEqual(listed, []workspace.ChapterView{chapter}) {
		t.Fatalf("ListChapters = (%#v, %v)", listed, err)
	}
	created, err := api.CreateChapter("第一章")
	if err != nil || created != chapter || workspaces.createdChapterTitle != "第一章" {
		t.Fatalf("CreateChapter = (%#v, %v), title = %q", created, err, workspaces.createdChapterTitle)
	}
	opened, err := api.OpenChapter(chapter.ID)
	if err != nil || opened != chapter || workspaces.openedChapterID != chapter.ID {
		t.Fatalf("OpenChapter = (%#v, %v), id = %q", opened, err, workspaces.openedChapterID)
	}
	if err := api.SaveDraft(chapter.ID, "草稿", 1); err != nil {
		t.Fatal(err)
	}
	if workspaces.savedDraft != (draftCall{id: chapter.ID, body: "草稿", revision: 1}) {
		t.Fatalf("saved draft = %#v", workspaces.savedDraft)
	}
	if err := api.DiscardDraft(chapter.ID, chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if workspaces.discardedDraftID != chapter.ID {
		t.Fatalf("discarded draft id = %q", workspaces.discardedDraftID)
	}
	if workspaces.discardedDraftRevision != chapter.Revision {
		t.Fatalf("discarded draft revision = %d", workspaces.discardedDraftRevision)
	}
	gotCommit, err := api.CommitChapter(chapter.ID, "正文", 1)
	if err != nil || gotCommit != committed {
		t.Fatalf("CommitChapter = (%#v, %v)", gotCommit, err)
	}
	if workspaces.committed != (draftCall{id: chapter.ID, body: "正文", revision: 1}) {
		t.Fatalf("commit call = %#v", workspaces.committed)
	}
}

func TestDesktopAPIForwardsStoryStructureOperations(t *testing.T) {
	structure := workspace.StoryStructureView{OutlineNodes: []workspace.OutlineNodeView{{ID: "book", Title: "全书大纲"}}}
	workspaces := &fakeWorkspace{storyStructureResult: structure}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.StoryStructure()
	if err != nil || !reflect.DeepEqual(got, structure) {
		t.Fatalf("StoryStructure = (%#v, %v)", got, err)
	}
	outlineInput := workspace.CreateOutlineNodeInput{Level: "book", Title: "全书大纲"}
	if _, err := api.CreateOutlineNode(outlineInput); err != nil || workspaces.outlineInput != outlineInput {
		t.Fatalf("CreateOutlineNode input = %#v, error = %v", workspaces.outlineInput, err)
	}
	lineInput := workspace.CreateStoryLineInput{Name: "主线"}
	if _, err := api.CreateStoryLine(lineInput); err != nil || workspaces.storyLineInput != lineInput {
		t.Fatalf("CreateStoryLine input = %#v, error = %v", workspaces.storyLineInput, err)
	}
	linkInput := workspace.LinkStoryLineInput{StoryLineID: "line", OutlineNodeID: "book"}
	if _, err := api.LinkStoryLine(linkInput); err != nil || workspaces.storyLinkInput != linkInput {
		t.Fatalf("LinkStoryLine input = %#v, error = %v", workspaces.storyLinkInput, err)
	}
	cardInput := workspace.CreateSceneCardInput{ChapterID: "chapter", OutlineNodeID: "scene", Title: "场景"}
	if _, err := api.CreateSceneCard(cardInput); err != nil || workspaces.sceneCardInput != cardInput {
		t.Fatalf("CreateSceneCard input = %#v, error = %v", workspaces.sceneCardInput, err)
	}
	if err := api.ArchiveOutlineNode("book"); err != nil || workspaces.archivedOutlineID != "book" {
		t.Fatalf("ArchiveOutlineNode id = %q, error = %v", workspaces.archivedOutlineID, err)
	}
	if err := api.MoveOutlineNode("book", "down"); err != nil || workspaces.movedOutlineID != "book" || workspaces.moveDirection != "down" {
		t.Fatalf("MoveOutlineNode = (%q, %q, %v)", workspaces.movedOutlineID, workspaces.moveDirection, err)
	}
}

func TestDesktopAPIForwardsStoryWorldOperations(t *testing.T) {
	world := workspace.StoryWorldView{Characters: []workspace.CharacterView{{ID: "character", Name: "林默"}}}
	workspaces := &fakeWorkspace{storyWorldResult: world}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.StoryWorld()
	if err != nil || !reflect.DeepEqual(got, world) {
		t.Fatalf("StoryWorld = (%#v, %v)", got, err)
	}
	characterInput := workspace.CharacterInput{Name: "林默", Aliases: []string{"阿默"}, Role: "protagonist"}
	if _, err := api.CreateCharacter(characterInput); err != nil || !reflect.DeepEqual(workspaces.characterInput, characterInput) {
		t.Fatalf("CreateCharacter input = %#v, error = %v", workspaces.characterInput, err)
	}
	if _, err := api.UpdateCharacter("character", characterInput); err != nil || workspaces.updatedCharacterID != "character" {
		t.Fatalf("UpdateCharacter id = %q, error = %v", workspaces.updatedCharacterID, err)
	}
	if err := api.ArchiveCharacter("character"); err != nil || workspaces.archivedCharacterID != "character" {
		t.Fatalf("ArchiveCharacter id = %q, error = %v", workspaces.archivedCharacterID, err)
	}
	factionInput := workspace.FactionInput{Name: "守夜人", Goal: "守护港城"}
	if _, err := api.CreateFaction(factionInput); err != nil || workspaces.factionInput != factionInput {
		t.Fatalf("CreateFaction input = %#v, error = %v", workspaces.factionInput, err)
	}
	if _, err := api.UpdateFaction("faction", factionInput); err != nil || workspaces.updatedFactionID != "faction" {
		t.Fatalf("UpdateFaction id = %q, error = %v", workspaces.updatedFactionID, err)
	}
	if err := api.ArchiveFaction("faction"); err != nil || workspaces.archivedFactionID != "faction" {
		t.Fatalf("ArchiveFaction id = %q, error = %v", workspaces.archivedFactionID, err)
	}
	membershipInput := workspace.MembershipInput{CharacterID: "character", FactionID: "faction", Role: "调查员", Status: "active"}
	if _, err := api.UpsertMembership(membershipInput); err != nil || workspaces.membershipInput != membershipInput {
		t.Fatalf("UpsertMembership input = %#v, error = %v", workspaces.membershipInput, err)
	}
	if err := api.DeleteMembership("character", "faction"); err != nil || workspaces.deletedMembership != "character:faction" {
		t.Fatalf("DeleteMembership = %q, error = %v", workspaces.deletedMembership, err)
	}
	relationshipInput := workspace.RelationshipInput{SourceCharacterID: "character", TargetCharacterID: "other", Kind: "ally"}
	if _, err := api.CreateRelationship(relationshipInput); err != nil || workspaces.relationshipInput != relationshipInput {
		t.Fatalf("CreateRelationship input = %#v, error = %v", workspaces.relationshipInput, err)
	}
	if err := api.ArchiveRelationship("relationship"); err != nil || workspaces.archivedRelationshipID != "relationship" {
		t.Fatalf("ArchiveRelationship id = %q, error = %v", workspaces.archivedRelationshipID, err)
	}
	appearanceInput := workspace.AppearanceInput{CharacterID: "character", TargetID: "chapter", Role: "pov"}
	if err := api.LinkCharacterToChapter(appearanceInput); err != nil || workspaces.chapterAppearanceInput != appearanceInput {
		t.Fatalf("LinkCharacterToChapter input = %#v, error = %v", workspaces.chapterAppearanceInput, err)
	}
	if err := api.UnlinkCharacterFromChapter("character", "chapter"); err != nil || workspaces.unlinkedChapter != "character:chapter" {
		t.Fatalf("UnlinkCharacterFromChapter = %q, error = %v", workspaces.unlinkedChapter, err)
	}
	appearanceInput.TargetID = "scene"
	if err := api.LinkCharacterToScene(appearanceInput); err != nil || workspaces.sceneAppearanceInput != appearanceInput {
		t.Fatalf("LinkCharacterToScene input = %#v, error = %v", workspaces.sceneAppearanceInput, err)
	}
	if err := api.UnlinkCharacterFromScene("character", "scene"); err != nil || workspaces.unlinkedScene != "character:scene" {
		t.Fatalf("UnlinkCharacterFromScene = %q, error = %v", workspaces.unlinkedScene, err)
	}
}

func TestDesktopAPIDirectoryCancellationReturnsEmpty(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{}, &fakeDirectoryPicker{})
	api.startup(context.Background())

	parent, err := api.ChooseProjectParent()
	if err != nil || parent != "" {
		t.Fatalf("ChooseProjectParent = (%q, %v)", parent, err)
	}
	projectPath, err := api.ChooseProjectToOpen()
	if err != nil || projectPath != "" {
		t.Fatalf("ChooseProjectToOpen = (%q, %v)", projectPath, err)
	}
}

func TestDesktopAPIMapsWorkspaceErrorToSafeDesktopError(t *testing.T) {
	internal := errors.New("SELECT body FROM chapters failed: secret text")
	api := newDesktopAPI(
		&fakeWorkspace{openErr: internal},
		&fakeDirectoryPicker{},
	)
	api.startup(context.Background())

	_, err := api.OpenProject("/books/broken.zjproject")
	var safe *desktopError
	if !errors.As(err, &safe) || safe.Code != errorOperationFailed {
		t.Fatalf("error = %#v, want safe operation error", err)
	}
	if got := err.Error(); got == "" || containsAny(got, "SELECT", "secret text") {
		t.Fatalf("unsafe error was exposed: %q", got)
	}
}

func TestSafeDesktopErrorClassifiesRecoveryFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code desktopErrorCode
	}{
		{name: "missing", err: os.ErrNotExist, code: errorProjectNotFound},
		{name: "permission", err: os.ErrPermission, code: errorPermissionDenied},
		{name: "corrupt", err: sqlite.ErrDatabaseCorrupt, code: errorProjectCorrupt},
		{name: "too new", err: sqlite.ErrDatabaseTooNew, code: errorProjectTooNew},
		{name: "revision", err: projectdomain.ErrRevisionConflict, code: errorRevisionConflict},
		{name: "write", err: sqlite.ErrDatabaseWrite, code: errorWriteFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var safe *desktopError
			if err := safeDesktopError(test.err); !errors.As(err, &safe) || safe.Code != test.code {
				t.Fatalf("error = %#v, want code %q", err, test.code)
			}
		})
	}
}

func TestSafeDesktopErrorExplainsReferencedWorldEntity(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(worlddomain.ErrReferenced)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("REFERENCE_CONFLICT") || safe.Message != "该内容仍被其他故事资料引用，请先解除关联后再归档。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func TestSafeDesktopErrorExplainsReferencedStoryEntity(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(storydomain.ErrReferenced)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("REFERENCE_CONFLICT") || safe.Message != "该内容仍被其他故事资料引用，请先解除关联后再归档。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func TestSafeDesktopErrorExplainsBlankWorldValue(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(worlddomain.ErrBlankValue)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("VALIDATION_FAILED") || safe.Message != "请填写完整的角色、势力或归属信息。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func TestAppShutdownClosesWorkspace(t *testing.T) {
	workspaces := &fakeWorkspace{}
	app := &App{api: newDesktopAPI(workspaces, &fakeDirectoryPicker{})}

	app.shutdown(context.Background())

	if workspaces.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", workspaces.closeCalls)
	}
}

func TestAppWaitsForFrontendFlushBeforeClosing(t *testing.T) {
	workspaces := &fakeWorkspace{}
	emitCalls := 0
	quitCalls := 0
	app := &App{
		api: newDesktopAPI(workspaces, &fakeDirectoryPicker{}),
		emitBeforeClose: func(context.Context) {
			emitCalls++
		},
		quit: func(context.Context) {
			quitCalls++
		},
	}
	app.api.completeClose = app.completeClose
	app.startup(context.Background())

	if prevent := app.beforeClose(context.Background()); !prevent {
		t.Fatal("first close was not prevented for frontend draft flush")
	}
	if emitCalls != 1 || quitCalls != 0 {
		t.Fatalf("before flush: emit=%d quit=%d", emitCalls, quitCalls)
	}

	app.api.CompleteClose()
	if quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1 after frontend flush", quitCalls)
	}
	if prevent := app.beforeClose(context.Background()); prevent {
		t.Fatal("close was prevented after frontend completed its flush")
	}
}

type fakeDirectoryPicker struct {
	parentPath string
	parentErr  error
	openPath   string
	openErr    error
}

func (f *fakeDirectoryPicker) ChooseProjectParent(context.Context) (string, error) {
	return f.parentPath, f.parentErr
}

func (f *fakeDirectoryPicker) ChooseProjectToOpen(context.Context) (string, error) {
	return f.openPath, f.openErr
}

type draftCall struct {
	id       string
	body     string
	revision int64
}

type fakeWorkspace struct {
	createInput            CreateProjectInput
	createResult           workspace.ProjectView
	createErr              error
	openResult             workspace.ProjectView
	openErr                error
	listResult             []workspace.ChapterView
	listErr                error
	createChapterResult    workspace.ChapterView
	createChapterErr       error
	createdChapterTitle    string
	openChapterResult      workspace.ChapterView
	openChapterErr         error
	openedChapterID        string
	savedDraft             draftCall
	discardedDraftID       string
	discardedDraftRevision int64
	saveDraftErr           error
	commitResult           workspace.ChapterView
	commitErr              error
	committed              draftCall
	closeCalls             int
	storyStructureResult   workspace.StoryStructureView
	outlineInput           workspace.CreateOutlineNodeInput
	storyLineInput         workspace.CreateStoryLineInput
	storyLinkInput         workspace.LinkStoryLineInput
	sceneCardInput         workspace.CreateSceneCardInput
	archivedOutlineID      string
	movedOutlineID         string
	moveDirection          string
	storyWorldResult       workspace.StoryWorldView
	characterInput         workspace.CharacterInput
	updatedCharacterID     string
	archivedCharacterID    string
	factionInput           workspace.FactionInput
	updatedFactionID       string
	archivedFactionID      string
	membershipInput        workspace.MembershipInput
	deletedMembership      string
	relationshipInput      workspace.RelationshipInput
	archivedRelationshipID string
	chapterAppearanceInput workspace.AppearanceInput
	sceneAppearanceInput   workspace.AppearanceInput
	unlinkedChapter        string
	unlinkedScene          string
}

func (f *fakeWorkspace) CreateProject(
	_ context.Context,
	parentDirectory, title, genre, summary string,
) (workspace.ProjectView, error) {
	f.createInput = CreateProjectInput{
		ParentDirectory: parentDirectory,
		Title:           title,
		Genre:           genre,
		Summary:         summary,
	}
	return f.createResult, f.createErr
}

func (f *fakeWorkspace) OpenProject(context.Context, string) (workspace.ProjectView, error) {
	return f.openResult, f.openErr
}

func (f *fakeWorkspace) ListChapters(context.Context) ([]workspace.ChapterView, error) {
	return f.listResult, f.listErr
}

func (f *fakeWorkspace) CreateChapter(_ context.Context, title string) (workspace.ChapterView, error) {
	f.createdChapterTitle = title
	return f.createChapterResult, f.createChapterErr
}

func (f *fakeWorkspace) OpenChapter(_ context.Context, id string) (workspace.ChapterView, error) {
	f.openedChapterID = id
	return f.openChapterResult, f.openChapterErr
}

func (f *fakeWorkspace) SaveDraft(_ context.Context, id, body string, revision int64) error {
	f.savedDraft = draftCall{id: id, body: body, revision: revision}
	return f.saveDraftErr
}

func (f *fakeWorkspace) DiscardDraft(_ context.Context, id string, revision int64) error {
	f.discardedDraftID = id
	f.discardedDraftRevision = revision
	return nil
}

func (f *fakeWorkspace) CommitChapter(
	_ context.Context,
	id, body string,
	revision int64,
) (workspace.ChapterView, error) {
	f.committed = draftCall{id: id, body: body, revision: revision}
	return f.commitResult, f.commitErr
}

func (f *fakeWorkspace) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeWorkspace) StoryStructure(context.Context) (workspace.StoryStructureView, error) {
	return f.storyStructureResult, nil
}

func (f *fakeWorkspace) CreateOutlineNode(_ context.Context, input workspace.CreateOutlineNodeInput) (workspace.OutlineNodeView, error) {
	f.outlineInput = input
	return workspace.OutlineNodeView{}, nil
}

func (f *fakeWorkspace) CreateStoryLine(_ context.Context, input workspace.CreateStoryLineInput) (workspace.StoryLineView, error) {
	f.storyLineInput = input
	return workspace.StoryLineView{}, nil
}

func (f *fakeWorkspace) LinkStoryLine(_ context.Context, input workspace.LinkStoryLineInput) (workspace.StoryLineLinkView, error) {
	f.storyLinkInput = input
	return workspace.StoryLineLinkView{}, nil
}

func (f *fakeWorkspace) CreateSceneCard(_ context.Context, input workspace.CreateSceneCardInput) (workspace.SceneCardView, error) {
	f.sceneCardInput = input
	return workspace.SceneCardView{}, nil
}

func (f *fakeWorkspace) ArchiveOutlineNode(_ context.Context, id string) error {
	f.archivedOutlineID = id
	return nil
}

func (f *fakeWorkspace) MoveOutlineNode(_ context.Context, id, direction string) error {
	f.movedOutlineID, f.moveDirection = id, direction
	return nil
}

func (f *fakeWorkspace) StoryWorld(context.Context) (workspace.StoryWorldView, error) {
	return f.storyWorldResult, nil
}
func (f *fakeWorkspace) CreateCharacter(_ context.Context, input workspace.CharacterInput) (workspace.CharacterView, error) {
	f.characterInput = input
	return workspace.CharacterView{}, nil
}
func (f *fakeWorkspace) UpdateCharacter(_ context.Context, id string, input workspace.CharacterInput) (workspace.CharacterView, error) {
	f.updatedCharacterID, f.characterInput = id, input
	return workspace.CharacterView{}, nil
}
func (f *fakeWorkspace) ArchiveCharacter(_ context.Context, id string) error {
	f.archivedCharacterID = id
	return nil
}
func (f *fakeWorkspace) CreateFaction(_ context.Context, input workspace.FactionInput) (workspace.FactionView, error) {
	f.factionInput = input
	return workspace.FactionView{}, nil
}
func (f *fakeWorkspace) UpdateFaction(_ context.Context, id string, input workspace.FactionInput) (workspace.FactionView, error) {
	f.updatedFactionID, f.factionInput = id, input
	return workspace.FactionView{}, nil
}
func (f *fakeWorkspace) ArchiveFaction(_ context.Context, id string) error {
	f.archivedFactionID = id
	return nil
}
func (f *fakeWorkspace) UpsertMembership(_ context.Context, input workspace.MembershipInput) (workspace.MembershipView, error) {
	f.membershipInput = input
	return workspace.MembershipView{}, nil
}
func (f *fakeWorkspace) DeleteMembership(_ context.Context, characterID, factionID string) error {
	f.deletedMembership = characterID + ":" + factionID
	return nil
}
func (f *fakeWorkspace) CreateRelationship(_ context.Context, input workspace.RelationshipInput) (workspace.RelationshipView, error) {
	f.relationshipInput = input
	return workspace.RelationshipView{}, nil
}
func (f *fakeWorkspace) ArchiveRelationship(_ context.Context, id string) error {
	f.archivedRelationshipID = id
	return nil
}
func (f *fakeWorkspace) LinkCharacterToChapter(_ context.Context, input workspace.AppearanceInput) error {
	f.chapterAppearanceInput = input
	return nil
}
func (f *fakeWorkspace) UnlinkCharacterFromChapter(_ context.Context, characterID, chapterID string) error {
	f.unlinkedChapter = characterID + ":" + chapterID
	return nil
}
func (f *fakeWorkspace) LinkCharacterToScene(_ context.Context, input workspace.AppearanceInput) error {
	f.sceneAppearanceInput = input
	return nil
}
func (f *fakeWorkspace) UnlinkCharacterFromScene(_ context.Context, characterID, sceneCardID string) error {
	f.unlinkedScene = characterID + ":" + sceneCardID
	return nil
}
