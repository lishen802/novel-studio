package appshell

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

type fakeWorkspace struct {
	createResult       workspace.ProjectView
	createErr          error
	openResult         workspace.ProjectView
	openErr            error
	openPath           string
	openReadOnlyResult workspace.ProjectView
	openReadOnlyErr    error
	openReadOnlyPath   string
	closeErr           error
	closeCalls         int
}

func (f *fakeWorkspace) CreateProject(_ context.Context, _, _, _, _ string) (workspace.ProjectView, error) {
	return f.createResult, f.createErr
}

func (f *fakeWorkspace) OpenProject(_ context.Context, path string) (workspace.ProjectView, error) {
	f.openPath = path
	return f.openResult, f.openErr
}

func (f *fakeWorkspace) OpenProjectReadOnly(_ context.Context, path string) (workspace.ProjectView, error) {
	f.openReadOnlyPath = path
	return f.openReadOnlyResult, f.openReadOnlyErr
}

func (f *fakeWorkspace) Close() error {
	f.closeCalls++
	return f.closeErr
}

type fakeSettings struct {
	recent        []appdomain.RecentProject
	recorded      []appdomain.RecentProject
	recordErr     error
	recentErr     error
	markedMissing map[string]bool
	markErr       error
}

func (f *fakeSettings) RecentProjects(_ context.Context, _ int) ([]appdomain.RecentProject, error) {
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	return f.recent, nil
}

func (f *fakeSettings) RecordRecentProject(_ context.Context, rp appdomain.RecentProject) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.recorded = append(f.recorded, rp)
	return nil
}

func (f *fakeSettings) MarkProjectMissing(_ context.Context, projectID string, missing bool) error {
	if f.markErr != nil {
		return f.markErr
	}
	if f.markedMissing == nil {
		f.markedMissing = make(map[string]bool)
	}
	f.markedMissing[projectID] = missing
	return nil
}

func newTestService(t *testing.T, ws *fakeWorkspace, settings *fakeSettings) (*Service, *fakeClock) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)}
	if ws == nil {
		ws = &fakeWorkspace{}
	}
	if settings == nil {
		settings = &fakeSettings{}
	}
	return New(ws, settings, clock), clock
}

func TestOpenProjectRecordsRecentOnlyAfterSuccessfulOpen(t *testing.T) {
	project := workspace.ProjectView{ID: "p1", Title: "长夜织境", Path: "/books/长夜.zjproject"}
	ws := &fakeWorkspace{openResult: project}
	settings := &fakeSettings{}
	shell, clock := newTestService(t, ws, settings)

	result, err := shell.OpenProject(context.Background(), "/books/长夜.zjproject")
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if !reflect.DeepEqual(result.Project, project) {
		t.Fatalf("result.Project = %#v, want %#v", result.Project, project)
	}
	if ws.openPath != "/books/长夜.zjproject" {
		t.Fatalf("workspace openPath = %q", ws.openPath)
	}
	if len(settings.recorded) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(settings.recorded))
	}
	recorded := settings.recorded[0]
	if recorded.ProjectID != "p1" || recorded.Title != "长夜织境" || recorded.Path != "/books/长夜.zjproject" {
		t.Fatalf("recorded = %#v", recorded)
	}
	if !recorded.LastOpenedAt.Equal(clock.Now()) {
		t.Fatalf("LastOpenedAt = %s, want %s", recorded.LastOpenedAt, clock.Now())
	}
	if result.Warning != "" {
		t.Fatalf("unexpected warning: %q", result.Warning)
	}
}

func TestOpenProjectDoesNotRecordRecentOnFailure(t *testing.T) {
	ws := &fakeWorkspace{openErr: errors.New("workspace unavailable")}
	settings := &fakeSettings{}
	shell, _ := newTestService(t, ws, settings)

	_, err := shell.OpenProject(context.Background(), "/books/missing.zjproject")
	if err == nil {
		t.Fatal("OpenProject() error = nil, want error")
	}
	if len(settings.recorded) != 0 {
		t.Fatalf("recorded %d entries, want 0", len(settings.recorded))
	}
}

func TestRecentOpenLeaseConflictDoesNotAdvanceLastOpenedAt(t *testing.T) {
	ws := &fakeWorkspace{openErr: &ports.ProjectLeaseConflict{PackagePath: "/books/a.zjproject"}}
	settings := &fakeSettings{}
	shell, _ := newTestService(t, ws, settings)

	_, err := shell.OpenProject(context.Background(), "/books/a.zjproject")
	if err == nil {
		t.Fatal("OpenProject() error = nil, want lease conflict")
	}
	if len(settings.recorded) != 0 {
		t.Fatalf("recent writes = %d, want 0", len(settings.recorded))
	}
}

func TestOpenProjectReadOnlyRecordsRecentOnSuccess(t *testing.T) {
	project := workspace.ProjectView{ID: "p2", Title: "雾港来信", Path: "/books/雾港.zjproject"}
	ws := &fakeWorkspace{openReadOnlyResult: project}
	settings := &fakeSettings{}
	shell, _ := newTestService(t, ws, settings)

	result, err := shell.OpenProjectReadOnly(context.Background(), "/books/雾港.zjproject")
	if err != nil {
		t.Fatalf("OpenProjectReadOnly() error = %v", err)
	}
	if !reflect.DeepEqual(result.Project, project) {
		t.Fatalf("result.Project = %#v, want %#v", result.Project, project)
	}
	if len(settings.recorded) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(settings.recorded))
	}
}

func TestCreateProjectRecordsRecentOnSuccess(t *testing.T) {
	project := workspace.ProjectView{ID: "p3", Title: "新书", Path: "/books/新书.zjproject"}
	ws := &fakeWorkspace{createResult: project}
	settings := &fakeSettings{}
	shell, _ := newTestService(t, ws, settings)

	result, err := shell.CreateProject(context.Background(), "/books", "新书", "奇幻", "简介")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if !reflect.DeepEqual(result.Project, project) {
		t.Fatalf("result.Project = %#v, want %#v", result.Project, project)
	}
	if len(settings.recorded) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(settings.recorded))
	}
}

func TestRecordRecentFailureReturnsWarningButKeepsProjectOpen(t *testing.T) {
	project := workspace.ProjectView{ID: "p4", Title: "长夜", Path: "/books/长夜.zjproject"}
	ws := &fakeWorkspace{openResult: project}
	settings := &fakeSettings{recordErr: errors.New("settings database unavailable")}
	shell, _ := newTestService(t, ws, settings)

	result, err := shell.OpenProject(context.Background(), "/books/长夜.zjproject")
	if err != nil {
		t.Fatalf("OpenProject() error = %v, want nil (project stays open)", err)
	}
	if !reflect.DeepEqual(result.Project, project) {
		t.Fatalf("result.Project = %#v, want %#v", result.Project, project)
	}
	if result.Warning == "" {
		t.Fatal("expected non-empty warning when recording fails")
	}
}

func TestMissingRecentProjectReturnsRelocateAction(t *testing.T) {
	ws := &fakeWorkspace{openErr: fs.ErrNotExist}
	settings := &fakeSettings{
		recent: []appdomain.RecentProject{
			{ProjectID: "p1", Title: "长夜", Path: "/books/长夜.zjproject"},
		},
	}
	shell, _ := newTestService(t, ws, settings)

	_, err := shell.OpenRecentProject(context.Background(), "p1")
	if !errors.Is(err, ErrProjectRelocateRequired) {
		t.Fatalf("OpenRecentProject() error = %v, want ErrProjectRelocateRequired", err)
	}
	if !settings.markedMissing["p1"] {
		t.Fatalf("project p1 was not marked missing")
	}
}

func TestOpenRecentProjectNotFound(t *testing.T) {
	ws := &fakeWorkspace{}
	settings := &fakeSettings{
		recent: []appdomain.RecentProject{
			{ProjectID: "p1", Title: "长夜", Path: "/books/长夜.zjproject"},
		},
	}
	shell, _ := newTestService(t, ws, settings)

	_, err := shell.OpenRecentProject(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRecentProjectNotFound) {
		t.Fatalf("OpenRecentProject() error = %v, want ErrRecentProjectNotFound", err)
	}
	if ws.openPath != "" {
		t.Fatalf("workspace was opened for a missing recent id, path = %q", ws.openPath)
	}
}

func TestOpenRecentProjectSucceedsAndRecordsRecent(t *testing.T) {
	project := workspace.ProjectView{ID: "p1", Title: "长夜", Path: "/books/长夜.zjproject"}
	ws := &fakeWorkspace{openResult: project}
	settings := &fakeSettings{
		recent: []appdomain.RecentProject{
			{ProjectID: "p1", Title: "长夜", Path: "/books/长夜.zjproject"},
		},
	}
	shell, _ := newTestService(t, ws, settings)

	result, err := shell.OpenRecentProject(context.Background(), "p1")
	if err != nil {
		t.Fatalf("OpenRecentProject() error = %v", err)
	}
	if !reflect.DeepEqual(result.Project, project) {
		t.Fatalf("result.Project = %#v, want %#v", result.Project, project)
	}
	if len(settings.recorded) != 1 {
		t.Fatalf("recorded %d entries, want 1", len(settings.recorded))
	}
}

func TestCloseDelegatesToWorkspace(t *testing.T) {
	ws := &fakeWorkspace{}
	shell, _ := newTestService(t, ws, nil)

	if err := shell.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if ws.closeCalls != 1 {
		t.Fatalf("workspace close calls = %d, want 1", ws.closeCalls)
	}
}
