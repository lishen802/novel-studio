package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
)

func TestOpenProjectPreflightFailureKeepsCurrentWorkspace(t *testing.T) {
	currentProject := mustProject(t, "project-current", "当前项目")
	nextProject := mustProject(t, "project-next", "下一个项目")
	currentStore := newFakeStore(currentProject)
	chapter := mustChapter(t, "chapter-current", currentProject.ID, "第一章", 1)
	currentStore.chapters[chapter.ID] = chapter
	nextStore := newFakeStore(nextProject)
	packages := &fakePackageManager{
		createPackage: testPackage("/books/current.zjproject"),
		openPackage:   testPackage("/books/next.zjproject"),
	}
	factory := &fakeStoreFactory{createStore: currentStore, openStore: nextStore}
	service := NewService(
		packages,
		factory,
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"project-current"}},
		&fakeSnapshotArchive{},
	)
	if _, err := service.CreateProject(context.Background(), "/books", "当前项目", "", ""); err != nil {
		t.Fatal(err)
	}
	leases := &fakeProjectLeaseManager{}
	service.ConfigureProjectLease(leases, ports.ProjectLeaseMetadata{})
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{
		preflightErr: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPreflight},
	})

	_, err := service.OpenProject(context.Background(), packages.openPackage.Root)
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("OpenProject error = %v, want filesystem failure", err)
	}
	if got := activeWorkspacePath(t, service); got != packages.createPackage.Root {
		t.Fatalf("active path = %q, want %q", got, packages.createPackage.Root)
	}
	if factory.openCalls != 0 {
		t.Fatalf("opened failed project store %d times", factory.openCalls)
	}
	if len(leases.leases) != 1 || leases.leases[0].released != true {
		t.Fatalf("new project lease was not released: %#v", leases.leases)
	}
}

func TestFilesystemPreflightRunsOnlyForEditableOpen(t *testing.T) {
	project := mustProject(t, "project-read-only", "只读项目")
	guard := &fakeFilesystemGuard{}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/read-only.zjproject")},
		&fakeStoreFactory{readOnlyStore: newFakeStore(project), openStore: newFakeStore(project)},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(guard)

	readOnly, err := service.OpenProjectReadOnly(context.Background(), "/books/read-only.zjproject")
	if err != nil {
		t.Fatal(err)
	}
	if guard.preflightCalls != 0 {
		t.Fatalf("read-only open preflight calls = %d, want 0", guard.preflightCalls)
	}
	if !reflect.DeepEqual(readOnly.FilesystemHealth, ports.NormalProjectFilesystemHealth()) {
		t.Fatalf("read-only health = %#v", readOnly.FilesystemHealth)
	}

	if _, err := service.OpenProject(context.Background(), "/books/read-only.zjproject"); err != nil {
		t.Fatal(err)
	}
	if guard.preflightCalls != 1 {
		t.Fatalf("editable open preflight calls = %d, want 1", guard.preflightCalls)
	}
}

func TestCommitFilesystemFailureDegradesWithoutChangingRevision(t *testing.T) {
	project := mustProject(t, "project-commit", "提交项目")
	chapter := mustChapter(t, "chapter-commit", project.ID, "第一章", 1)
	store := newFakeStore(project)
	store.chapters[chapter.ID] = chapter
	store.commitErr = errors.New("sharing violation")
	guard := &fakeFilesystemGuard{
		failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation},
	}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/commit.zjproject")},
		&fakeStoreFactory{openStore: store},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(guard)
	if _, err := service.OpenProject(context.Background(), "/books/commit.zjproject"); err != nil {
		t.Fatal(err)
	}

	_, err := service.CommitChapter(context.Background(), chapter.ID, "未保存文本", chapter.Revision)
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("CommitChapter error = %v, want filesystem failure", err)
	}
	if got := activeFilesystemHealth(t, service); got.Level != ports.ProjectFilesystemDegraded || got.Category != ports.ProjectFilesystemSharingViolation {
		t.Fatalf("filesystem health = %#v", got)
	}
	if got := store.chapters[chapter.ID].Revision; got != chapter.Revision {
		t.Fatalf("stored revision = %d, want %d", got, chapter.Revision)
	}
}

func TestProjectViewIncludesNormalFilesystemHealth(t *testing.T) {
	project := mustProject(t, "project-normal", "正常项目")
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/normal.zjproject")},
		&fakeStoreFactory{openStore: newFakeStore(project)},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)

	view, err := service.OpenProject(context.Background(), "/books/normal.zjproject")
	if err != nil {
		t.Fatal(err)
	}
	if got := view.FilesystemHealth; !reflect.DeepEqual(got, ports.NormalProjectFilesystemHealth()) {
		t.Fatalf("filesystem health = %#v", got)
	}
}

func TestRecheckFilesystemHealthClearsDegradedState(t *testing.T) {
	project := mustProject(t, "project-recheck", "复检项目")
	guard := &fakeFilesystemGuard{health: ports.ProjectFilesystemHealth{
		Level:    ports.ProjectFilesystemWarning,
		Category: ports.ProjectFilesystemSyncRisk,
	}}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/recheck.zjproject")},
		&fakeStoreFactory{openStore: newFakeStore(project)},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(guard)
	if _, err := service.OpenProject(context.Background(), "/books/recheck.zjproject"); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.active.filesystemHealth = ports.ProjectFilesystemHealth{Level: ports.ProjectFilesystemDegraded}
	service.mu.Unlock()

	health, err := service.RecheckFilesystemHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(health, guard.health) || !reflect.DeepEqual(activeFilesystemHealth(t, service), guard.health) {
		t.Fatalf("rechecked health = %#v, active = %#v", health, activeFilesystemHealth(t, service))
	}
}

func TestCreateRecoveryCopyRejectsFilesystemPathFailureBeforeCopying(t *testing.T) {
	project := mustProject(t, "project-copy-path", "恢复副本")
	source := fakeProjectPackage("/books/source.zjproject")
	published := fakeProjectPackage("/copies/source-recovery.zjproject")
	guard := &fakeFilesystemGuard{
		recoveryCopyErr: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPathTooLong},
	}
	packages := &fakePackageManager{openPackage: source, copyPackage: published}
	factory := &fakeStoreFactory{readOnlyStore: newFakeStore(project), openStore: newFakeStore(project)}
	service := NewService(
		packages,
		factory,
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(guard)

	_, err := service.CreateProjectRecoveryCopy(context.Background(), source.Root, "/copies")
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("CreateProjectRecoveryCopy error = %v, want filesystem failure", err)
	}
	if factory.readOnlyCalls != 0 || packages.copyCalls != 0 {
		t.Fatalf("recovery copy reached source/archive after validation failure: readOnly=%d copy=%d", factory.readOnlyCalls, packages.copyCalls)
	}
}

func TestRecoveryCopyFilesystemFailureDegradesActiveSourceHealth(t *testing.T) {
	project := mustProject(t, "project-copy-write", "恢复副本")
	source := fakeProjectPackage("/books/source-write.zjproject")
	published := fakeProjectPackage("/copies/source-write-recovery.zjproject")
	activeStore := newFakeStore(project)
	readOnlyStore := newFakeStore(project)
	readOnlyStore.backupErr = errors.New("sharing violation")
	guard := &fakeFilesystemGuard{
		failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation},
	}
	service := NewService(
		&fakePackageManager{openPackage: source, copyPackage: published},
		&fakeStoreFactory{openStore: activeStore, readOnlyStore: readOnlyStore},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(guard)
	if _, err := service.OpenProject(context.Background(), source.Root); err != nil {
		t.Fatal(err)
	}

	_, err := service.CreateProjectRecoveryCopy(context.Background(), source.Root, "/copies")
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("CreateProjectRecoveryCopy error = %v, want filesystem failure", err)
	}
	if got := activeFilesystemHealth(t, service); got.Level != ports.ProjectFilesystemDegraded || got.Category != ports.ProjectFilesystemSharingViolation {
		t.Fatalf("filesystem health = %#v", got)
	}
}

func activeWorkspacePath(t *testing.T, service *Service) string {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.active == nil {
		t.Fatal("active workspace is nil")
	}
	return service.active.packageInfo.Root
}

func activeFilesystemHealth(t *testing.T, service *Service) ports.ProjectFilesystemHealth {
	t.Helper()
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.active == nil {
		t.Fatal("active workspace is nil")
	}
	return service.active.filesystemHealth
}

type fakeFilesystemGuard struct {
	health          ports.ProjectFilesystemHealth
	preflightErr    error
	failure         *ports.ProjectFilesystemFailure
	recoveryCopyErr error
	preflightCalls  int
}

func (f *fakeFilesystemGuard) PreflightEditable(ports.ProjectPackage) (ports.ProjectFilesystemHealth, error) {
	f.preflightCalls++
	if f.preflightErr != nil {
		return ports.NormalProjectFilesystemHealth(), f.preflightErr
	}
	if f.health.Level == "" {
		return ports.NormalProjectFilesystemHealth(), nil
	}
	return f.health, nil
}

func (f *fakeFilesystemGuard) ValidateCreatePath(string, string) error { return nil }

func (f *fakeFilesystemGuard) ValidateRecoveryCopyPath(ports.ProjectPackage, string) error {
	return f.recoveryCopyErr
}

func (f *fakeFilesystemGuard) ClassifyRuntimeError(error) *ports.ProjectFilesystemFailure {
	return f.failure
}
