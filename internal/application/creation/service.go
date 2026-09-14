package creation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	"github.com/lishen802/novel-studio/internal/quality"
)

var (
	ErrConfirmationRequired = errors.New("creation cost confirmation required")
	ErrRunActive            = errors.New("creation run is already active")
	ErrInvalidStage         = errors.New("invalid creation generate stage")
	ErrUnavailable          = errors.New("creation service unavailable")
	ErrDirectionRequired    = errors.New("creation direction selection is incomplete")
	ErrOperationFailed      = errors.New("creation operation failed")
)

// Kernel 是创作调度依赖的内核端口；Run 用于投影与恢复对账读取持久化运行。
type Kernel interface {
	CreateRun(context.Context, agentkernel.CreateRequest) (agentdomain.Run, error)
	ExecuteRun(context.Context, string, string) (agentdomain.Run, error)
	Artifacts(context.Context, string) ([]agentdomain.Artifact, error)
	Run(ctx context.Context, runID string) (agentdomain.Run, error)
}

// RunLease 表示创作运行持有的作业租约与可取消上下文。
type RunLease interface {
	Context() context.Context
	// Stale 表示排水超时已把租约标记为过期，禁止再向捕获的 store 写入。
	Stale() bool
}

// RunCoordinator 由工作区实现：创作运行与 Agent、抽取作业共用项目租约、排水与取消注册，
// 项目关闭/切换时会先取消创作运行并等待其 lease 释放，避免向已关闭的 store 写入。
type RunCoordinator interface {
	BeginRun(ctx context.Context, runID string) (RunLease, func(), error)
	Cancel(runID string)
	Live(runID string) bool
}

// memoryRunLease 是测试与独立服务使用的进程内租约；过期性由 store generation 检查承担。
type memoryRunLease struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (lease *memoryRunLease) Context() context.Context { return lease.ctx }
func (lease *memoryRunLease) Stale() bool              { return false }

// memoryCoordinator 是默认的进程内运行协调器。
type memoryCoordinator struct {
	mu    sync.Mutex
	runs  map[string]*memoryRunLease
	order []string
}

func newMemoryCoordinator() *memoryCoordinator {
	return &memoryCoordinator{runs: map[string]*memoryRunLease{}}
}

func (coordinator *memoryCoordinator) BeginRun(ctx context.Context, runID string) (RunLease, func(), error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if _, exists := coordinator.runs[runID]; exists {
		return nil, nil, fmt.Errorf("%w: %s", ErrRunActive, runID)
	}
	jobCtx, cancel := context.WithCancel(ctx)
	lease := &memoryRunLease{ctx: jobCtx, cancel: cancel}
	coordinator.runs[runID] = lease
	coordinator.order = append(coordinator.order, runID)
	release := func() {
		coordinator.mu.Lock()
		defer coordinator.mu.Unlock()
		if _, ok := coordinator.runs[runID]; ok {
			delete(coordinator.runs, runID)
			cancel()
		}
	}
	return lease, release, nil
}

func (coordinator *memoryCoordinator) Cancel(runID string) {
	coordinator.mu.Lock()
	lease := coordinator.runs[runID]
	coordinator.mu.Unlock()
	if lease != nil {
		lease.cancel()
	}
}

func (coordinator *memoryCoordinator) Live(runID string) bool {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	_, ok := coordinator.runs[runID]
	return ok
}

type GenerateRequest struct {
	ProjectID          string
	BranchID           string
	ChapterID          string
	CanonicalHead      string
	Stage              creationdomain.Stage
	Profile            agentdomain.ResolvedCreativeProfile
	ConfirmationDigest string
	IdempotencyKey     string
}

// RoleModelView 描述费用预览中该角色将使用的模型来源。
type RoleModelView struct {
	Role     string `json:"role"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type CostPreview struct {
	Stage              creationdomain.Stage   `json:"stage"`
	WorkflowID         string                 `json:"workflowId"`
	WorkflowVersion    int                    `json:"workflowVersion"`
	EstimatedCalls     int                    `json:"estimatedCalls"`
	MaxRunTokens       int                    `json:"maxRunTokens"`
	RunBudgetUnlimited bool                   `json:"runBudgetUnlimited"`
	RemainingStages    []creationdomain.Stage `json:"remainingStages"`
	ConfirmationDigest string                 `json:"confirmationDigest"`
	RoleModels         []RoleModelView        `json:"roleModels"`
}

type GenerateResult struct {
	Session   creationdomain.Session   `json:"session"`
	Candidate creationdomain.Candidate `json:"candidate"`
	RunID     string                   `json:"runId"`
}

type Config struct {
	Store       Store
	Kernel      Kernel
	Clock       ports.Clock
	IDs         ports.IDGenerator
	Generation  func() uint64
	Coordinator RunCoordinator
}

type Service struct {
	store        Store
	kernel       Kernel
	clock        ports.Clock
	ids          ports.IDGenerator
	generation   func() uint64
	registry     *agentassets.Registry
	coordinator  RunCoordinator
	projectLocks sync.Map
}

func New(config Config) (*Service, error) {
	if config.Store == nil || config.Kernel == nil || config.Clock == nil || config.IDs == nil {
		return nil, ErrUnavailable
	}
	registry, err := agentassets.Builtin()
	if err != nil {
		return nil, err
	}
	generation := config.Generation
	if generation == nil {
		generation = func() uint64 { return 1 }
	}
	coordinator := config.Coordinator
	if coordinator == nil {
		coordinator = newMemoryCoordinator()
	}
	return &Service{
		store: config.Store, kernel: config.Kernel, clock: config.Clock, ids: config.IDs,
		generation: generation, registry: registry, coordinator: coordinator,
	}, nil
}

// lockProject 返回项目级互斥锁，保证运行槽检查与占用的原子性。
func (service *Service) lockProject(projectID string) *sync.Mutex {
	value, _ := service.projectLocks.LoadOrStore(projectID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// EnsureSession 为项目取得或新建原创构建会话。
func (service *Service) EnsureSession(ctx context.Context, projectID, branchID string) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err == nil {
		if session.BranchID != branchID {
			return creationdomain.Session{}, creationdomain.ErrSessionVersionConflict
		}
		return session, nil
	}
	if !isMissingSession(err) {
		return creationdomain.Session{}, err
	}
	session, err = creationdomain.NewSession(service.ids.NewID(), projectID, branchID, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	return service.store.CreateSession(ctx, session)
}

// saveSession 按读取时内容版本写入；冲突交还调用方，不自动重放旧作者命令。
func (service *Service) saveSession(ctx context.Context, session *creationdomain.Session) error {
	if err := service.store.SaveSession(ctx, *session); err != nil {
		return err
	}
	session.ContentVersion++
	return nil
}

// SaveIntent 保存作者创意文本，不创建运行。
func (service *Service) SaveIntent(ctx context.Context, projectID, text string) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SaveIntent(text, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.refreshImpact(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// SetAdvanceMode 切换逐步审阅或自动推进，不触发付费调用。
func (service *Service) SetAdvanceMode(ctx context.Context, projectID string, mode creationdomain.AdvanceMode) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SetAdvanceMode(mode, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// SelectDirection 记录作者选中的方向候选与其中一张卡，后端校验卡属于该版本。
func (service *Service) SelectDirection(ctx context.Context, projectID, candidateID, cardID string) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	candidate, err := service.store.Candidate(ctx, candidateID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if candidate.SessionID != session.ID {
		return creationdomain.Session{}, ErrSessionNotFound
	}
	if _, err := directionCardByID(candidate, cardID); err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SelectDirection(candidateID, cardID, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.refreshImpact(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// KeepDirection 持久化保留一张方向卡，重启后可恢复。
func (service *Service) KeepDirection(ctx context.Context, projectID, cardID string) (creationdomain.Session, error) {
	return service.updateKeptDirection(ctx, projectID, cardID, true)
}

// UnkeepDirection 取消保留一张方向卡。
func (service *Service) UnkeepDirection(ctx context.Context, projectID, cardID string) (creationdomain.Session, error) {
	return service.updateKeptDirection(ctx, projectID, cardID, false)
}

func (service *Service) updateKeptDirection(ctx context.Context, projectID, cardID string, keep bool) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if keep {
		session, err = session.KeepDirection(cardID, service.clock.Now())
	} else {
		session, err = session.UnkeepDirection(cardID, service.clock.Now())
	}
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// AdvanceStage 在当前阶段完成条件满足时进入下一阶段，不写类型表。
func (service *Service) AdvanceStage(ctx context.Context, projectID string) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if session.EntryKind() == creationdomain.EntryAuthorPlan && session.Stage == creationdomain.StageIntent {
		if _, _, err := validatedMaterial(session, candidates); err != nil {
			return creationdomain.Session{}, err
		}
		session.Stage = creationdomain.StageProposal
		if err := service.saveSession(ctx, &session); err != nil {
			return creationdomain.Session{}, err
		}
		return session, nil
	}
	if err := stageReady(session, candidates); err != nil {
		return creationdomain.Session{}, err
	}
	if stageRequiresCandidateAcceptance(session.Stage) {
		current, ok := currentCandidate(candidates, session.Stage)
		if !ok || current.Status != creationdomain.StatusConfirmed {
			return creationdomain.Session{}, creationdomain.ErrCandidateAcceptanceRequired
		}
	}
	session, err = session.Advance(service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// AcceptCandidate 明确接受指定阶段的当前精确候选版本，但不推进阶段、不写正式档案。
func (service *Service) AcceptCandidate(ctx context.Context, projectID, candidateID string) (creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	target, ok := candidateByID(candidates, strings.TrimSpace(candidateID))
	if !ok || !stageRequiresCandidateAcceptance(target.Stage) || !stageReached(session.Stage, target.Stage) {
		return creationdomain.Candidate{}, creationdomain.ErrCandidateAcceptanceRequired
	}
	current, ok := currentCandidate(candidates, target.Stage)
	if !ok || current.ID != target.ID {
		return creationdomain.Candidate{}, creationdomain.ErrCandidateAcceptanceRequired
	}
	if current.Status == creationdomain.StatusConfirmed {
		return current, nil
	}
	if err := validateCandidateSchema(current.Stage, current.PayloadJSON, current.PayloadSchemaVersion); err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := validateCandidateDependency(session, candidates, current); err != nil {
		return creationdomain.Candidate{}, err
	}
	expected := current
	// 软影响例外已经绑定当前上游摘要时，接受动作保留例外证据并结束 stale 状态。
	if current.Status == creationdomain.StatusStale && current.ExceptionMatches(liveUpstreamDigest(session, candidates, current.Stage)) {
		current.Status = creationdomain.StatusDraft
	}
	accepted, err := current.Confirm(service.clock.Now())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.store.ApplyCandidateChanges(ctx,
		[]creationdomain.Candidate{expected}, []creationdomain.Candidate{accepted}, nil,
	); err != nil {
		return creationdomain.Candidate{}, err
	}
	return accepted, nil
}

func stageRequiresCandidateAcceptance(stage creationdomain.Stage) bool {
	return stage == creationdomain.StageProposal || stage == creationdomain.StagePeopleWorld || stage == creationdomain.StageRoadmap
}

func stageReached(current, target creationdomain.Stage) bool {
	currentIndex, targetIndex := -1, -1
	for index, stage := range creationdomain.Rail() {
		if stage == current {
			currentIndex = index
		}
		if stage == target {
			targetIndex = index
		}
	}
	return currentIndex >= 0 && targetIndex >= 0 && targetIndex <= currentIndex
}

// LookBackStage 回到更早阶段，不删除候选、不写正式资产。
func (service *Service) LookBackStage(ctx context.Context, projectID string, target creationdomain.Stage) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if session.EntryKind() == creationdomain.EntryAuthorPlan && target == creationdomain.StageDirections {
		return creationdomain.Session{}, ErrInvalidStage
	}
	session, err = session.LookBack(target, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.saveSession(ctx, &session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// LockCandidateField 锁定候选字段，重新生成时必须保持。
func (service *Service) LockCandidateField(ctx context.Context, projectID, candidateID, path string) (creationdomain.Candidate, error) {
	return service.updateCandidateLock(ctx, projectID, candidateID, path, true)
}

// UnlockCandidateField 解除字段锁，之后作者草稿才能改该路径。
func (service *Service) UnlockCandidateField(ctx context.Context, projectID, candidateID, path string) (creationdomain.Candidate, error) {
	return service.updateCandidateLock(ctx, projectID, candidateID, path, false)
}

func (service *Service) updateCandidateLock(ctx context.Context, projectID, candidateID, path string, lock bool) (creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	candidate, err := service.store.Candidate(ctx, candidateID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if candidate.SessionID != session.ID {
		return creationdomain.Candidate{}, ErrSessionNotFound
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	current, ok := currentCandidate(candidates, candidate.Stage)
	if !ok || current.ID != candidate.ID {
		return creationdomain.Candidate{}, creationdomain.ErrCandidateVersionConflict
	}

	expected := candidate
	if lock {
		candidate, err = candidate.LockField(path, service.clock.Now())
	} else {
		candidate, err = candidate.UnlockField(path, service.clock.Now())
	}
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.store.ApplyCandidateChanges(ctx,
		[]creationdomain.Candidate{expected}, []creationdomain.Candidate{candidate}, nil,
	); err != nil {
		return creationdomain.Candidate{}, err
	}
	return candidate, nil
}

// CandidatePayload 有界读取单个候选的完整 payload，供中央区编辑。
func (service *Service) CandidatePayload(ctx context.Context, projectID, candidateID string) (creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	candidate, err := service.store.Candidate(ctx, candidateID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if candidate.SessionID != session.ID {
		return creationdomain.Candidate{}, ErrSessionNotFound
	}
	return candidate, nil
}

// PreviewGenerate 冻结费用预览摘要，不创建运行、不写候选。
func (service *Service) PreviewGenerate(ctx context.Context, request GenerateRequest) (CostPreview, error) {
	session, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return CostPreview{}, err
	}
	stage, err := resolveGenerateStage(session, request.Stage)
	if err != nil {
		return CostPreview{}, err
	}
	return service.preview(ctx, session, request.Profile, stage)
}

// ConfirmGenerate 校验预算摘要后创建内核运行并立即返回运行引用；
// 执行在后台按项目租约进行，结果通过持久化操作回执与工作台投影收敛。
func (service *Service) ConfirmGenerate(ctx context.Context, request GenerateRequest) (GenerateResult, error) {
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return GenerateResult{}, creationdomain.ErrBlankValue
	}
	// 先按原始请求核对回执，不能用已经变化的实时输入重新定义同一操作。
	arguments, err := json.Marshal(struct {
		Stage                                  creationdomain.Stage
		Digest, Project, Branch, Chapter, Head string
		Profile                                agentdomain.ResolvedCreativeProfile
	}{request.Stage, request.ConfirmationDigest, request.ProjectID, request.BranchID, request.ChapterID, request.CanonicalHead, request.Profile})
	if err != nil {
		return GenerateResult{}, err
	}
	argumentsHash := hashParts(string(arguments))
	command := generateCommand(request.Stage)
	if existing, lookupErr := service.store.OperationByIdempotency(ctx, request.ProjectID, request.BranchID, command, request.IdempotencyKey); lookupErr == nil {
		return service.replayOperation(ctx, existing, argumentsHash)
	} else if !errors.Is(lookupErr, ErrOperationNotFound) {
		return GenerateResult{}, lookupErr
	}
	session, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return GenerateResult{}, err
	}
	stage, err := resolveGenerateStage(session, request.Stage)
	if err != nil {
		return GenerateResult{}, err
	}
	preview, err := service.preview(ctx, session, request.Profile, stage)
	if err != nil {
		return GenerateResult{}, err
	}
	if strings.TrimSpace(request.ConfirmationDigest) == "" || request.ConfirmationDigest != preview.ConfirmationDigest {
		return GenerateResult{}, ErrConfirmationRequired
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return GenerateResult{}, creationdomain.ErrBlankValue
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return GenerateResult{}, err
	}
	sources, err := generateSources(session, candidates, stage)
	if err != nil {
		return GenerateResult{}, err
	}
	frozenDigest := liveUpstreamDigest(session, candidates, stage)

	unlock := service.lockProject(request.ProjectID)
	unlock.Lock()
	defer unlock.Unlock()
	latest, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return GenerateResult{}, err
	}
	if strings.TrimSpace(latest.ActiveRunID) != "" {
		return GenerateResult{}, ErrRunActive
	}
	latestPreview, err := service.preview(ctx, latest, request.Profile, stage)
	if err != nil {
		return GenerateResult{}, err
	}
	if latestPreview.ConfirmationDigest != request.ConfirmationDigest {
		return GenerateResult{}, ErrConfirmationRequired
	}
	if latest.Stage == creationdomain.StageIntent && stage == creationdomain.StageDirections {
		latest, err = latest.Advance(service.clock.Now())
		if err != nil {
			return GenerateResult{}, err
		}
		if err := service.saveSession(ctx, &latest); err != nil {
			return GenerateResult{}, err
		}
	}
	now := service.clock.Now()
	reserved, created, err := service.store.ReserveOperation(ctx, creationdomain.Operation{
		ID: service.ids.NewID(), ProjectID: request.ProjectID, BranchID: request.BranchID,
		Command: command, IdempotencyKey: request.IdempotencyKey,
		ArgumentsHash: argumentsHash, FrozenInputDigest: frozenDigest, Status: creationdomain.OperationReserved,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return GenerateResult{}, err
	}
	if !created {
		return service.replayOperation(ctx, reserved, argumentsHash)
	}
	runID := service.ids.NewID()
	captured := service.generation()
	lease, release, err := service.coordinator.BeginRun(context.WithoutCancel(ctx), runID)
	if err != nil {
		return GenerateResult{}, err
	}
	launched := false
	defer func() {
		if !launched {
			release()
		}
	}()
	if lease.Stale() || service.generation() != captured {
		return GenerateResult{}, ErrStaleStore
	}
	run, err := service.startStageRun(lease.Context(), runID, reserved.ID, latest, request, stage, sources)
	if err != nil {
		service.finishOperation(context.WithoutCancel(lease.Context()), reserved, creationdomain.OperationFailed, "", operationErrorCode(err))
		return GenerateResult{}, err
	}
	running := reserved
	running.Status = creationdomain.OperationRunning
	running.RunID = run.ID
	running.UpdatedAt = service.clock.Now()
	if err := service.store.UpdateOperation(lease.Context(), running); err != nil {
		service.clearRunSlot(context.WithoutCancel(lease.Context()), request.ProjectID, run.ID)
		return GenerateResult{}, err
	}
	service.launchRun(running, latest, request, stage, frozenDigest, captured, lease, release)
	launched = true
	return GenerateResult{Session: latest, RunID: run.ID}, nil
}

// replayOperation 处理同键重复请求：相同参数返回原结果，不同参数返回冲突。
func (service *Service) replayOperation(
	ctx context.Context, operation creationdomain.Operation, argumentsHash string,
) (GenerateResult, error) {
	if operation.ArgumentsHash != argumentsHash {
		return GenerateResult{}, ErrOperationConflict
	}
	switch operation.Status {
	case creationdomain.OperationSucceeded:
		session, err := service.loadSession(ctx, operation.ProjectID)
		if err != nil {
			return GenerateResult{}, err
		}
		result := GenerateResult{Session: session, RunID: operation.RunID}
		if strings.TrimSpace(operation.CandidateID) != "" {
			if candidate, loadErr := service.store.Candidate(ctx, operation.CandidateID); loadErr == nil {
				result.Candidate = candidate
			}
		}
		return result, nil
	case creationdomain.OperationFailed:
		return GenerateResult{}, fmt.Errorf("%w: %s", ErrOperationFailed, operation.ErrorCode)
	default:
		session, err := service.loadSession(ctx, operation.ProjectID)
		return GenerateResult{Session: session, RunID: operation.RunID}, err
	}
}

// launchRun 在后台执行已创建的运行；请求上下文不等待模型完成。
func (service *Service) launchRun(
	operation creationdomain.Operation,
	session creationdomain.Session,
	request GenerateRequest,
	stage creationdomain.Stage,
	frozenDigest string,
	captured uint64,
	lease RunLease,
	release func(),
) {
	go func() {
		defer release()
		result, execErr := service.executeStageRun(lease, session, request, stage, operation.RunID, frozenDigest, captured)
		if execErr == nil && session.AdvanceMode == creationdomain.ModeAutoAdvance && stage != creationdomain.StageIntent {
			result, execErr = service.autoAdvance(lease, result, request, captured)
		}
		if lease.Stale() || service.generation() != captured {
			return
		}
		storeCtx := context.WithoutCancel(lease.Context())
		latest, readErr := service.loadSession(storeCtx, session.ProjectID)
		if readErr != nil {
			return
		}
		if latest.ActiveOperationID == operation.ID {
			// 自动推进可能已把同一操作回执切换到后续子运行；重读以免完成回调覆盖其冻结摘要。
			if current, operationErr := service.store.Operation(storeCtx, operation.ID); operationErr == nil {
				operation = current
			}
			operation.RunID = latest.ActiveRunID
		}
		if execErr != nil {
			service.finishOperation(storeCtx, operation, creationdomain.OperationFailed, "", operationErrorCode(execErr))
			service.clearRunSlot(storeCtx, session.ProjectID, operation.RunID)
			return
		}
		service.finishOperation(storeCtx, operation, creationdomain.OperationSucceeded, result.Candidate.ID, "")
		service.clearRunSlot(storeCtx, session.ProjectID, operation.RunID)
	}()
}

// finishOperation 更新操作回执终态；失败不掩盖原始错误。
func (service *Service) finishOperation(ctx context.Context, operation creationdomain.Operation, status creationdomain.OperationStatus, candidateID, errorCode string) {
	operation.Status = status
	operation.CandidateID = candidateID
	operation.ErrorCode = errorCode
	operation.UpdatedAt = service.clock.Now()
	_ = service.store.UpdateOperation(ctx, operation)
}

// startStageRun 创建内核运行并占用会话运行槽；调用方保证项目锁内槽位空闲。
func (service *Service) startStageRun(
	ctx context.Context,
	runID string,
	operationID string,
	session creationdomain.Session,
	request GenerateRequest,
	stage creationdomain.Stage,
	sources []agentkernel.Source,
) (agentdomain.Run, error) {
	binding, err := bindingForEntry(session.EntryKind(), stage)
	if err != nil {
		return agentdomain.Run{}, err
	}
	created, err := service.kernel.CreateRun(ctx, agentkernel.CreateRequest{
		RunID: runID, ProjectID: session.ProjectID, BranchID: request.BranchID, ChapterID: request.ChapterID,
		ChapterBaseRevision: 1, CanonicalHead: request.CanonicalHead, AuthorInstruction: creationInstruction(session),
		WorkflowID: binding.WorkflowID, WorkflowVersion: binding.Version, Profile: request.Profile,
		Sources: sources, QualitySpec: quality.Spec{}, Deadline: service.clock.Now().Add(defaultRunTimeout),
	})
	if err != nil {
		return agentdomain.Run{}, err
	}
	// 重读后用独立槽版本 CAS；读取与写入之间的作者修改不参与回写。
	latest, err := service.loadSession(ctx, session.ProjectID)
	if err != nil {
		return agentdomain.Run{}, err
	}
	if strings.TrimSpace(latest.ActiveRunID) != "" && latest.ActiveRunID != created.ID {
		return agentdomain.Run{}, ErrRunActive
	}
	latest, err = latest.AttachRun(operationID, created.ID, service.clock.Now())
	if err != nil {
		return agentdomain.Run{}, err
	}
	if err := service.store.SaveRunSlot(ctx, latest); err != nil {
		return agentdomain.Run{}, err
	}
	return created, nil
}

// executeStageRun 在租约下执行运行并物化候选；不清理运行槽，由操作包装方统一清理。
func (service *Service) executeStageRun(
	lease RunLease,
	session creationdomain.Session,
	request GenerateRequest,
	stage creationdomain.Stage,
	runID, frozenDigest string,
	captured uint64,
) (GenerateResult, error) {
	if service.generation() != captured || lease.Stale() {
		return GenerateResult{}, ErrStaleStore
	}
	binding, err := bindingForEntry(session.EntryKind(), stage)
	if err != nil {
		return GenerateResult{}, err
	}
	completed, err := service.kernel.ExecuteRun(lease.Context(), runID, request.CanonicalHead)
	if err != nil {
		return GenerateResult{}, err
	}
	storeCtx := context.WithoutCancel(lease.Context())
	if service.generation() != captured || lease.Stale() {
		return GenerateResult{}, ErrStaleStore
	}
	artifacts, err := service.kernel.Artifacts(storeCtx, runID)
	if err != nil {
		return GenerateResult{}, err
	}
	payload, artifactID, err := service.resultPayload(storeCtx, completed, artifacts, binding)
	if err != nil {
		return GenerateResult{}, err
	}
	candidate, err := service.persistCandidate(storeCtx, session, stage, runID, artifactID, payload, completed, frozenDigest, binding)
	if err != nil {
		return GenerateResult{}, err
	}
	// 只读回最新会话投影，不整份回写作者期间的修改。
	latest, err := service.loadSession(storeCtx, session.ProjectID)
	if err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Session: latest, Candidate: candidate, RunID: runID}, nil
}

// resultPayload 按冻结工作流的结果出口选择最终产物：优先结果节点（有成功修订时
// 指向最后被评估的修订稿），否则取拓扑序最后一个结构化输出节点；不搜索能力或取第一个 Artifact。
func (service *Service) resultPayload(
	ctx context.Context,
	run agentdomain.Run,
	artifacts []agentdomain.Artifact,
	binding stageBinding,
) (string, string, error) {
	compiled, err := service.compile(ctx, binding)
	if err != nil {
		return "", "", err
	}
	completed := make(map[string]agentdomain.RunStep, len(run.Steps))
	for _, step := range run.Steps {
		if step.Status == agentdomain.StepCompleted && strings.TrimSpace(step.PrimaryArtifactID) != "" {
			completed[step.NodeID] = step
		}
	}
	artifactContent := func(step agentdomain.RunStep) (string, string, error) {
		for _, artifact := range artifacts {
			if artifact.ID == step.PrimaryArtifactID && strings.TrimSpace(artifact.Content) != "" {
				return artifact.Content, artifact.ID, nil
			}
		}
		return "", "", fmt.Errorf("%w: missing artifact %s", ErrUnavailable, step.PrimaryArtifactID)
	}
	if step, ok := completed[compiled.ResultNodeID]; ok && structuredNode(compiled, compiled.ResultNodeID) {
		return artifactContent(step)
	}
	for index := len(compiled.Nodes) - 1; index >= 0; index-- {
		node := compiled.Nodes[index]
		if node.OutputKind != agentdomain.OutputStructured {
			continue
		}
		if step, ok := completed[node.ID]; ok {
			return artifactContent(step)
		}
	}
	return "", "", fmt.Errorf("%w: missing structured result artifact for %s@%d", ErrUnavailable, binding.WorkflowID, binding.Version)
}

func structuredNode(compiled agentkernel.CompiledWorkflow, nodeID string) bool {
	for _, node := range compiled.Nodes {
		if node.ID == nodeID {
			return node.OutputKind == agentdomain.OutputStructured
		}
	}
	return false
}

func (service *Service) compile(ctx context.Context, binding stageBinding) (agentkernel.CompiledWorkflow, error) {
	return agentkernel.Compile(ctx, agentkernel.Registries{
		Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry,
	}, binding.WorkflowID, binding.Version)
}

// autoAdvance 在作者授权的阶段范围内连续生成；质量未通过、预算或待处理冲突时暂停。
func (service *Service) autoAdvance(lease RunLease, result GenerateResult, request GenerateRequest, captured uint64) (GenerateResult, error) {
	storeCtx := context.WithoutCancel(lease.Context())
	session := result.Session
	for {
		if err := lease.Context().Err(); err != nil {
			return result, err
		}
		if service.generation() != captured || lease.Stale() {
			return GenerateResult{}, ErrStaleStore
		}
		completed, readErr := service.kernel.Run(storeCtx, result.RunID)
		if readErr != nil {
			return result, readErr
		}
		request.Profile.MaxRunTokens -= completed.InputTokens + completed.OutputTokens
		// 质量门禁：needs_review/blocked 都暂停自动推进，由作者处理。
		if result.Candidate.ID != "" && result.Candidate.QualityStatus != creationdomain.QualityPassed {
			paused, err := service.pauseSession(storeCtx, session, creationdomain.BlockQualityReview)
			if err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		next, ok := creationdomain.NextStage(session.Stage)
		if !ok || session.MustPauseBefore(next) {
			paused, err := service.pauseSession(storeCtx, session, creationdomain.BlockFormalWrite)
			if err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		if request.Profile.RunBudgetUnlimited {
			paused, err := service.pauseSession(storeCtx, session, creationdomain.BlockCostAuthorization)
			if err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		candidates, err := service.store.ListCandidates(storeCtx, session.ID)
		if err != nil {
			return GenerateResult{}, err
		}
		if pending, err := service.store.PendingActionCount(storeCtx, session.ProjectID); err != nil {
			return GenerateResult{}, err
		} else if pending > 0 {
			paused, pauseErr := service.pauseSession(storeCtx, session, creationdomain.BlockPendingAction)
			if pauseErr != nil {
				return GenerateResult{}, pauseErr
			}
			result.Session = paused
			return result, nil
		}
		if current, exists := currentCandidate(candidates, session.Stage); exists && current.HasLocks() {
			paused, err := service.pauseSession(storeCtx, session, creationdomain.BlockLockedFields)
			if err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		if unresolvedMustFix(session, candidates) {
			paused, err := service.pauseSession(storeCtx, session, creationdomain.BlockConflict)
			if err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		if err := stageReady(session, candidates); err != nil {
			result.Session = session
			return result, nil
		}
		// 推进前重读会话，避免覆盖作者期间的修改。
		latest, err := service.loadSession(storeCtx, session.ProjectID)
		if err != nil {
			return GenerateResult{}, err
		}
		if latest.ContentVersion != session.ContentVersion || latest.AdvanceMode != creationdomain.ModeAutoAdvance || latest.Stage != session.Stage {
			result.Session = latest
			return result, nil
		}
		nextBinding, err := bindingFor(next)
		if err != nil {
			return result, err
		}
		compiled, err := service.compile(storeCtx, nextBinding)
		if err != nil {
			return result, err
		}
		if request.Profile.MaxRunTokens < compiled.RequiredTokenBudget() {
			result.Session, err = service.pauseSession(storeCtx, latest, creationdomain.BlockCostAuthorization)
			return result, err
		}
		latest, err = latest.Advance(service.clock.Now())
		if err != nil {
			return GenerateResult{}, err
		}
		if err := service.saveSession(storeCtx, &latest); err != nil {
			return GenerateResult{}, err
		}
		session = latest
		sources, err := generateSources(session, candidates, session.Stage)
		if err != nil {
			return GenerateResult{}, err
		}
		frozen := liveUpstreamDigest(session, candidates, session.Stage)
		// 短锁内交接运行槽：释放上一子运行的槽位并占用下一阶段槽位，
		// 防止与作者显式确认的新操作并发占用。
		unlock := service.lockProject(session.ProjectID)
		unlock.Lock()
		runID := service.ids.NewID()
		childLease, childRelease, leaseErr := service.coordinator.BeginRun(lease.Context(), runID)
		if leaseErr != nil {
			unlock.Unlock()
			return result, leaseErr
		}
		if err := service.clearRunSlot(storeCtx, session.ProjectID, result.RunID); err != nil {
			childRelease()
			unlock.Unlock()
			return GenerateResult{}, err
		}
		run, startErr := service.startStageRun(childLease.Context(), runID, result.Session.ActiveOperationID, session, request, session.Stage, sources)
		unlock.Unlock()
		if startErr != nil {
			childRelease()
			return GenerateResult{}, startErr
		}
		operation, operationErr := service.store.Operation(storeCtx, session.ActiveOperationID)
		if operationErr != nil {
			childRelease()
			return GenerateResult{}, operationErr
		}
		operation.RunID = run.ID
		operation.FrozenInputDigest = frozen
		operation.Status = creationdomain.OperationRunning
		operation.UpdatedAt = service.clock.Now()
		if operationErr := service.store.UpdateOperation(storeCtx, operation); operationErr != nil {
			childRelease()
			return GenerateResult{}, operationErr
		}
		advanced, err := service.executeStageRun(childLease, session, request, session.Stage, run.ID, frozen, captured)
		childRelease()
		if err != nil {
			return GenerateResult{}, err
		}
		result = advanced
		session = advanced.Session
	}
}

// pauseSession 重读最新会话后只设置暂停原因，不覆盖其他字段。
func (service *Service) pauseSession(ctx context.Context, session creationdomain.Session, reason creationdomain.BlockingReason) (creationdomain.Session, error) {
	latest, err := service.loadSession(ctx, session.ProjectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if latest.ActiveRunID != session.ActiveRunID || latest.RunSlotVersion != session.RunSlotVersion {
		return latest, creationdomain.ErrSessionVersionConflict
	}
	paused, err := latest.Pause(reason, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveRunSlot(ctx, paused); err != nil {
		return creationdomain.Session{}, err
	}
	return service.loadSession(ctx, session.ProjectID)
}

// clearRunSlot 只在运行槽仍指向本次运行时清除，不覆盖作者期间的修改。
func (service *Service) clearRunSlot(ctx context.Context, projectID, runID string) error {
	// 仅对同一运行的幂等清理重试；每次都重新核对归属，绝不重放内容写入。
	for range 3 {
		latest, err := service.loadSession(ctx, projectID)
		if err != nil {
			return err
		}
		if latest.ActiveRunID != runID {
			return nil
		}
		err = service.store.SaveRunSlot(ctx, latest.ClearActiveRun(service.clock.Now()))
		if !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
			return err
		}
	}
	return creationdomain.ErrSessionVersionConflict
}

// CancelRun 取消进行中的生成；进程内无对应运行时清理恢复后残留的运行槽。
func (service *Service) CancelRun(ctx context.Context, projectID string) error {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(session.ActiveRunID) == "" {
		return nil
	}
	if service.coordinator.Live(session.ActiveRunID) {
		service.coordinator.Cancel(session.ActiveRunID)
		return nil
	}
	return service.clearRunSlot(ctx, projectID, session.ActiveRunID)
}

// Recover 恢复会话状态，但绝不自动继续未重新确认的付费运行。
// 进程内无活跃执行时按持久化运行状态对账：完成的运行物化结果，终结的清理槽位，中断的暂停并释放槽。
func (service *Service) Recover(ctx context.Context, projectID string) (creationdomain.Session, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if strings.TrimSpace(session.ActiveRunID) == "" {
		return session, nil
	}
	if service.coordinator.Live(session.ActiveRunID) {
		return session, nil
	}
	return service.reconcileInterruptedRun(ctx, session)
}

func (service *Service) reconcileInterruptedRun(ctx context.Context, session creationdomain.Session) (creationdomain.Session, error) {
	run, err := service.kernel.Run(ctx, session.ActiveRunID)
	if err != nil {
		return service.markInterrupted(ctx, session)
	}
	switch run.Status {
	case agentdomain.RunCompleted:
		if materializeErr := service.materializeRunResult(ctx, session, run); materializeErr != nil {
			return session, materializeErr
		}
		if saveErr := service.clearRunSlot(ctx, session.ProjectID, run.ID); saveErr != nil {
			return session, saveErr
		}
		return service.loadSession(ctx, session.ProjectID)
	case agentdomain.RunFailed, agentdomain.RunCancelled:
		if saveErr := service.clearRunSlot(ctx, session.ProjectID, session.ActiveRunID); saveErr != nil {
			return session, saveErr
		}
		return service.loadSession(ctx, session.ProjectID)
	default:
		return service.markInterrupted(ctx, session)
	}
}

// markInterrupted 处理执行中被中断的运行：保留运行引用与工件用量记录并暂停会话；
// 作者显式取消旧运行后才能发起新运行，不自动重放（P2-14）。
func (service *Service) markInterrupted(ctx context.Context, session creationdomain.Session) (creationdomain.Session, error) {
	paused, err := session.Pause(creationdomain.BlockCostAuthorization, service.clock.Now())
	if err != nil {
		return session, err
	}
	if err := service.store.SaveRunSlot(ctx, paused); err != nil {
		return session, err
	}
	return service.loadSession(ctx, session.ProjectID)
}

// materializeRunResult 结果重放：运行完成但候选未落库时，按运行产物物化一次候选，不调用模型。
func (service *Service) materializeRunResult(ctx context.Context, session creationdomain.Session, run agentdomain.Run) error {
	binding, err := bindingByWorkflow(run.WorkflowBinding.ID, run.WorkflowBinding.Version)
	if err != nil {
		return nil
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if candidate.RunID == run.ID {
			return service.refreshImpact(ctx, session)
		}
	}
	operation, err := service.store.Operation(ctx, session.ActiveOperationID)
	if err != nil {
		return err
	}
	if operation.RunID != run.ID {
		return fmt.Errorf("%w: operation %s belongs to run %s, not %s", ErrUnavailable, operation.ID, operation.RunID, run.ID)
	}
	if strings.TrimSpace(operation.FrozenInputDigest) == "" {
		return fmt.Errorf("%w: missing frozen input digest for run %s", ErrUnavailable, run.ID)
	}
	artifacts, err := service.kernel.Artifacts(ctx, run.ID)
	if err != nil {
		return err
	}
	payload, artifactID, err := service.resultPayload(ctx, run, artifacts, binding)
	if err != nil {
		return err
	}
	_, err = service.persistCandidate(ctx, session, binding.Stage, run.ID, artifactID, payload, run, operation.FrozenInputDigest, binding)
	return err
}

// persistCandidate 把运行产物落为候选新版本；已有版本时走 Regenerate 合并锁定字段并继承锁。
func (service *Service) persistCandidate(
	ctx context.Context,
	session creationdomain.Session,
	stage creationdomain.Stage,
	runID, artifactID, payload string,
	run agentdomain.Run,
	frozenDigest string,
	binding stageBinding,
) (creationdomain.Candidate, error) {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if binding.SchemaID == "author_material.schema" {
		if _, err := decodeMaterialForEntry(session.Entry, payload); err != nil {
			return creationdomain.Candidate{}, err
		}
	}
	version := 1
	var previous creationdomain.Candidate
	hasPrevious := false
	for _, item := range candidates {
		if item.Stage != stage || item.Kind != binding.Kind {
			continue
		}
		if item.Version >= version {
			version = item.Version + 1
		}
		if item.Status != creationdomain.StatusSuperseded && item.Status != creationdomain.StatusDiscarded &&
			(!hasPrevious || item.Version > previous.Version) {
			previous = item
			hasPrevious = true
		}
	}
	var candidate creationdomain.Candidate
	if hasPrevious {
		// 重生成强制保留锁定字段与锁元数据（P1-4）。
		candidate, err = previous.Regenerate(service.ids.NewID(), runID, artifactID, payload, service.clock.Now())
		if err == nil {
			candidate.Version = version
		}
	} else {
		candidate, err = creationdomain.NewCandidate(
			service.ids.NewID(), session.ID, stage, binding.Kind, version,
			"", runID, artifactID, payload, binding.SchemaID, binding.SchemaVersion, service.clock.Now(),
		)
	}
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	candidate = candidate.WithUpstream(frozenDigest).WithQuality(candidateQuality(run.QualityStatus))
	if hasPrevious {
		superseded, markErr := previous.MarkSuperseded(service.clock.Now())
		if markErr != nil {
			return creationdomain.Candidate{}, markErr
		}
		if err := service.store.ApplyCandidateChanges(ctx,
			[]creationdomain.Candidate{previous}, []creationdomain.Candidate{superseded}, []creationdomain.Candidate{candidate},
		); err != nil {
			if errors.Is(err, creationdomain.ErrCandidateResultExists) {
				return service.candidateForRunResult(ctx, session, runID, artifactID, binding.Kind)
			}
			return creationdomain.Candidate{}, err
		}
	} else if err := service.store.SaveCandidate(ctx, candidate); err != nil {
		if errors.Is(err, creationdomain.ErrCandidateResultExists) {
			return service.candidateForRunResult(ctx, session, runID, artifactID, binding.Kind)
		}
		return creationdomain.Candidate{}, err
	}
	// 用最新会话重算下游影响，避免旧快照误判 stale。
	latest, err := service.loadSession(ctx, session.ProjectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.refreshImpact(ctx, latest); err != nil {
		return creationdomain.Candidate{}, err
	}
	return candidate, nil
}

func (service *Service) candidateForRunResult(
	ctx context.Context, session creationdomain.Session, runID, artifactID string, kind creationdomain.CandidateKind,
) (creationdomain.Candidate, error) {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	for _, candidate := range candidates {
		if candidate.RunID == runID && candidate.ArtifactID == artifactID && candidate.Kind == kind {
			latest, loadErr := service.loadSession(ctx, session.ProjectID)
			if loadErr != nil {
				return creationdomain.Candidate{}, loadErr
			}
			if refreshErr := service.refreshImpact(ctx, latest); refreshErr != nil {
				return creationdomain.Candidate{}, refreshErr
			}
			return service.store.Candidate(ctx, candidate.ID)
		}
	}
	return creationdomain.Candidate{}, creationdomain.ErrCandidateResultExists
}

// PreviewConfirmArchive 冻结档案确认摘要，不写正式资产。
func (service *Service) PreviewConfirmArchive(ctx context.Context, projectID string) (ArchivePreview, error) {
	session, candidates, err := service.loadArchiveInputs(ctx, projectID)
	if err != nil {
		return ArchivePreview{}, err
	}
	if err := validateArchiveReadiness(session, candidates); err != nil {
		return ArchivePreview{}, err
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return ArchivePreview{}, err
	}
	return buildArchivePreview(session, candidates, project)
}

// buildArchivePreview 从已读取的单一快照（会话 + 候选 + 项目基线）计算确认摘要与预览，不再读库。
// ConfirmArchive 复用同一份快照，避免“预览摘要来自一次读取、入档计划来自另一次读取”的错位。
func buildArchivePreview(session creationdomain.Session, candidates []creationdomain.Candidate, project projectdomain.Project) (ArchivePreview, error) {
	digest, err := archiveDigest(session, candidates, project)
	if err != nil {
		return ArchivePreview{}, err
	}
	proposal, people, roadmap, err := requiredArchiveCandidates(session, candidates)
	if err != nil {
		return ArchivePreview{}, err
	}
	blueprint, err := decodeBlueprint(proposal.PayloadJSON)
	if err != nil {
		return ArchivePreview{}, err
	}
	cast, err := decodeArchivableCast(people)
	if err != nil {
		return ArchivePreview{}, err
	}
	road, err := decodeArchivableRoadmap(roadmap)
	if err != nil {
		return ArchivePreview{}, err
	}
	return ArchivePreview{
		ConfirmationDigest: digest, Title: blueprint.Title,
		CharacterCount: len(cast.Characters),
		OutlineCount:   1 + len(road.Volumes) + len(road.Arcs) + len(road.NearTermChapters),
	}, nil
}

// ConfirmArchive 把已阶段确认的候选映射为 ChangeSet 并一次性写入正式资产。
// 预览摘要、入档计划与写入的会话/候选/项目都来自同一次读取；候选确认在事务内按快照条件执行，
// 选中方向作为上游依赖一并校验，预览后发生的改锁、改选、候选修改、质量或项目基线变化都会被拒绝并整单回滚。
func (service *Service) ConfirmArchive(ctx context.Context, request ArchiveRequest) (ArchiveResult, error) {
	session, candidates, err := service.loadArchiveInputs(ctx, request.ProjectID)
	if err != nil {
		return ArchiveResult{}, err
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return ArchiveResult{}, creationdomain.ErrBlankValue
	}
	if strings.TrimSpace(request.ConfirmationDigest) == "" {
		return ArchiveResult{}, ErrConfirmationRequired
	}
	// 幂等重放：已入档时按回执键与请求摘要直接返回原结果，不重新计算实时摘要，
	// 入档成功后的候选/锁/质量/项目变化不影响已成功 ChangeSet 的重放。
	if strings.TrimSpace(session.ArchiveChangeSetID) != "" {
		if replay, ok := matchingArchiveReceipt(session, request); ok {
			return replay, nil
		}
		return ArchiveResult{}, ErrConfirmationRequired
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return ArchiveResult{}, err
	}
	preview, err := buildArchivePreview(session, candidates, project)
	if err != nil {
		return ArchiveResult{}, err
	}
	if request.ConfirmationDigest != preview.ConfirmationDigest {
		return ArchiveResult{}, ErrConfirmationRequired
	}
	if err := validateArchiveReadiness(session, candidates); err != nil {
		return ArchiveResult{}, err
	}
	workbench, err := service.store.WorkbenchState(ctx)
	if err != nil {
		workbench = projectdomain.WorkbenchState{}
	}
	now := service.clock.Now()
	if session.Stage == creationdomain.StageRoadmap {
		session, err = session.Advance(now)
		if err != nil {
			return ArchiveResult{}, err
		}
	}
	if session.Stage != creationdomain.StageArchiveConfirm {
		return ArchiveResult{}, ErrInvalidStage
	}
	plan, err := buildArchivePlan(session, candidates, project, service.ids)
	if err != nil {
		return ArchiveResult{}, err
	}
	changeSet, err := revisiondomain.NewChangeSet(
		service.ids.NewID(), session.ProjectID, "确认作品档案", plan.Title, "author",
		plan.Specs, now,
	)
	if err != nil {
		return ArchiveResult{}, err
	}
	verify := []creationdomain.Candidate{}
	if session.EntryKind() != creationdomain.EntryAuthorPlan {
		direction, err := requiredSelectedDirectionCandidate(session, candidates)
		if err != nil {
			return ArchiveResult{}, err
		}
		verify = append(verify, direction)
	} else {
		analysis, _, err := validatedMaterial(session, candidates)
		if err != nil {
			return ArchiveResult{}, err
		}
		verify = append(verify, analysis)
	}
	session, err = session.RecordArchive(preview.ConfirmationDigest, request.IdempotencyKey, changeSet.ID, now)
	if err != nil {
		return ArchiveResult{}, err
	}
	workbench.Stage = projectdomain.StageWriting
	workbench.ArchiveStatus = "confirmed"
	workbench.UpdatedAt = now
	if err := service.store.ApplyArchive(ctx, ArchiveApply{
		ChangeSet: changeSet, Session: session, Candidates: plan.Confirmed,
		Verify: verify, Workbench: workbench,
	}); err != nil {
		// 另一个完全相同的请求可能刚刚提交成功。事务失败后只信任持久化回执；
		// 不匹配或读取失败仍返回原错误，不能把真实冲突伪装成幂等成功。
		if latest, readErr := service.loadSession(ctx, request.ProjectID); readErr == nil {
			if replay, ok := matchingArchiveReceipt(latest, request); ok {
				return replay, nil
			}
		}
		return ArchiveResult{}, err
	}
	session.ContentVersion++
	session.RunSlotVersion++
	return ArchiveResult{Session: session, ChangeSetID: changeSet.ID}, nil
}

// matchingArchiveReceipt 仅匹配已原子提交、且键与参数摘要都相同的入档回执。
func matchingArchiveReceipt(session creationdomain.Session, request ArchiveRequest) (ArchiveResult, bool) {
	if strings.TrimSpace(session.ArchiveChangeSetID) == "" ||
		session.ArchiveIdempotencyKey != request.IdempotencyKey ||
		session.ArchiveDigest != request.ConfirmationDigest {
		return ArchiveResult{}, false
	}
	return ArchiveResult{Session: session, ChangeSetID: session.ArchiveChangeSetID}, true
}

func (service *Service) loadArchiveInputs(ctx context.Context, projectID string) (creationdomain.Session, []creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, nil, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Session{}, nil, err
	}
	return session, candidates, nil
}

func (service *Service) preview(ctx context.Context, session creationdomain.Session, profile agentdomain.ResolvedCreativeProfile, stage creationdomain.Stage) (CostPreview, error) {
	if session.EntryKind() == creationdomain.EntryAuthorPlan && stage != creationdomain.StageIntent {
		candidates, err := service.store.ListCandidates(ctx, session.ID)
		if err != nil {
			return CostPreview{}, err
		}
		_, output, err := validatedMaterial(session, candidates)
		if err != nil {
			return CostPreview{}, err
		}
		for _, gap := range output.Gaps {
			if gap.Kind == "conflict" {
				return CostPreview{}, creationdomain.ErrInvalidGapReport
			}
		}
	}

	binding, err := bindingForEntry(session.EntryKind(), stage)
	if err != nil {
		return CostPreview{}, err
	}
	compiled, err := service.compile(ctx, binding)
	if err != nil {
		return CostPreview{}, err
	}
	if !profile.RunBudgetUnlimited && profile.MaxRunTokens < compiled.RequiredTokenBudget() {
		return CostPreview{}, fmt.Errorf("%w: 创作方案运行预算至少需要 %d Token", agentkernel.ErrTokenBudget, compiled.RequiredTokenBudget())
	}
	remaining := []creationdomain.Stage{}
	estimated := len(compiled.Nodes)
	if session.AdvanceMode == creationdomain.ModeAutoAdvance {
		remaining = remainingGenerateStages(stage)
		for _, next := range remaining {
			nextBinding, bindErr := bindingForEntry(session.EntryKind(), next)
			if bindErr != nil {
				return CostPreview{}, bindErr
			}
			nextCompiled, compileErr := service.compile(ctx, nextBinding)
			if compileErr != nil {
				return CostPreview{}, compileErr
			}
			estimated += len(nextCompiled.Nodes)
		}
	}
	preview := CostPreview{
		Stage: stage, WorkflowID: binding.WorkflowID, WorkflowVersion: binding.Version,
		EstimatedCalls: estimated, MaxRunTokens: profile.MaxRunTokens,
		RunBudgetUnlimited: profile.RunBudgetUnlimited, RemainingStages: remaining,
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return CostPreview{}, err
	}
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return CostPreview{}, err
	}
	for _, role := range profile.Roles {
		preview.RoleModels = append(preview.RoleModels, RoleModelView{
			Role: string(role.Role), Provider: role.ProviderName, Model: role.Model,
		})
	}
	payload, err := json.Marshal(struct {
		UpstreamDigest, ResolvedProfile                              string
		Stage, WorkflowID, WorkflowHash, ProfileID, Intent, Selected string
		WorkflowVersion                                              int
		ProfileVersion                                               int64
		AutoAdvance                                                  bool
		Remaining                                                    []creationdomain.Stage
		MaxRunTokens                                                 int
		Unlimited                                                    bool
	}{
		UpstreamDigest: liveUpstreamDigest(session, candidates, stage), ResolvedProfile: string(profileJSON),
		Stage: string(stage), WorkflowID: compiled.Workflow.ID, WorkflowHash: compiled.Workflow.Hash,
		ProfileID: profile.ProfileID, Intent: session.IntentText, Selected: session.SelectedDirectionCandidateID,
		WorkflowVersion: compiled.Workflow.Version, ProfileVersion: profile.Version,
		AutoAdvance: session.AdvanceMode == creationdomain.ModeAutoAdvance, Remaining: remaining,
		MaxRunTokens: profile.MaxRunTokens, Unlimited: profile.RunBudgetUnlimited,
	})
	if err != nil {
		return CostPreview{}, err
	}
	sum := sha256.Sum256(payload)
	preview.ConfirmationDigest = hex.EncodeToString(sum[:])
	return preview, nil
}

func resolveGenerateStage(session creationdomain.Session, requested creationdomain.Stage) (creationdomain.Stage, error) {
	if session.EntryKind() == creationdomain.EntryAuthorPlan && requested == creationdomain.StageDirections {
		return "", ErrInvalidStage
	}
	if session.EntryKind() != creationdomain.EntryAuthorPlan && requested == creationdomain.StageIntent {
		return "", ErrInvalidStage
	}
	if requested != "" {
		if _, err := bindingFor(requested); err != nil {
			return "", err
		}
		return requested, nil
	}
	if session.Stage == creationdomain.StageIntent {
		if session.EntryKind() == creationdomain.EntryAuthorPlan {
			return creationdomain.StageIntent, nil
		}
		return creationdomain.StageDirections, nil
	}
	if _, err := bindingFor(session.Stage); err != nil {
		return "", err
	}
	return session.Stage, nil
}

func generateCommand(stage creationdomain.Stage) string {
	return "generate:" + string(stage)
}

func operationErrorCode(err error) string {
	text := err.Error()
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}

// candidateQuality 把内核运行质量结论映射为候选质量状态。
func candidateQuality(status agentdomain.QualityStatus) creationdomain.CandidateQuality {
	switch status {
	case agentdomain.QualityPassed:
		return creationdomain.QualityPassed
	case agentdomain.QualityNeedsReview:
		return creationdomain.QualityNeedsReview
	case agentdomain.QualityBlocked:
		return creationdomain.QualityBlocked
	default:
		return creationdomain.QualityNone
	}
}

// hashParts 稳定拼接并计算参数摘要。
func hashParts(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

const defaultRunTimeout = 30 * time.Minute
