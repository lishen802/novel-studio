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
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

type fakeProjectDefaults struct {
	selection ports.ProjectProfileSelection
	err       error
}

func (f *fakeProjectDefaults) DefaultProfileSelection(context.Context) (ports.ProjectProfileSelection, error) {
	return f.selection, f.err
}

func TestCreateProjectPinsCurrentDefaultProfileVersion(t *testing.T) {
	ctx := context.Background()
	defaults := &fakeProjectDefaults{selection: ports.ProjectProfileSelection{ProfileID: "profile-1", Version: 3}}

	service := newWorkbenchTestService(t)
	service.ConfigureProjectDefaults(defaults)

	parentDir := t.TempDir()
	view, err := service.CreateProject(ctx, parentDir, "新书", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if view.ProfileSelection.ProfileID != "profile-1" || view.ProfileSelection.Version != 3 {
		t.Fatalf("selection = %#v, want profile-1 v3", view.ProfileSelection)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	// Changing the default after creation must not affect the existing project.
	defaults.selection.Version = 4

	reopened := newWorkbenchTestService(t)
	reopened.ConfigureProjectDefaults(defaults)
	t.Cleanup(func() { _ = reopened.Close() })

	reopenedView, err := reopened.OpenProject(ctx, view.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopenedView.ProfileSelection.Version != 3 {
		t.Fatalf("existing project moved to default v%d, want 3", reopenedView.ProfileSelection.Version)
	}
}

func TestOpenProjectNeverUpgradesPinnedProfileImplicitly(t *testing.T) {
	ctx := context.Background()

	createDefaults := &fakeProjectDefaults{selection: ports.ProjectProfileSelection{ProfileID: "profile-1", Version: 3}}
	createService := newWorkbenchTestService(t)
	createService.ConfigureProjectDefaults(createDefaults)
	parentDir := t.TempDir()
	view, err := createService.CreateProject(ctx, parentDir, "v3项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := createService.Close(); err != nil {
		t.Fatal(err)
	}

	// Open with a different default version; the pin must stay at v3.
	openDefaults := &fakeProjectDefaults{selection: ports.ProjectProfileSelection{ProfileID: "profile-1", Version: 4}}
	openService := newWorkbenchTestService(t)
	openService.ConfigureProjectDefaults(openDefaults)
	t.Cleanup(func() { _ = openService.Close() })

	reopenedView, err := openService.OpenProject(ctx, view.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopenedView.ProfileSelection.Version != 3 {
		t.Fatalf("profile version = %d, want 3 (pinned, not upgraded to v4)", reopenedView.ProfileSelection.Version)
	}
}

func TestCreateProjectProfilePinFailureReleasesLease(t *testing.T) {
	calls := make([]string, 0)
	project := mustProject(t, "project-1", "新书")
	store := newFakeStore(project)
	store.closeEvent = "store.close"
	store.events = &calls

	leaseManager := &lifecycleLeaseManager{calls: &calls}
	service := NewService(
		&fakePackageManager{createPackage: testPackage("/books/新书.zjproject")},
		&fakeStoreFactory{createStore: store},
		&fakeLicense{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-1"}},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectLease(leaseManager, ports.ProjectLeaseMetadata{})
	service.ConfigureProjectDefaults(&fakeProjectDefaults{err: errors.New("settings unavailable")})

	_, err := service.CreateProject(context.Background(), "/books", "新书", "", "")
	if err == nil {
		t.Fatal("CreateProject() succeeded; want settings unavailable error")
	}

	want := []string{"store.close", "lease.release"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("cleanup order = %v, want %v", calls, want)
	}
}

func TestReadOnlyProjectReturnsProjectionWithoutWritingState(t *testing.T) {
	ctx := context.Background()

	createService := newWorkbenchTestService(t)
	parentDir := t.TempDir()
	view, err := createService.CreateProject(ctx, parentDir, "只读工作台", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createService.CreateChapter(ctx, "第一章"); err != nil {
		t.Fatal(err)
	}
	// Persist workbench state in the editable session so the read-only session
	// has something to read.
	if _, err := createService.Workbench(ctx); err != nil {
		t.Fatal(err)
	}
	if err := createService.Close(); err != nil {
		t.Fatal(err)
	}

	readOnly := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = readOnly.Close() })
	if _, err := readOnly.OpenProjectReadOnly(ctx, view.Path); err != nil {
		t.Fatal(err)
	}

	// Snapshot file hashes after the read-only open (which may create WAL/SHM
	// sidecars) so the comparison isolates Workbench() writes.
	before := projectFileHashes(t, view.Path)
	projection, err := readOnly.Workbench(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Stage != projectdomain.StageWriting {
		t.Fatalf("read-only stage = %q, want %q", projection.Stage, projectdomain.StageWriting)
	}

	after := projectFileHashes(t, view.Path)
	if !equalProjectHashes(before, after) {
		t.Fatalf("read-only Workbench() modified project files: before=%v after=%v", before, after)
	}
}

func TestWorkbenchProjectionSurvivesServiceRestart(t *testing.T) {
	ctx := context.Background()

	createService := newWorkbenchTestService(t)
	parentDir := t.TempDir()
	view, err := createService.CreateProject(ctx, parentDir, "重启工作台", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createService.CreateChapter(ctx, "第一章"); err != nil {
		t.Fatal(err)
	}
	before, err := createService.Workbench(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Stage != projectdomain.StageWriting || before.Canvas != "manuscript" {
		t.Fatalf("before projection = %#v, want stage=writing canvas=manuscript", before)
	}
	if err := createService.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = reopened.Close() })
	if _, err := reopened.OpenProject(ctx, view.Path); err != nil {
		t.Fatal(err)
	}
	after, err := reopened.Workbench(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("workbench projection changed across restart: before=%#v after=%#v", before, after)
	}
}

func TestWorkbenchDefaultProjectionForProjectWithoutChapters(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "空项目", "奇幻", ""); err != nil {
		t.Fatal(err)
	}
	projection, err := service.Workbench(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Stage != projectdomain.StageIdea {
		t.Fatalf("stage = %q, want %q", projection.Stage, projectdomain.StageIdea)
	}
	if projection.Canvas != "" {
		t.Fatalf("canvas = %q, want empty", projection.Canvas)
	}
	if projection.AIPanel != "hidden" {
		t.Fatalf("aiPanel = %q, want hidden", projection.AIPanel)
	}
	if projection.ArchiveStatus != "not_generated" {
		t.Fatalf("archiveStatus = %q, want not_generated", projection.ArchiveStatus)
	}
}

func TestCreateProjectWithoutDefaultsSkipsPinning(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	// No ConfigureProjectDefaults call — backward compatible.
	view, err := service.CreateProject(ctx, t.TempDir(), "无默认项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if view.ProfileSelection.ProfileID != "" || view.ProfileSelection.Version != 0 {
		t.Fatalf("selection = %#v, want zero value when defaults not configured", view.ProfileSelection)
	}
}

// newWorkbenchTestService creates a service backed by real SQLite stores and
// the real project filesystem manager, matching the pattern in read_only_test.go.
func newWorkbenchTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(
		projectfs.Manager{}, sqlite.Factory{}, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{},
	)
}

type lifecycleLeaseManager struct {
	calls *[]string
}

func (m *lifecycleLeaseManager) Acquire(ports.ProjectPackage, ports.ProjectLeaseMetadata) (ports.ProjectLease, error) {
	return &lifecycleLease{calls: m.calls}, nil
}

type lifecycleLease struct {
	calls *[]string
}

func (l *lifecycleLease) UpdateMetadata(ports.ProjectLeaseMetadata) error { return nil }

func (l *lifecycleLease) Release() error {
	*l.calls = append(*l.calls, "lease.release")
	return nil
}
