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

	mu                  sync.RWMutex
	active              *activeWorkspace
	branchMergePreviews map[string]branchMergePreview

	agentJobsMu      sync.Mutex
	agentJobs        map[string]context.CancelFunc
	versionOperation bool
	importOperation  bool
}

type activeWorkspace struct {
	packageInfo ports.ProjectPackage
	store       ports.ProjectStore
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
	if agentStore, ok := store.(ports.AgentStore); ok {
		if _, err := agentStore.RecoverInterruptedAgentRuns(ctx, s.clock.Now()); err != nil {
			_ = store.Close()
			return ProjectView{}, fmt.Errorf("recover interrupted agent runs: %w", err)
		}
	}
	if err := s.ensureBranchCatalog(ctx, &candidate); err != nil {
		_ = store.Close()
		return ProjectView{}, err
	}
	s.replaceActive(candidate)
	return projectView(project, packageInfo.Root), nil
}

func (s *Service) OpenProject(ctx context.Context, packagePath string) (ProjectView, error) {
	packageInfo, err := s.packages.Open(packagePath)
	if err != nil {
		return ProjectView{}, fmt.Errorf("open project package: %w", err)
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
	if agentStore, ok := store.(ports.AgentStore); ok {
		if _, err := agentStore.RecoverInterruptedAgentRuns(ctx, s.clock.Now()); err != nil {
			_ = store.Close()
			return ProjectView{}, fmt.Errorf("recover interrupted agent runs: %w", err)
		}
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
	s.cancelAllAgentJobs()
	s.mu.Lock()
	active := s.active
	s.active = nil
	s.branchMergePreviews = make(map[string]branchMergePreview)
	s.mu.Unlock()
	if active == nil {
		return nil
	}
	return active.store.Close()
}

func (s *Service) replaceActive(next activeWorkspace) {
	s.cancelAllAgentJobs()
	s.mu.Lock()
	previous := s.active
	s.active = &next
	s.branchMergePreviews = make(map[string]branchMergePreview)
	s.mu.Unlock()
	if previous != nil {
		_ = previous.store.Close()
	}
}
