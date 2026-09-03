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
	"github.com/lishen802/novel-studio/internal/quality"
)

var (
	ErrConfirmationRequired = errors.New("creation cost confirmation required")
	ErrRunActive            = errors.New("creation run is already active")
	ErrInvalidStage         = errors.New("invalid creation generate stage")
	ErrUnavailable          = errors.New("creation service unavailable")
)

type Kernel interface {
	CreateRun(context.Context, agentkernel.CreateRequest) (agentdomain.Run, error)
	ExecuteRun(context.Context, string, string) (agentdomain.Run, error)
	Artifacts(context.Context, string) ([]agentdomain.Artifact, error)
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

type CostPreview struct {
	Stage              creationdomain.Stage
	WorkflowID         string
	WorkflowVersion    int
	EstimatedCalls     int
	MaxRunTokens       int
	RunBudgetUnlimited bool
	RemainingStages    []creationdomain.Stage
	ConfirmationDigest string
}

type GenerateResult struct {
	Session   creationdomain.Session
	Candidate creationdomain.Candidate
	RunID     string
}

type Config struct {
	Store      Store
	Kernel     Kernel
	Clock      ports.Clock
	IDs        ports.IDGenerator
	Generation func() uint64
}

type Service struct {
	store      Store
	kernel     Kernel
	clock      ports.Clock
	ids        ports.IDGenerator
	generation func() uint64
	registry   *agentassets.Registry
	mu         sync.Mutex
	cancels    map[string]context.CancelFunc
	grants     map[string]string
	results    map[string]GenerateResult
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
	return &Service{
		store: config.Store, kernel: config.Kernel, clock: config.Clock, ids: config.IDs,
		generation: generation, registry: registry,
		cancels: map[string]context.CancelFunc{}, grants: map[string]string{},
		results: map[string]GenerateResult{},
	}, nil
}

// EnsureSession 为项目取得或新建原创构建会话。
func (service *Service) EnsureSession(ctx context.Context, projectID, branchID string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err == nil {
		return session, nil
	}
	if !isMissingSession(err) {
		return creationdomain.Session{}, err
	}
	session, err = creationdomain.NewSession(service.ids.NewID(), projectID, branchID, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// SaveIntent 保存作者创意文本，不创建运行。
func (service *Service) SaveIntent(ctx context.Context, projectID, text string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SaveIntent(text, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// SetAdvanceMode 切换逐步审阅或自动推进，不触发付费调用。
func (service *Service) SetAdvanceMode(ctx context.Context, projectID string, mode creationdomain.AdvanceMode) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SetAdvanceMode(mode, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// SelectDirection 记录作者选中的方向候选，仍不写正式资产。
func (service *Service) SelectDirection(ctx context.Context, projectID, candidateID string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if _, err := service.store.Candidate(ctx, candidateID); err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.SelectDirection(candidateID, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// AdvanceStage 在当前阶段完成条件满足时进入下一阶段，不写类型表。
func (service *Service) AdvanceStage(ctx context.Context, projectID string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := stageReady(session, candidates); err != nil {
		return creationdomain.Session{}, err
	}
	session, err = session.Advance(service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

// PreviewGenerate 冻结费用预览摘要，不创建运行、不写候选。
func (service *Service) PreviewGenerate(ctx context.Context, request GenerateRequest) (CostPreview, error) {
	session, err := service.store.Session(ctx, request.ProjectID)
	if err != nil {
		return CostPreview{}, err
	}
	stage, err := resolveGenerateStage(session, request.Stage)
	if err != nil {
		return CostPreview{}, err
	}
	return service.preview(ctx, session, request.Profile, stage)
}

// ConfirmGenerate 校验预算摘要后创建内核运行，并把架构产物落为候选。
func (service *Service) ConfirmGenerate(ctx context.Context, request GenerateRequest) (GenerateResult, error) {
	session, err := service.store.Session(ctx, request.ProjectID)
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
	if cached, ok := service.lookupResult(request); ok {
		return cached, nil
	}
	if strings.TrimSpace(session.ActiveRunID) != "" {
		return GenerateResult{}, ErrRunActive
	}
	captured := service.generation()
	if session.Stage == creationdomain.StageIntent && stage == creationdomain.StageDirections {
		session, err = session.Advance(service.clock.Now())
		if err != nil {
			return GenerateResult{}, err
		}
		if err := service.store.SaveSession(ctx, session); err != nil {
			return GenerateResult{}, err
		}
	}
	result, err := service.runStage(ctx, session, request, stage, captured)
	if err != nil {
		return GenerateResult{}, err
	}
	service.rememberGrant(request.ProjectID, preview.ConfirmationDigest)
	if session.AdvanceMode == creationdomain.ModeAutoAdvance {
		result, err = service.autoAdvance(ctx, result, request, captured)
		if err != nil {
			return GenerateResult{}, err
		}
	}
	service.rememberResult(request, result)
	return result, nil
}

// CancelRun 取消进行中的生成；若进程内没有对应运行，则清掉恢复后残留的 ActiveRunID。
func (service *Service) CancelRun(ctx context.Context, projectID string) error {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return err
	}
	service.mu.Lock()
	cancel := service.cancels[session.ActiveRunID]
	service.mu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}
	if strings.TrimSpace(session.ActiveRunID) == "" {
		return nil
	}
	return service.store.SaveSession(ctx, session.ClearActiveRun(service.clock.Now()))
}

// Recover 恢复会话状态，但绝不自动继续未重新确认的付费运行。
func (service *Service) Recover(ctx context.Context, projectID string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Session{}, err
	}
	service.forgetGrant(projectID)
	if strings.TrimSpace(session.ActiveRunID) == "" {
		return session, nil
	}
	session, err = session.Pause(creationdomain.BlockCostAuthorization, service.clock.Now())
	if err != nil {
		return creationdomain.Session{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return creationdomain.Session{}, err
	}
	return session, nil
}

func (service *Service) preview(ctx context.Context, session creationdomain.Session, profile agentdomain.ResolvedCreativeProfile, stage creationdomain.Stage) (CostPreview, error) {
	binding, err := bindingFor(stage)
	if err != nil {
		return CostPreview{}, err
	}
	compiled, err := agentkernel.Compile(ctx, agentkernel.Registries{
		Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry,
	}, binding.WorkflowID, binding.Version)
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
			nextBinding, bindErr := bindingFor(next)
			if bindErr != nil {
				return CostPreview{}, bindErr
			}
			nextCompiled, compileErr := agentkernel.Compile(ctx, agentkernel.Registries{
				Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry,
			}, nextBinding.WorkflowID, nextBinding.Version)
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
	payload, err := json.Marshal(struct {
		Stage, WorkflowID, WorkflowHash, ProfileID, Intent, Selected string
		WorkflowVersion                                              int
		ProfileVersion                                               int64
		AutoAdvance                                                  bool
		Remaining                                                    []creationdomain.Stage
		MaxRunTokens                                                 int
		Unlimited                                                    bool
	}{
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

// runStage 创建并执行一次阶段生成，把架构产物落为候选；store generation 变化则拒绝落库。
func (service *Service) runStage(
	ctx context.Context,
	session creationdomain.Session,
	request GenerateRequest,
	stage creationdomain.Stage,
	captured uint64,
) (GenerateResult, error) {
	if service.generation() != captured {
		return GenerateResult{}, ErrStaleStore
	}
	binding, err := bindingFor(stage)
	if err != nil {
		return GenerateResult{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return GenerateResult{}, err
	}
	sources, err := generateSources(session, candidates, stage)
	if err != nil {
		return GenerateResult{}, err
	}
	runID := service.ids.NewID()
	created, err := service.kernel.CreateRun(ctx, agentkernel.CreateRequest{
		RunID: runID, ProjectID: session.ProjectID, BranchID: request.BranchID, ChapterID: request.ChapterID,
		ChapterBaseRevision: 1, CanonicalHead: request.CanonicalHead, AuthorInstruction: session.IntentText,
		WorkflowID: binding.WorkflowID, WorkflowVersion: binding.Version, Profile: request.Profile,
		Sources: sources, QualitySpec: quality.Spec{}, Deadline: service.clock.Now().Add(defaultRunTimeout),
	})
	if err != nil {
		return GenerateResult{}, err
	}
	session, err = session.AttachRun(created.ID, service.clock.Now())
	if err != nil {
		return GenerateResult{}, err
	}
	if err := service.store.SaveSession(ctx, session); err != nil {
		return GenerateResult{}, err
	}
	jobCtx, cancel := context.WithCancel(ctx)
	service.mu.Lock()
	service.cancels[created.ID] = cancel
	service.mu.Unlock()
	defer func() {
		cancel()
		service.mu.Lock()
		delete(service.cancels, created.ID)
		service.mu.Unlock()
	}()
	completed, err := service.kernel.ExecuteRun(jobCtx, created.ID, request.CanonicalHead)
	if err != nil {
		session = session.ClearActiveRun(service.clock.Now())
		_ = service.store.SaveSession(ctx, session)
		return GenerateResult{}, err
	}
	if service.generation() != captured {
		session = session.ClearActiveRun(service.clock.Now())
		_ = service.store.SaveSession(ctx, session)
		return GenerateResult{}, ErrStaleStore
	}
	artifacts, err := service.kernel.Artifacts(ctx, completed.ID)
	if err != nil {
		return GenerateResult{}, err
	}
	payload, artifactID, err := architectPayload(completed, artifacts)
	if err != nil {
		return GenerateResult{}, err
	}
	candidate, err := service.persistCandidate(ctx, session, completed.ID, artifactID, payload, binding)
	if err != nil {
		return GenerateResult{}, err
	}
	session = session.ClearActiveRun(service.clock.Now())
	if err := service.store.SaveSession(ctx, session); err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{Session: session, Candidate: candidate, RunID: completed.ID}, nil
}

func (service *Service) autoAdvance(ctx context.Context, result GenerateResult, request GenerateRequest, captured uint64) (GenerateResult, error) {
	session := result.Session
	for {
		if service.generation() != captured {
			return GenerateResult{}, ErrStaleStore
		}
		next, ok := creationdomain.NextStage(session.Stage)
		if !ok || session.MustPauseBefore(next) {
			paused, err := session.Pause(creationdomain.BlockFormalWrite, service.clock.Now())
			if err != nil {
				return GenerateResult{}, err
			}
			if err := service.store.SaveSession(ctx, paused); err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		if request.Profile.RunBudgetUnlimited {
			paused, err := session.Pause(creationdomain.BlockCostAuthorization, service.clock.Now())
			if err != nil {
				return GenerateResult{}, err
			}
			if err := service.store.SaveSession(ctx, paused); err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		pending, err := service.store.PendingActionCount(ctx, session.ProjectID)
		if err != nil {
			return GenerateResult{}, err
		}
		if pending > 0 {
			paused, err := session.Pause(creationdomain.BlockPendingAction, service.clock.Now())
			if err != nil {
				return GenerateResult{}, err
			}
			if err := service.store.SaveSession(ctx, paused); err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		candidates, err := service.store.ListCandidates(ctx, session.ID)
		if err != nil {
			return GenerateResult{}, err
		}
		if current, exists := currentCandidate(candidates, session.Stage); exists && hasLocks(current) {
			paused, err := session.Pause(creationdomain.BlockLockedFields, service.clock.Now())
			if err != nil {
				return GenerateResult{}, err
			}
			if err := service.store.SaveSession(ctx, paused); err != nil {
				return GenerateResult{}, err
			}
			result.Session = paused
			return result, nil
		}
		if err := stageReady(session, candidates); err != nil {
			result.Session = session
			return result, nil
		}
		session, err = session.Advance(service.clock.Now())
		if err != nil {
			return GenerateResult{}, err
		}
		if err := service.store.SaveSession(ctx, session); err != nil {
			return GenerateResult{}, err
		}
		advanced, err := service.runStage(ctx, session, request, session.Stage, captured)
		if err != nil {
			return GenerateResult{}, err
		}
		result = advanced
		session = advanced.Session
	}
}

func (service *Service) persistCandidate(
	ctx context.Context,
	session creationdomain.Session,
	runID, artifactID, payload string,
	binding stageBinding,
) (creationdomain.Candidate, error) {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	version := 1
	var previous creationdomain.Candidate
	hasPrevious := false
	for _, item := range candidates {
		if item.Stage != session.Stage || item.Kind != binding.Kind {
			continue
		}
		if item.Version >= version {
			version = item.Version + 1
		}
		if !hasPrevious || item.Version > previous.Version {
			previous = item
			hasPrevious = true
		}
	}
	parent := ""
	if hasPrevious {
		parent = previous.ID
	}
	candidate, err := creationdomain.NewCandidate(
		service.ids.NewID(), session.ID, session.Stage, binding.Kind, version,
		parent, runID, artifactID, payload, binding.SchemaID, binding.SchemaVersion, service.clock.Now(),
	)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if hasPrevious {
		superseded, markErr := previous.MarkSuperseded(service.clock.Now())
		if markErr != nil {
			return creationdomain.Candidate{}, markErr
		}
		candidate.SupersedesCandidateID = previous.ID
		if err := service.store.UpdateCandidate(ctx, superseded); err != nil {
			return creationdomain.Candidate{}, err
		}
	}
	if err := service.store.SaveCandidate(ctx, candidate); err != nil {
		return creationdomain.Candidate{}, err
	}
	return candidate, nil
}

func (service *Service) rememberGrant(projectID, digest string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.grants[projectID] = digest
}

func (service *Service) forgetGrant(projectID string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	delete(service.grants, projectID)
}

func (service *Service) rememberResult(request GenerateRequest, result GenerateResult) {
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	service.results[request.ProjectID+"\x00"+key] = result
}

func (service *Service) lookupResult(request GenerateRequest) (GenerateResult, bool) {
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return GenerateResult{}, false
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	result, ok := service.results[request.ProjectID+"\x00"+key]
	return result, ok
}

func resolveGenerateStage(session creationdomain.Session, requested creationdomain.Stage) (creationdomain.Stage, error) {
	if requested != "" {
		if _, err := bindingFor(requested); err != nil {
			return "", err
		}
		return requested, nil
	}
	if session.Stage == creationdomain.StageIntent {
		return creationdomain.StageDirections, nil
	}
	if _, err := bindingFor(session.Stage); err != nil {
		return "", err
	}
	return session.Stage, nil
}

func generateSources(session creationdomain.Session, candidates []creationdomain.Candidate, stage creationdomain.Stage) ([]agentkernel.Source, error) {
	sources := []agentkernel.Source{{
		Kind: agentdomain.ManifestAuthorInstruction, ID: "intent", Version: "1",
		Content: session.IntentText, Priority: 1000, SelectionReason: "作者创作意图", Required: true,
	}}
	if stage == creationdomain.StageDirections {
		return sources, nil
	}
	if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
		return nil, creationdomain.ErrStageIncomplete
	}
	selected, ok := candidateByID(candidates, session.SelectedDirectionCandidateID)
	if !ok {
		if current, found := currentCandidate(candidates, creationdomain.StageDirections); found {
			selected = current
			ok = true
		}
	}
	if !ok {
		return nil, creationdomain.ErrStageIncomplete
	}
	sources = append(sources, agentkernel.Source{
		Kind: agentdomain.ManifestStoryAsset, ID: selected.ID, Version: fmt.Sprintf("%d", selected.Version),
		Content: selected.PayloadJSON, Priority: 900, SelectionReason: "已选故事方向", Required: true,
	})
	return sources, nil
}

func stageReady(session creationdomain.Session, candidates []creationdomain.Candidate) error {
	switch session.Stage {
	case creationdomain.StageIntent:
		if strings.TrimSpace(session.IntentText) == "" {
			return creationdomain.ErrStageIncomplete
		}
	case creationdomain.StageDirections:
		if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
			return creationdomain.ErrStageIncomplete
		}
	case creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap:
		current, ok := currentCandidate(candidates, session.Stage)
		if !ok {
			return creationdomain.ErrStageIncomplete
		}
		if err := validateCandidatePayload(session.Stage, current.PayloadJSON); err != nil {
			return err
		}
	}
	return nil
}

func validateCandidatePayload(stage creationdomain.Stage, payload string) error {
	kind := agentdomain.TaskStoryBlueprint
	switch stage {
	case creationdomain.StageDirections:
		kind = agentdomain.TaskStoryDirections
	case creationdomain.StagePeopleWorld:
		kind = agentdomain.TaskStoryCast
	case creationdomain.StageRoadmap:
		kind = agentdomain.TaskStoryRoadmap
	}
	decoded, err := agentdomain.DecodeTaskOutput(kind, []byte(payload))
	if err != nil {
		return err
	}
	if stage == creationdomain.StageProposal {
		return decoded.(*agentdomain.StoryBlueprintOutput).ValidateProduct()
	}
	return nil
}

func currentCandidate(candidates []creationdomain.Candidate, stage creationdomain.Stage) (creationdomain.Candidate, bool) {
	var current creationdomain.Candidate
	found := false
	for _, candidate := range candidates {
		if candidate.Stage != stage || candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
			continue
		}
		if !found || candidate.Version > current.Version {
			current = candidate
			found = true
		}
	}
	return current, found
}

func candidateByID(candidates []creationdomain.Candidate, id string) (creationdomain.Candidate, bool) {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return creationdomain.Candidate{}, false
}

func hasLocks(candidate creationdomain.Candidate) bool {
	for _, locked := range candidate.LockMap {
		if locked {
			return true
		}
	}
	return false
}

func architectPayload(run agentdomain.Run, artifacts []agentdomain.Artifact) (string, string, error) {
	for _, step := range run.Steps {
		if step.Capability != agentdomain.CapabilityStoryArchitect || step.Status != agentdomain.StepCompleted {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.ID == step.PrimaryArtifactID && strings.TrimSpace(artifact.Content) != "" {
				return artifact.Content, artifact.ID, nil
			}
		}
	}
	return "", "", fmt.Errorf("%w: missing architect artifact", ErrUnavailable)
}

const defaultRunTimeout = 30 * time.Minute
