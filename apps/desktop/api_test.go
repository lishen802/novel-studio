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
