package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/workspace"
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
	gotCommit, err := api.CommitChapter(chapter.ID, "正文", 1)
	if err != nil || gotCommit != committed {
		t.Fatalf("CommitChapter = (%#v, %v)", gotCommit, err)
	}
	if workspaces.committed != (draftCall{id: chapter.ID, body: "正文", revision: 1}) {
		t.Fatalf("commit call = %#v", workspaces.committed)
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

func TestDesktopAPIPropagatesWorkspaceError(t *testing.T) {
	want := errors.New("project is corrupt")
	api := newDesktopAPI(
		&fakeWorkspace{openErr: want},
		&fakeDirectoryPicker{},
	)
	api.startup(context.Background())

	_, err := api.OpenProject("/books/broken.zjproject")
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestAppShutdownClosesWorkspace(t *testing.T) {
	workspaces := &fakeWorkspace{}
	app := &App{api: newDesktopAPI(workspaces, &fakeDirectoryPicker{})}

	app.shutdown(context.Background())

	if workspaces.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", workspaces.closeCalls)
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
	createInput         CreateProjectInput
	createResult        workspace.ProjectView
	createErr           error
	openResult          workspace.ProjectView
	openErr             error
	listResult          []workspace.ChapterView
	listErr             error
	createChapterResult workspace.ChapterView
	createChapterErr    error
	createdChapterTitle string
	openChapterResult   workspace.ChapterView
	openChapterErr      error
	openedChapterID     string
	savedDraft          draftCall
	saveDraftErr        error
	commitResult        workspace.ChapterView
	commitErr           error
	committed           draftCall
	closeCalls          int
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
