package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
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
	events *[]string
	err    error
}

func (f *fakeLicense) Require(context.Context, license.Capability) error {
	appendEvent(f.events, "license")
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
	project     projectdomain.Project
	chapters    map[string]projectdomain.Chapter
	draft       draftRecord
	savedDraft  draftRecord
	commitCalls int
	closeEvent  string
	events      *[]string
}

func newFakeStore(project projectdomain.Project) *fakeStore {
	return &fakeStore{project: project, chapters: make(map[string]projectdomain.Chapter)}
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

func (f *fakeStore) Draft(_ context.Context, chapterID string) (string, int64, bool, error) {
	if !f.draft.found || f.draft.chapterID != chapterID {
		return "", 0, false, nil
	}
	return f.draft.body, f.draft.baseRevision, true, nil
}

func (f *fakeStore) CommitChapter(_ context.Context, chapter, _ projectdomain.Chapter) error {
	f.commitCalls++
	f.chapters[chapter.ID] = chapter
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
