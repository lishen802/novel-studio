package workspace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type Service struct {
	packages    ports.ProjectPackageManager
	stores      ports.ProjectStoreFactory
	license     license.Service
	clock       ports.Clock
	ids         ports.IDGenerator
	snapshots   ports.SnapshotArchive
	branches    ports.BranchArchive
	imports     ports.ImportSourceArchive
	credentials ports.CredentialStore
	providers   ports.ModelProviderFactory

	lifecycleMu sync.Mutex
	closed      bool

	mu                  sync.RWMutex
	active              *activeWorkspace
	branchMergePreviews map[string]branchMergePreview

	agentJobsMu      sync.Mutex
	agentJobs        map[string]context.CancelFunc
	extractionJobsMu sync.Mutex
	extractionJobs   map[string]context.CancelFunc
	versionOperation bool
	importOperation  bool

	// storeGeneration is a monotonic counter incremented whenever the active
	// project store changes (create/open/switch/close/recovery). Jobs capture
	// the generation when they acquire a lease and re-check it before each
	// persistence step so they never write to a store that has been replaced.
	storeGeneration uint64
	// leases coordinates draining in-flight jobs before a project store is
	// closed. New jobs acquire a lease bound to the current generation; switch
	// and close drain leases (after cancelling job contexts) before closing the
	// previous store.
	leases *leaseRegistry
	// drainTimeout bounds how long switch/close waits for in-flight job leases
	// to release before force-marking them stale. Tests inject shorter values.
	drainTimeout time.Duration
	// notifier delivers best-effort job:updated wake-ups to the UI bridge after
	// each JobEvent commit. It defaults to a noop so tests that don't assert
	// notifications are unaffected; the Wails layer injects a real emitter.
	notifier ports.JobEventNotifier
}

type activeWorkspace struct {
	packageInfo ports.ProjectPackage
	store       ports.ProjectStore
	generation  uint64
}

func NewService(
	packages ports.ProjectPackageManager,
	stores ports.ProjectStoreFactory,
	licenseService license.Service,
	clock ports.Clock,
	ids ports.IDGenerator,
	snapshots ports.SnapshotArchive,
	branchArchives ...ports.BranchArchive,
) *Service {
	service := &Service{
		packages:            packages,
		stores:              stores,
		license:             licenseService,
		clock:               clock,
		ids:                 ids,
		snapshots:           snapshots,
		branchMergePreviews: make(map[string]branchMergePreview),
		agentJobs:           make(map[string]context.CancelFunc),
		extractionJobs:      make(map[string]context.CancelFunc),
		storeGeneration:     0,
		leases:              newLeaseRegistry(),
		drainTimeout:        drainTimeoutDefault,
		notifier:            noopJobEventNotifier{},
	}
	if len(branchArchives) > 0 {
		service.branches = branchArchives[0]
	}
	return service
}

func (s *Service) ConfigureAgentRuntime(
	credentials ports.CredentialStore,
	providers ports.ModelProviderFactory,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials = credentials
	s.providers = providers
}

// ConfigureImportSourceArchive installs the platform adapter used to validate,
// decode, and archive manuscript sources. It is separate from NewService so
// existing non-import callers keep their construction contract.
func (s *Service) ConfigureImportSourceArchive(archive ports.ImportSourceArchive) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imports = archive
}

func (s *Service) CreateProject(
	ctx context.Context,
	parentDirectory, title, genre, summary string,
) (ProjectView, error) {
	if err := s.license.Require(ctx, license.CreateProject); err != nil {
		return ProjectView{}, err
	}
	if err := s.beginLifecycleOperation(); err != nil {
		return ProjectView{}, err
	}
	defer s.endLifecycleOperation()

	project, err := projectdomain.NewProject(s.ids.NewID(), title, genre, summary, s.clock.Now())
	if err != nil {
		return ProjectView{}, err
	}
	packageInfo, err := s.packages.Create(parentDirectory, project.Title)
	if err != nil {
		return ProjectView{}, fmt.Errorf("create project package: %w", err)
	}
	store, err := s.stores.Create(ctx, packageInfo.Database, project)
	if err != nil {
		return ProjectView{}, fmt.Errorf("create project store: %w", err)
	}

	candidate := activeWorkspace{packageInfo: packageInfo, store: store}
	if err := s.recoverInterruptedJobs(ctx, store); err != nil {
		_ = store.Close()
		return ProjectView{}, err
	}
	if err := s.ensureBranchCatalog(ctx, &candidate); err != nil {
		_ = store.Close()
		return ProjectView{}, err
	}
	s.replaceActive(candidate)
	return projectView(project, packageInfo.Root), nil
}

// recoverInterruptedJobs transitions agent runs and extraction tasks left in
// the Running state by a previous process into Recoverable, appending a
// job_recovered event for each so the UI can replay the recovery on remount.
// Called during CreateProject/OpenProject after the store is opened but before
// it becomes the active workspace.
func (s *Service) recoverInterruptedJobs(ctx context.Context, store ports.ProjectStore) error {
	now := s.clock.Now()
	if agentStore, ok := store.(ports.AgentStore); ok {
		if _, err := agentStore.RecoverInterruptedAgentRuns(ctx, now); err != nil {
			return fmt.Errorf("recover interrupted agent runs: %w", err)
		}
	}
	if extractionStore, ok := store.(ports.ExtractionStore); ok {
		if _, err := extractionStore.RecoverInterruptedTasks(ctx, now); err != nil {
			return fmt.Errorf("recover interrupted extraction tasks: %w", err)
		}
	}
	return nil
}

func (s *Service) OpenProject(ctx context.Context, packagePath string) (ProjectView, error) {
	if err := s.beginLifecycleOperation(); err != nil {
		return ProjectView{}, err
	}
	defer s.endLifecycleOperation()

	packageInfo, err := s.packages.Inspect(packagePath)
	if err != nil {
		return ProjectView{}, fmt.Errorf("inspect project package: %w", err)
	}
	if err := s.packages.PrepareEditable(packageInfo); err != nil {
		return ProjectView{}, fmt.Errorf("prepare project package for editing: %w", err)
	}
	if err := s.snapshots.RecoverInterruptedRestore(packageInfo.Database); err != nil {
		return ProjectView{}, fmt.Errorf("recover interrupted project restore: %w", err)
	}
	if s.branches != nil {
		if err := s.branches.RecoverInterruptedOperation(packageInfo.Branches, packageInfo.Database); err != nil {
			return ProjectView{}, fmt.Errorf("recover interrupted branch operation: %w", err)
		}
	}
	store, err := s.stores.Open(ctx, packageInfo.Database)
	if err != nil {
		return ProjectView{}, fmt.Errorf("open project store: %w", err)
	}
	project, err := store.Project(ctx)
	if err != nil {
		_ = store.Close()
		return ProjectView{}, fmt.Errorf("read project: %w", err)
	}

	candidate := activeWorkspace{packageInfo: packageInfo, store: store}
	if err := s.recoverInterruptedJobs(ctx, store); err != nil {
		_ = store.Close()
		return ProjectView{}, err
	}
	if err := s.ensureBranchCatalog(ctx, &candidate); err != nil {
		_ = store.Close()
		return ProjectView{}, err
	}
	s.replaceActive(candidate)
	return projectView(project, packageInfo.Root), nil
}

func (s *Service) ListChapters(ctx context.Context) ([]ChapterView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}

	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	views := make([]ChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		views = append(views, chapterView(chapter))
	}
	return views, nil
}

func (s *Service) CreateChapter(ctx context.Context, title string) (ChapterView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChapterView{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ChapterView{}, ErrNoActiveProject
	}

	project, err := s.active.store.Project(ctx)
	if err != nil {
		return ChapterView{}, fmt.Errorf("read active project: %w", err)
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return ChapterView{}, fmt.Errorf("list chapters: %w", err)
	}
	nextPosition := 1
	for _, chapter := range chapters {
		if chapter.Position >= nextPosition {
			nextPosition = chapter.Position + 1
		}
	}
	chapter, err := projectdomain.NewChapter(
		s.ids.NewID(),
		project.ID,
		title,
		nextPosition,
		s.clock.Now(),
	)
	if err != nil {
		return ChapterView{}, err
	}
	if err := s.active.store.CreateChapter(ctx, chapter); err != nil {
		return ChapterView{}, fmt.Errorf("create chapter: %w", err)
	}
	return chapterView(chapter), nil
}

func (s *Service) OpenChapter(ctx context.Context, chapterID string) (ChapterView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ChapterView{}, ErrNoActiveProject
	}

	chapter, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		return ChapterView{}, fmt.Errorf("open chapter: %w", err)
	}
	view := chapterView(chapter)
	body, baseRevision, draftUpdatedAt, found, err := s.active.store.Draft(ctx, chapterID)
	if err != nil {
		return ChapterView{}, fmt.Errorf("read recovery draft: %w", err)
	}
	if found && baseRevision == chapter.Revision {
		view.RecoveryBody = body
		view.RecoveryUpdatedAt = draftUpdatedAt.UTC().Format(time.RFC3339Nano)
		view.HasRecoveryDraft = true
	}
	return view, nil
}

func (s *Service) SaveDraft(ctx context.Context, chapterID, body string, baseRevision int64) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ErrNoActiveProject
	}
	if err := s.active.store.SaveDraft(ctx, chapterID, body, baseRevision); err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	return nil
}

func (s *Service) DiscardDraft(ctx context.Context, chapterID string, baseRevision int64) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ErrNoActiveProject
	}
	if err := s.active.store.DeleteDraft(ctx, chapterID, baseRevision); err != nil {
		return fmt.Errorf("discard recovery draft: %w", err)
	}
	return nil
}

func (s *Service) CommitChapter(
	ctx context.Context,
	chapterID, body string,
	baseRevision int64,
) (ChapterView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChapterView{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ChapterView{}, ErrNoActiveProject
	}
	previous, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		return ChapterView{}, fmt.Errorf("read chapter before commit: %w", err)
	}
	updated, err := previous.Commit(body, baseRevision, s.clock.Now())
	if err != nil {
		return ChapterView{}, err
	}
	if err := s.active.store.CommitChapter(ctx, updated, previous); err != nil {
		return ChapterView{}, fmt.Errorf("commit chapter: %w", err)
	}
	return chapterView(updated), nil
}

func (s *Service) Close() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	// Reject new jobs, signal in-flight jobs to exit, then wait for their
	// leases to drain before closing the store. This prevents jobs from
	// writing to a closed store.
	s.leases.beginDraining()
	s.cancelAllAgentJobs()
	s.cancelAllExtractionJobs()
	_ = s.leases.drainAndBlock(context.Background(), s.drainTimeout)
	s.mu.Lock()
	active := s.active
	s.active = nil
	s.branchMergePreviews = make(map[string]branchMergePreview)
	s.mu.Unlock()
	s.leases.endDraining()
	if active == nil {
		return nil
	}
	return active.store.Close()
}

func (s *Service) beginLifecycleOperation() error {
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		return ErrServiceClosed
	}
	return nil
}

func (s *Service) endLifecycleOperation() {
	s.lifecycleMu.Unlock()
}

// replaceActive must be called while lifecycleMu is held. It drains in-flight
// job leases before swapping the active workspace so jobs don't write to the
// closed store.
func (s *Service) replaceActive(next activeWorkspace) {
	// Reject new jobs, then signal in-flight jobs to exit.
	s.leases.beginDraining()
	s.cancelAllAgentJobs()
	s.cancelAllExtractionJobs()
	// Wait for cancelled jobs to release their leases (bounded by drainTimeout).
	// If the timeout fires, remaining leases are marked stale and their next
	// persistence attempt will transition to Recoverable instead of writing.
	_ = s.leases.drainAndBlock(context.Background(), s.drainTimeout)
	s.mu.Lock()
	previous := s.active
	s.storeGeneration++
	next.generation = s.storeGeneration
	s.active = &next
	s.branchMergePreviews = make(map[string]branchMergePreview)
	s.mu.Unlock()
	s.leases.endDraining()
	if previous != nil {
		_ = previous.store.Close()
	}
}

// currentGeneration returns the store generation of the active workspace, or 0
// when no project is active. Jobs compare their captured generation against
// this value before persisting; a mismatch means the store they captured has
// been replaced and writes must be skipped.
func (s *Service) currentGeneration() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return 0
	}
	return s.active.generation
}
