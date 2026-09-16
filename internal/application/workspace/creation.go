package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type creationKernel struct {
	inner *agentkernel.Service
	store agentkernel.Store
}

func (kernel creationKernel) CreateRun(ctx context.Context, request agentkernel.CreateRequest) (agentdomain.Run, error) {
	return kernel.inner.CreateRun(ctx, request)
}

func (kernel creationKernel) ExecuteRun(ctx context.Context, runID, canonicalHead string) (agentdomain.Run, error) {
	return kernel.inner.ExecuteRun(ctx, runID, canonicalHead)
}

func (kernel creationKernel) Artifacts(ctx context.Context, runID string) ([]agentdomain.Artifact, error) {
	return kernel.store.ListAgentArtifacts(ctx, runID)
}

// Run 读取持久化运行，供创作恢复对账使用。
func (kernel creationKernel) Run(ctx context.Context, runID string) (agentdomain.Run, error) {
	return kernel.store.AgentRun(ctx, runID)
}

// creationRunLease 把工作区作业租约适配为创作运行租约。
type creationRunLease struct {
	ctx   context.Context
	lease *jobLease
}

type creationLeaseKey struct{}

// creationCoordinator 固定项目代次，旧页面不能在切换后的新项目取得运行租约。
type creationCoordinator struct {
	workspace  *Service
	generation uint64
}

func (coordinator creationCoordinator) BeginRun(ctx context.Context, runID string) (creationapp.RunLease, func(), error) {
	return coordinator.workspace.beginCreationRun(ctx, runID, coordinator.generation)
}

func (coordinator creationCoordinator) Cancel(runID string) { coordinator.workspace.Cancel(runID) }
func (coordinator creationCoordinator) Live(runID string) bool {
	return coordinator.workspace.Live(runID)
}

func (lease creationRunLease) Context() context.Context { return lease.ctx }

// Stale 表示排水超时已把租约标记为过期，创作运行不得再向捕获的 store 写入。
func (lease creationRunLease) Stale() bool { return lease.lease.IsStale() }

// BeginRun 实现 creation.RunCoordinator：创作运行与 Agent、抽取作业共用
// 项目作业租约注册表；项目关闭/切换进入排水时会取消并等待创作运行释放。
func (s *Service) BeginRun(parent context.Context, runID string) (creationapp.RunLease, func(), error) {
	return s.beginCreationRun(parent, runID, s.currentGeneration())
}

// beginCreationRun 在短工作区锁内检查代次并注册租约，不持锁执行模型调用。
func (s *Service) beginCreationRun(parent context.Context, runID string, generation uint64) (creationapp.RunLease, func(), error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireEditableLocked(); err != nil {
		return nil, nil, err
	}
	if s.active.generation != generation {
		return nil, nil, creationapp.ErrStaleStore
	}
	s.creationJobsMu.Lock()
	if _, exists := s.creationJobs[runID]; exists {
		s.creationJobsMu.Unlock()
		return nil, nil, fmt.Errorf("%w: %s", creationapp.ErrRunActive, runID)
	}
	jobCtx, cancel := context.WithCancel(parent)
	s.creationJobs[runID] = cancel
	s.creationJobsMu.Unlock()
	lease, err := s.leases.acquire(runID, generation)
	if err != nil {
		s.endCreationRun(runID)
		return nil, nil, err
	}
	jobCtx = context.WithValue(jobCtx, creationLeaseKey{}, lease)
	return creationRunLease{ctx: jobCtx, lease: lease}, func() { s.endCreationRun(runID) }, nil
}

// endCreationRun 注销取消函数并释放作业租约；租约在终态持久化完成后才释放。
func (s *Service) endCreationRun(runID string) {
	s.creationJobsMu.Lock()
	cancel := s.creationJobs[runID]
	delete(s.creationJobs, runID)
	s.creationJobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.leases.release(runID)
}

// Cancel 取消一个进行中的创作运行。
func (s *Service) Cancel(runID string) {
	s.creationJobsMu.Lock()
	cancel := s.creationJobs[runID]
	s.creationJobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Live 表示运行是否仍在本进程内活跃执行。
func (s *Service) Live(runID string) bool {
	s.creationJobsMu.Lock()
	defer s.creationJobsMu.Unlock()
	_, ok := s.creationJobs[runID]
	return ok
}

// cancelAllCreationJobs 在项目关闭/切换时取消全部进行中的创作运行。
func (s *Service) cancelAllCreationJobs() {
	s.creationJobsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.creationJobs))
	for _, cancel := range s.creationJobs {
		cancels = append(cancels, cancel)
	}
	s.creationJobsMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Service) CreationWorkbench(ctx context.Context) (creationapp.WorkbenchView, error) {
	return s.creationWorkbench(ctx, false)
}

// creationWorkbench 仅在可写原创入口初始化；只读与轮询不执行恢复写入。
func (s *Service) creationWorkbench(ctx context.Context, recoverRun bool) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	view, err := runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	readOnly := s.requireEditable() != nil
	if view.SessionID == "" && !readOnly {
		s.mu.RLock()
		if s.active == nil {
			s.mu.RUnlock()
			return creationapp.WorkbenchView{}, ErrNoActiveProject
		}
		chapters, readErr := s.active.store.ListChapters(ctx)
		s.mu.RUnlock()
		if readErr != nil {
			return creationapp.WorkbenchView{}, readErr
		}
		if len(chapters) == 0 {
			if _, err := runtime.service.EnsureSession(ctx, runtime.projectID, runtime.branchID); err != nil {
				return creationapp.WorkbenchView{}, err
			}
			view, err = runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
		} else {
			view.Applicability = "not_applicable"
		}
	}
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	var revisions []creationapp.RevisionView
	var directionActions []creationapp.DirectionActionView
	if view.SessionID != "" {
		revisions, err = runtime.service.RevisionActions(ctx, runtime.projectID, recoverRun && !readOnly)
		if err != nil {
			return creationapp.WorkbenchView{}, err
		}
		directionActions, err = runtime.service.DirectionActions(ctx, runtime.projectID, recoverRun && !readOnly)
		if err != nil {
			return creationapp.WorkbenchView{}, err
		}
	}
	if recoverRun && !readOnly && view.SessionID != "" {
		if _, err := runtime.service.Recover(ctx, runtime.projectID); err != nil {
			return creationapp.WorkbenchView{}, err
		}
		view, err = runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
	}
	view.Revisions = revisions
	view.DirectionActions = directionActions
	view.ReadOnly = readOnly
	if readOnly {
		view.Capabilities = []string{}
	}
	return view, err
}

func (s *Service) SaveCreationIntent(ctx context.Context, text string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveIntent(ctx, runtime.projectID, text); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SetCreationAdvanceMode(ctx context.Context, mode string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SetAdvanceMode(ctx, runtime.projectID, creationdomain.AdvanceMode(mode)); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// SelectCreationDirection 记录作者选中的整套方向候选与其中一张卡。
func (s *Service) SelectCreationDirection(ctx context.Context, candidateID, cardID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SelectDirection(ctx, runtime.projectID, candidateID, cardID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// KeepCreationDirection 持久化保留一张方向卡。
func (s *Service) KeepCreationDirection(ctx context.Context, cardID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.KeepDirection(ctx, runtime.projectID, cardID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// UnkeepCreationDirection 取消保留一张方向卡。
func (s *Service) UnkeepCreationDirection(ctx context.Context, cardID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.UnkeepDirection(ctx, runtime.projectID, cardID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) AdvanceCreationStage(ctx context.Context) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.AdvanceStage(ctx, runtime.projectID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// AcceptCreationCandidate 接受当前阶段的精确候选，不隐式推进或写入正式档案。
func (s *Service) AcceptCreationCandidate(ctx context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.AcceptCandidate(ctx, runtime.projectID, candidateID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) LookBackCreationStage(ctx context.Context, stage string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.LookBackStage(ctx, runtime.projectID, creationdomain.Stage(stage)); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SaveCreationCandidateDraft(ctx context.Context, candidateID, payloadJSON string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveCandidateDraft(ctx, runtime.projectID, candidateID, payloadJSON); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) LockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.LockCandidateField(ctx, runtime.projectID, candidateID, path); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) UnlockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.UnlockCandidateField(ctx, runtime.projectID, candidateID, path); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) GetCreationCandidate(ctx context.Context, candidateID string) (creationapp.CandidateView, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.CandidateView{}, err
	}
	candidate, err := runtime.service.CandidatePayload(ctx, runtime.projectID, candidateID)
	if err != nil {
		return creationapp.CandidateView{}, err
	}
	locks := candidate.LockMap
	if locks == nil {
		locks = map[string]bool{}
	}
	return creationapp.CandidateView{
		ID: candidate.ID, Stage: candidate.Stage, Kind: string(candidate.Kind), Version: candidate.Version,
		Status: string(candidate.Status), PayloadJSON: candidate.PayloadJSON, LockMap: locks,
	}, nil
}

func (s *Service) PreviewCreationGenerate(ctx context.Context, stage string) (creationapp.CostPreview, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.CostPreview{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.CostPreview{}, err
	}
	return runtime.service.PreviewGenerate(ctx, creationapp.GenerateRequest{
		ProjectID: runtime.projectID, BranchID: runtime.branchID, ChapterID: "planning",
		CanonicalHead: "planning", Stage: creationdomain.Stage(stage), Profile: profile,
	})
}

func (s *Service) ConfirmCreationGenerate(ctx context.Context, stage, digest, idempotencyKey string) (creationapp.GenerateResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	chapterID, head, err := s.ensurePlanningChapter(ctx)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	return runtime.service.ConfirmGenerate(ctx, creationapp.GenerateRequest{
		ProjectID: runtime.projectID, BranchID: runtime.branchID, ChapterID: chapterID,
		CanonicalHead: head, Stage: creationdomain.Stage(stage), Profile: profile,
		ConfirmationDigest: digest, IdempotencyKey: idempotencyKey,
	})
}

func (s *Service) CancelCreationRun(ctx context.Context) error {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return err
	}
	return runtime.service.CancelRun(ctx, runtime.projectID)
}

// RecoverCreation 恢复创作工作台：新项目首次进入先确保会话存在再恢复，
// 不以 no rows 当业务失败（P1-1）；只读项目不隐式创建会话，直接读取既有工作台。
func (s *Service) RecoverCreation(ctx context.Context) (creationapp.WorkbenchView, error) {
	return s.creationWorkbench(ctx, true)
}

func (s *Service) PreviewCreationArchive(ctx context.Context) (creationapp.ArchivePreview, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.ArchivePreview{}, err
	}
	return runtime.service.PreviewConfirmArchive(ctx, runtime.projectID)
}

func (s *Service) ConfirmCreationArchive(ctx context.Context, digest, idempotencyKey string) (creationapp.ArchiveResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.ArchiveResult{}, err
	}
	return runtime.service.ConfirmArchive(ctx, creationapp.ArchiveRequest{
		ProjectID: runtime.projectID, ConfirmationDigest: digest, IdempotencyKey: idempotencyKey,
	})
}

func (s *Service) ResolveCreationImpact(ctx context.Context, choice string) (creationapp.ResolveImpactResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.ResolveImpactResult{}, err
	}
	return runtime.service.ResolveImpact(ctx, creationapp.ResolveImpactRequest{
		ProjectID: runtime.projectID, Choice: creationdomain.ImpactChoice(choice),
	})
}

type creationRuntime struct {
	service   *creationapp.Service
	projectID string
	branchID  string
}

func (s *Service) creationRuntime(ctx context.Context, write bool) (creationRuntime, error) {
	if err := ctx.Err(); err != nil {
		return creationRuntime{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return creationRuntime{}, ErrNoActiveProject
	}
	if write {
		if err := s.requireEditableLocked(); err != nil {
			return creationRuntime{}, err
		}
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return creationRuntime{}, err
	}
	backend, ok := s.active.store.(creationapp.SQLBackend)
	if !ok {
		return creationRuntime{}, ErrCreationUnavailable
	}
	kernelStore, ok := s.active.store.(agentkernel.Store)
	if !ok {
		return creationRuntime{}, ErrCreationUnavailable
	}
	if s.active.creation == nil {
		registry, regErr := agentassets.Builtin()
		if regErr != nil {
			return creationRuntime{}, fmt.Errorf("%w: %v", ErrCreationUnavailable, regErr)
		}
		onUpdated := func(context.Context, string) {}
		if events, ok := s.active.store.(ports.JobEventStore); ok {
			onUpdated = func(callback context.Context, runID string) {
				s.notifyJobUpdated(callback, events, runID, agentdomain.JobKindAgent)
			}
		}
		guardedKernelStore := creationKernelStore{Store: kernelStore, workspace: s, generation: s.active.generation}
		kernel := agentkernel.NewService(guardedKernelStore, s.credentials, s.providers, agentkernel.Registries{
			Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
		}, s.clock, s.ids, onUpdated)
		svc, svcErr := creationapp.New(creationapp.Config{
			Store:  creationStore{inner: creationapp.NewSQLiteStore(backend), guard: guardedKernelStore},
			Kernel: creationKernel{inner: kernel, store: guardedKernelStore},
			Clock:  s.clock, IDs: s.ids,
			Coordinator: creationCoordinator{workspace: s, generation: s.active.generation},
			Generation: func() uint64 {
				s.mu.RLock()
				defer s.mu.RUnlock()
				if s.active == nil {
					return 0
				}
				return s.active.generation
			},
		})
		if svcErr != nil {
			return creationRuntime{}, svcErr
		}
		s.active.creation = svc
	}
	branchID := "main"
	return creationRuntime{service: s.active.creation, projectID: project.ID, branchID: branchID}, nil
}

func (s *Service) creationProfile(ctx context.Context) (agentdomain.ResolvedCreativeProfile, error) {
	s.mu.RLock()
	if s.active == nil {
		s.mu.RUnlock()
		return agentdomain.ResolvedCreativeProfile{}, ErrNoActiveProject
	}
	store := s.active.store
	s.mu.RUnlock()
	if s.profileResolver == nil {
		return agentdomain.ResolvedCreativeProfile{}, ErrAgentRuntimeUnavailable
	}
	selection, found, err := readProfileSelection(ctx, store)
	if err != nil {
		return agentdomain.ResolvedCreativeProfile{}, err
	}
	if !found || strings.TrimSpace(selection.ProfileID) == "" {
		return agentdomain.ResolvedCreativeProfile{}, ErrProjectProfileNotConfigured
	}
	return s.profileResolver.ResolveCreativeProfile(ctx, selection.ProfileID, selection.Version)
}

func (s *Service) ensurePlanningChapter(ctx context.Context) (string, string, error) {
	s.mu.RLock()
	if s.active == nil {
		s.mu.RUnlock()
		return "", "", ErrNoActiveProject
	}
	store := s.active.store
	s.mu.RUnlock()
	chapters, err := store.ListChapters(ctx)
	if err != nil {
		return "", "", err
	}
	if len(chapters) == 0 {
		created, createErr := s.CreateChapter(ctx, "作品规划")
		if createErr != nil {
			return "", "", createErr
		}
		return created.ID, created.ID, nil
	}
	return chapters[0].ID, chapters[0].ID, nil
}
