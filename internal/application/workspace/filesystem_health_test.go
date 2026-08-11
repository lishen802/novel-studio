package workspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	importingdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
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

func TestCreateChapterFilesystemFailureDegradesWithoutCreatingChapter(t *testing.T) {
	project := mustProject(t, "project-create-chapter", "章节失败")
	store := newFakeStore(project)
	store.createChapterErr = errors.New("disk is full")
	service := filesystemHealthService(t, project, store)

	_, err := service.CreateChapter(context.Background(), "第一章")
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("CreateChapter error = %v, want filesystem failure", err)
	}
	if len(store.chapters) != 0 {
		t.Fatalf("chapter persisted after failed create: %#v", store.chapters)
	}
	assertDegradedSharingViolation(t, service)
}

func TestArchiveCharacterFilesystemFailureDegradesWithoutArchivingCharacter(t *testing.T) {
	project := mustProject(t, "project-archive-character", "角色归档失败")
	store := newFakeStore(project)
	character := mustWorldCharacter(t, "character-archive", project.ID, "林默", worlddomain.RoleProtagonist)
	store.characters[character.ID] = character
	store.archiveCharacterErr = errors.New("sharing violation")
	service := filesystemHealthService(t, project, store)

	err := service.ArchiveCharacter(context.Background(), character.ID)
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("ArchiveCharacter error = %v, want filesystem failure", err)
	}
	if store.characters[character.ID].DeletedAt != nil {
		t.Fatalf("character archived after failed write: %#v", store.characters[character.ID])
	}
	assertDegradedSharingViolation(t, service)
}

func TestCreateSnapshotFilesystemFailureDegradesWithoutRecordingSnapshot(t *testing.T) {
	project := mustProject(t, "project-snapshot", "快照失败")
	store := newFakeStore(project)
	store.backupErr = errors.New("sharing violation")
	archive := &fakeSnapshotArchive{}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/snapshot.zjproject")},
		&fakeStoreFactory{openStore: store},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"snapshot-1"}},
		archive,
	)
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}})
	if _, err := service.OpenProject(context.Background(), "/books/snapshot.zjproject"); err != nil {
		t.Fatal(err)
	}

	_, err := service.CreateSnapshot(context.Background(), "保存点")
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("CreateSnapshot error = %v, want filesystem failure", err)
	}
	if len(archive.recorded) != 0 {
		t.Fatalf("snapshot recorded after failed backup: %#v", archive.recorded)
	}
	assertDegradedSharingViolation(t, service)
}

func TestApplyImportFilesystemFailurePreservesTypedErrorAndDegradesActiveHealth(t *testing.T) {
	project := mustProject(t, "project-import-write", "导入写入失败")
	store := newFakeImportStore(project)
	job, candidates := mustWorkspaceImportJob(t, "import-write-job", project.ID, importingdomain.JobStatusPrepared)
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	store.applyErr = errors.New("sharing violation")
	service := openImportTestService(t, store, validFakeImportArchive(), &fakeLicense{}, &sequenceIDs{ids: []string{"chapter-import"}})
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}})

	_, err := service.ApplyImportJob(context.Background(), job.ID, []string{candidates[0].ID})
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("ApplyImportJob error = %v, want filesystem failure", err)
	}
	assertDegradedSharingViolation(t, service)
}

func TestApplyImportNonFilesystemFailureRetainsStableImportError(t *testing.T) {
	project := mustProject(t, "project-import-nonfilesystem", "导入普通失败")
	store := newFakeImportStore(project)
	job, candidates := mustWorkspaceImportJob(t, "import-nonfilesystem-job", project.ID, importingdomain.JobStatusPrepared)
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	store.applyErr = errors.New("transaction aborted")
	service := openImportTestService(t, store, validFakeImportArchive(), &fakeLicense{}, &sequenceIDs{ids: []string{"chapter-import"}})
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{})

	_, err := service.ApplyImportJob(context.Background(), job.ID, []string{candidates[0].ID})
	if !errors.Is(err, ports.ErrImportPersistence) {
		t.Fatalf("ApplyImportJob error = %v, want stable import persistence error", err)
	}
	if errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("ApplyImportJob error = %v, unexpectedly classified as filesystem failure", err)
	}
	if got := activeFilesystemHealth(t, service); !reflect.DeepEqual(got, ports.NormalProjectFilesystemHealth()) {
		t.Fatalf("filesystem health = %#v, want normal", got)
	}
}

func TestAgentArtifactFilesystemFailureInBackgroundDegradesActiveHealth(t *testing.T) {
	providerFactory := &fakeModelProviderFactory{outputs: map[string]string{
		"planner":  `{"summary":"计划","scenes":[]}`,
		"writer":   "正文",
		"reviewer": `{"approved":true,"summary":"通过","issues":[]}`,
	}}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}})
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	replaceActiveStoreWithFilesystemFailure(t, service, func(store *filesystemFailingSQLiteStore) {
		store.agentArtifactErr = errors.New("sharing violation")
	})

	if _, err := service.ExecuteAgentRun(context.Background(), run.ID); err != nil {
		t.Fatalf("ExecuteAgentRun error = %v", err)
	}
	waitForAgentRunStatus(t, service, run.ID, "failed", 5*time.Second)
	failedRun, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(failedRun.ErrorMessage, ports.ErrProjectFilesystemFailure.Error()) {
		t.Fatalf("agent failure = %#v, missing filesystem failure", failedRun)
	}
	assertDegradedSharingViolation(t, service)
}

func TestExtractionArtifactFilesystemFailureInBackgroundDegradesActiveHealth(t *testing.T) {
	providerFactory := &fakeModelProviderFactory{outputs: map[string]string{
		"extraction-model": extractionEventModelOutput,
	}}
	service, taskID, _ := configuredExtractionService(t, providerFactory)
	defer service.Close()
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}})
	replaceActiveStoreWithFilesystemFailure(t, service, func(store *filesystemFailingSQLiteStore) {
		store.extractionArtifactErr = errors.New("sharing violation")
	})

	if _, err := service.ExecuteExtractionRun(context.Background(), taskID); err != nil {
		t.Fatalf("ExecuteExtractionRun error = %v", err)
	}
	waitForExtractionTaskStatus(t, service, taskID, "failed", 5*time.Second)
	failedTask, err := service.ExtractionTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(failedTask.Task.ErrorMessage, ports.ErrProjectFilesystemFailure.Error()) {
		t.Fatalf("extraction failure = %#v, missing filesystem failure", failedTask.Task)
	}
	assertDegradedSharingViolation(t, service)
}

func TestRestoreSnapshotPreservesFilesystemHealthInActiveWorkspaceAndView(t *testing.T) {
	project := mustProject(t, "project-restore-health", "恢复健康")
	currentStore := newFakeStore(project)
	restoredStore := newFakeStore(project)
	target, err := revisiondomain.NewSnapshot("snapshot-health", project.ID, "保存点", revisiondomain.SnapshotNamed, time.Unix(150, 0))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/restore-health.zjproject")},
		&fakeStoreFactory{openStores: []ports.ProjectStore{currentStore, restoredStore}},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"snapshot-recovery"}},
		&fakeSnapshotArchive{snapshots: []revisiondomain.Snapshot{target}},
	)
	if _, err := service.OpenProject(context.Background(), "/books/restore-health.zjproject"); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.active.filesystemHealth = ports.ProjectFilesystemHealth{Level: ports.ProjectFilesystemDegraded, Category: ports.ProjectFilesystemSharingViolation}
	service.mu.Unlock()

	result, err := service.RestoreSnapshot(context.Background(), target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Project.FilesystemHealth.Level; got != ports.ProjectFilesystemDegraded {
		t.Fatalf("snapshot restore view health = %#v", result.Project.FilesystemHealth)
	}
	if got := activeFilesystemHealth(t, service).Level; got != ports.ProjectFilesystemDegraded {
		t.Fatalf("active health = %#v", activeFilesystemHealth(t, service))
	}
}

func filesystemHealthService(t *testing.T, project projectdomain.Project, store *fakeStore) *Service {
	t.Helper()
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/filesystem-health.zjproject")},
		&fakeStoreFactory{openStore: store},
		license.NoopService{},
		fixedClock{now: time.Unix(100, 0)},
		&sequenceIDs{ids: []string{"generated-id"}},
		&fakeSnapshotArchive{},
	)
	service.ConfigureProjectFilesystemGuard(&fakeFilesystemGuard{failure: &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}})
	if _, err := service.OpenProject(context.Background(), "/books/filesystem-health.zjproject"); err != nil {
		t.Fatal(err)
	}
	return service
}

func assertDegradedSharingViolation(t *testing.T, service *Service) {
	t.Helper()
	health := activeFilesystemHealth(t, service)
	if health.Level != ports.ProjectFilesystemDegraded || health.Category != ports.ProjectFilesystemSharingViolation {
		t.Fatalf("filesystem health = %#v", health)
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

// filesystemFailingSQLiteStore promotes every production SQLite store method
// while overriding selected asynchronous write points.
type filesystemFailingSQLiteStore struct {
	*sqlite.Store
	agentArtifactErr      error
	extractionArtifactErr error
}

func (s *filesystemFailingSQLiteStore) CreateAgentArtifact(ctx context.Context, artifact agentdomain.AgentArtifact) error {
	if s.agentArtifactErr != nil {
		return s.agentArtifactErr
	}
	return s.Store.CreateAgentArtifact(ctx, artifact)
}

func (s *filesystemFailingSQLiteStore) CreateArtifact(ctx context.Context, artifact ports.ExtractionArtifact) error {
	if s.extractionArtifactErr != nil {
		return s.extractionArtifactErr
	}
	return s.Store.CreateArtifact(ctx, artifact)
}

func replaceActiveStoreWithFilesystemFailure(
	t *testing.T,
	service *Service,
	configure func(*filesystemFailingSQLiteStore),
) {
	t.Helper()
	service.mu.Lock()
	defer service.mu.Unlock()
	store, ok := service.active.store.(*sqlite.Store)
	if !ok {
		t.Fatalf("active store = %T, want *sqlite.Store", service.active.store)
	}
	failing := &filesystemFailingSQLiteStore{Store: store}
	configure(failing)
	service.active.store = failing
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
