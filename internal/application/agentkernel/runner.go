package agentkernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/providers"
	"github.com/lishen802/novel-studio/internal/quality"
)

var (
	ErrCreateRun          = errors.New("create agent kernel run")
	ErrExecuteRun         = errors.New("execute agent kernel run")
	ErrRunNotExecutable   = errors.New("agent kernel run is not executable")
	ErrRunStale           = errors.New("agent kernel run canonical head changed")
	ErrTokenBudget        = errors.New("agent kernel token budget exceeded")
	ErrContextBudget      = errors.New("agent kernel context budget exceeded")
	ErrInvalidOutput      = errors.New("agent kernel structured output invalid")
	ErrInvalidReviewProof = errors.New("agent kernel review evidence invalid")
	ErrInvalidRevision    = errors.New("agent kernel revision boundary invalid")
	ErrQualityPolicy      = errors.New("agent kernel quality policy unavailable")
	ErrRuntimeUnavailable = errors.New("agent kernel runtime unavailable")
)

const (
	defaultRunTimeout          = 30 * time.Minute
	tokenCharRatio             = 4
	requestTokenSafetyOverhead = 1024 // 为协议包装与结构化输出描述预留空间。
)

// Store 是执行器实际消费的最小持久化端口。
type Store interface {
	CreateAgentRunBundle(context.Context, agentdomain.RunBundle) error
	AgentRun(context.Context, string) (agentdomain.Run, error)
	UpdateAgentRun(context.Context, agentdomain.Run, ...agentdomain.JobEventDraft) error
	UpdateAgentRunStep(context.Context, agentdomain.RunStep, ...agentdomain.JobEventDraft) error
	AppendAgentRunStep(context.Context, agentdomain.RunStep, ...agentdomain.JobEventDraft) error
	SaveAgentStepBundle(context.Context, agentdomain.StepBundle, ...agentdomain.JobEventDraft) error
	ListAgentArtifacts(context.Context, string) ([]agentdomain.Artifact, error)
}

// Source 描述创建运行时已经明确选择、可进入上下文的不可变输入。
type Source struct {
	Kind            agentdomain.ManifestSourceKind `json:"kind"`
	ID              string                         `json:"id"`
	Version         string                         `json:"version"`
	Content         string                         `json:"content"`
	Priority        int                            `json:"priority"`
	SelectionReason string                         `json:"selection_reason"`
	Provisional     bool                           `json:"provisional"`
	Required        bool                           `json:"required"`
}

// CreateRequest 包含运行创建阶段必须冻结的项目、工作流、方案与输入。
type CreateRequest struct {
	RunID               string
	ProjectID           string
	BranchID            string
	ChapterID           string
	ChapterBaseRevision int64
	CanonicalHead       string
	AuthorInstruction   string
	WorkflowID          string
	WorkflowVersion     int
	Profile             agentdomain.ResolvedCreativeProfile
	Sources             []Source
	QualitySpec         quality.Spec
	RevisionDirective   *agentdomain.RevisionDirective
	Deadline            time.Time
	RetryOf             string
}

type persistedInput struct {
	BranchID          string                         `json:"branch_id"`
	CanonicalHead     string                         `json:"canonical_head"`
	AuthorInstruction string                         `json:"author_instruction"`
	Sources           []Source                       `json:"sources"`
	QualitySpec       quality.Spec                   `json:"quality_spec"`
	RevisionDirective *agentdomain.RevisionDirective `json:"revision_directive,omitempty"`
	Deadline          time.Time                      `json:"deadline"`
}

// Service 负责版本化工作流的冻结、执行和可恢复重放。
type Service struct {
	store        Store
	credentials  ports.CredentialStore
	providers    ports.ModelProviderFactory
	registries   Registries
	clock        ports.Clock
	ids          ports.IDGenerator
	onJobUpdated func(context.Context, string)
	quality      *quality.Engine
}

// NewService 创建执行器；回调仅用于唤醒观察界面，不参与正确性。
func NewService(
	store Store,
	credentials ports.CredentialStore,
	providers ports.ModelProviderFactory,
	registries Registries,
	clock ports.Clock,
	ids ports.IDGenerator,
	onJobUpdated func(context.Context, string),
) *Service {
	if onJobUpdated == nil {
		onJobUpdated = func(context.Context, string) {}
	}
	qualityRegistry, qualityRegistryErr := quality.BuiltinRegistry()
	var qualityEngine *quality.Engine
	if qualityRegistryErr == nil {
		qualityEngine = quality.NewEngine(qualityRegistry)
	}
	return &Service{
		store: store, credentials: credentials, providers: providers, registries: registries,
		clock: clock, ids: ids, onJobUpdated: onJobUpdated, quality: qualityEngine,
	}
}

// CreateRun 编译并冻结全部节点，然后原子保存运行与原始任务输入。
func (service *Service) CreateRun(ctx context.Context, request CreateRequest) (agentdomain.Run, error) {
	if service.store == nil || service.clock == nil || service.ids == nil {
		return agentdomain.Run{}, fmt.Errorf("%w: missing dependencies", ErrRuntimeUnavailable)
	}
	compiled, err := Compile(ctx, service.registries, strings.TrimSpace(request.WorkflowID), request.WorkflowVersion)
	if err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrCreateRun, err)
	}
	if err := validateCreateRequest(request, compiled); err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrCreateRun, err)
	}
	if _, err := service.quality.Policy(compiled.QualityPolicyVersion); err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrCreateRun, ErrQualityPolicy)
	}
	qualitySpec, err := frozenQualitySpec(request, compiled.TaskKind)
	if err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrCreateRun, err)
	}
	deadline := request.Deadline.UTC()
	if deadline.IsZero() {
		deadline = service.clock.Now().UTC().Add(defaultRunTimeout)
	}
	payload := persistedInput{
		BranchID: strings.TrimSpace(request.BranchID), CanonicalHead: strings.TrimSpace(request.CanonicalHead),
		AuthorInstruction: strings.TrimSpace(request.AuthorInstruction), Sources: cloneSources(request.Sources),
		QualitySpec: qualitySpec, RevisionDirective: request.RevisionDirective, Deadline: deadline,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: encode input: %w", ErrCreateRun, err)
	}
	inputDigest := agentdomain.SHA256Hex(encoded)
	roles := make(map[agentdomain.Role]agentdomain.ResolvedRole, len(request.Profile.Roles))
	for _, role := range request.Profile.Roles {
		roles[role.Role] = role
	}
	specs := make([]agentdomain.RunStepSpec, 0, len(compiled.Nodes))
	for _, node := range compiled.Nodes {
		role, ok := roles[node.RoleSlot]
		if !ok {
			return agentdomain.Run{}, fmt.Errorf("%w: missing role %s", ErrCreateRun, node.RoleSlot)
		}
		maxOutputTokens := node.MaxOutputTokens
		outputLimitMode := role.OutputLimitMode.Normalized()
		if outputLimitMode.OmitsConfiguredMaxOutput() {
			maxOutputTokens = 0
		} else {
			maxOutputTokens = min(node.MaxOutputTokens, role.MaxOutputTokens)
		}
		temperature := role.Temperature
		if node.TemperatureOverride != nil {
			temperature = *node.TemperatureOverride
		}
		nodeDigest := digestStrings(inputDigest, node.ID, strings.Join(node.InputSelectors, "\x00"))
		specs = append(specs, agentdomain.RunStepSpec{
			ID: service.ids.NewID(), Role: node.RoleSlot,
			CreativeProfileID: request.Profile.ProfileID, CreativeProfileVersion: request.Profile.Version,
			ProviderConnectionID: role.ConnectionID, ProviderName: role.ProviderName,
			ProviderKind: role.ProviderKind, BaseURL: role.BaseURL, CredentialRef: role.CredentialRef,
			Model: role.Model, MaxOutputTokens: maxOutputTokens, ReasoningEffort: role.ReasoningEffort.Normalized(),
			OutputLimitMode: outputLimitMode, Temperature: temperature,
			NodeID: node.ID, PublicStageID: node.PublicStageID, Capability: node.Capability, PromptBinding: node.Prompt,
			SchemaBinding: node.Schema, AssemblerID: node.Assembler.ID,
			AssemblerVersion: node.Assembler.Version, InputDigest: nodeDigest,
		})
	}
	tokenBudget := agentdomain.EffectiveBudget(request.Profile.MaxRunTokens, request.Profile.RunBudgetUnlimited)
	contextBudget := agentdomain.EffectiveBudget(request.Profile.MaxContextChars, request.Profile.ContextBudgetUnlimited)
	if !request.Profile.RunBudgetUnlimited && compiled.RequiredTokenBudget() > tokenBudget {
		return agentdomain.Run{}, fmt.Errorf("%w: frozen node limits exceed %d", ErrTokenBudget, tokenBudget)
	}
	run, err := agentdomain.NewWorkflowRun(
		strings.TrimSpace(request.RunID), strings.TrimSpace(request.ProjectID), strings.TrimSpace(request.ChapterID),
		request.ChapterBaseRevision, request.AuthorInstruction, compiled.Workflow, compiled.TaskKind,
		request.CanonicalHead, inputDigest, compiled.BudgetPolicyVersion, compiled.QualityPolicyVersion,
		compiled.MaxRevisionAttempts, tokenBudget, contextBudget, specs, service.clock.Now(),
	)
	if err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrCreateRun, err)
	}
	run.TokenBudgetUnlimited = request.Profile.RunBudgetUnlimited
	run.ContextBudgetUnlimited = request.Profile.ContextBudgetUnlimited
	run.RetryOf = strings.TrimSpace(request.RetryOf)
	artifact := newArtifact(service.ids.NewID(), run.ID, "", agentdomain.ArtifactTaskInput, string(encoded), service.clock.Now())
	if err := service.store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run, Artifacts: []agentdomain.Artifact{artifact}}); err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: persist: %w", ErrCreateRun, err)
	}
	service.onJobUpdated(ctx, run.ID)
	return run, nil
}

// ExecuteRun 执行排队运行；模型调用期间不持有数据库事务或工作区锁。
func (service *Service) ExecuteRun(ctx context.Context, runID, canonicalHead string) (agentdomain.Run, error) {
	run, input, compiled, err := service.loadExecutable(ctx, runID, canonicalHead)
	if err != nil {
		return agentdomain.Run{}, err
	}
	if run.Status != agentdomain.RunQueued {
		return agentdomain.Run{}, fmt.Errorf("%w: status %s", ErrRunNotExecutable, run.Status)
	}
	return service.execute(ctx, run, input, compiled, false)
}

// ResumeRun 从 recoverable 状态追加中断节点的新 attempt，并跳过已完成节点。
func (service *Service) ResumeRun(ctx context.Context, runID, canonicalHead string) (agentdomain.Run, error) {
	run, input, compiled, err := service.loadExecutable(ctx, runID, canonicalHead)
	if err != nil {
		return agentdomain.Run{}, err
	}
	if run.Status != agentdomain.RunRecoverable {
		return agentdomain.Run{}, fmt.Errorf("%w: status %s", ErrRunNotExecutable, run.Status)
	}
	latest := latestSteps(run.Steps)
	for _, node := range compiled.Nodes {
		step, ok := latest[node.ID]
		if !ok || step.Status != agentdomain.StepFailed || step.ErrorCode != "app_interrupted" {
			continue
		}
		next := step
		next.ID = service.ids.NewID()
		next.Attempt++
		next.Status = agentdomain.StepQueued
		next.Output = ""
		next.ErrorCode = ""
		next.ErrorMessage = ""
		next.ProviderRequestID = ""
		next.InputTokens = 0
		next.OutputTokens = 0
		next.StartedAt = nil
		next.CompletedAt = nil
		next.OutputDigest = ""
		next.PrimaryArtifactID = ""
		next.QualityStatus = ""
		next.CreatedAt = service.clock.Now().UTC()
		next.UpdatedAt = next.CreatedAt
		if err := service.store.AppendAgentRunStep(ctx, next); err != nil {
			return agentdomain.Run{}, fmt.Errorf("%w: append attempt: %w", ErrExecuteRun, err)
		}
		run.Steps = append(run.Steps, next)
	}
	return service.execute(ctx, run, input, compiled, true)
}

func (service *Service) execute(
	ctx context.Context,
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	resumed bool,
) (agentdomain.Run, error) {
	if service.credentials == nil || service.providers == nil {
		return agentdomain.Run{}, ErrRuntimeUnavailable
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return service.cancelRun(context.WithoutCancel(ctx), run)
		}
		return service.failRun(context.WithoutCancel(ctx), run, err)
	}
	if !input.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, input.Deadline)
		defer cancel()
	}
	running, err := run.Transition(agentdomain.RunRunning, "", service.clock.Now())
	if err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: %w", ErrExecuteRun, err)
	}
	started, err := jobDraft(running, agentdomain.EventJobStarted, "", map[string]any{"resumed": resumed}, service.clock.Now())
	if err != nil {
		return agentdomain.Run{}, err
	}
	if err := service.store.UpdateAgentRun(ctx, running, started); err != nil {
		return agentdomain.Run{}, fmt.Errorf("%w: start: %w", ErrExecuteRun, err)
	}
	service.onJobUpdated(ctx, running.ID)
	run = running
	artifacts, err := service.store.ListAgentArtifacts(ctx, run.ID)
	if err != nil {
		return service.failRun(ctx, run, err)
	}
	latest := latestSteps(run.Steps)
	for _, node := range compiled.Nodes {
		step, ok := latest[node.ID]
		if !ok {
			return service.failRun(ctx, run, fmt.Errorf("missing frozen node %s", node.ID))
		}
		if step.Status == agentdomain.StepCompleted {
			continue
		}
		if step.Status != agentdomain.StepQueued {
			if node.FailurePolicy == agentdomain.FailureSkipNode && step.Status == agentdomain.StepFailed {
				continue
			}
			return service.failRun(ctx, run, fmt.Errorf("node %s is %s", node.ID, step.Status))
		}
		if !shouldRunNode(node.Condition, latest) {
			step, newArtifacts, err := service.persistSkipped(ctx, run, step)
			if err != nil {
				return service.failRun(ctx, run, err)
			}
			latest[node.ID] = step
			run = replaceStep(run, step)
			artifacts = append(artifacts, newArtifacts...)
			continue
		}
		step, produced, execErr := service.executeNode(ctx, run, input, compiled, node, step, latest, artifacts)
		latest[node.ID] = step
		artifacts = append(artifacts, produced...)
		run = replaceStep(run, step)
		run.InputTokens += step.InputTokens
		run.OutputTokens += step.OutputTokens
		if updateErr := service.store.UpdateAgentRun(context.WithoutCancel(ctx), run); updateErr != nil {
			return run, fmt.Errorf("%w: refresh usage: %w", ErrExecuteRun, updateErr)
		}
		if execErr != nil {
			if errors.Is(execErr, context.Canceled) {
				return service.cancelRun(context.WithoutCancel(ctx), run)
			}
			if node.FailurePolicy == agentdomain.FailureSkipNode {
				continue
			}
			return service.failRun(context.WithoutCancel(ctx), run, execErr)
		}
		if run.RemainingTokens() == 0 && node.ID != compiled.ResultNodeID {
			return service.failRun(context.WithoutCancel(ctx), run, ErrTokenBudget)
		}
	}
	run.QualityStatus = aggregateQuality(compiled, latest, artifacts)
	completed, err := run.Transition(agentdomain.RunCompleted, "", service.clock.Now())
	if err != nil {
		return service.failRun(ctx, run, err)
	}
	draft, err := jobDraft(completed, agentdomain.EventJobCompleted, "", map[string]any{"quality_status": completed.QualityStatus}, service.clock.Now())
	if err != nil {
		return run, err
	}
	if err := service.store.UpdateAgentRun(context.WithoutCancel(ctx), completed, draft); err != nil {
		return run, fmt.Errorf("%w: complete: %w", ErrExecuteRun, err)
	}
	service.onJobUpdated(context.WithoutCancel(ctx), completed.ID)
	return completed, nil
}

func (service *Service) executeNode(
	ctx context.Context,
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) (agentdomain.RunStep, []agentdomain.Artifact, error) {
	prompt, err := service.registries.Prompts.Prompt(ctx, node.Prompt.ID, node.Prompt.Version)
	if err != nil {
		return service.failStep(ctx, run, step, "asset_missing", err, nil)
	}
	schema, err := service.registries.Schemas.Schema(ctx, node.Schema.ID, node.Schema.Version)
	if err != nil {
		return service.failStep(ctx, run, step, "asset_missing", err, nil)
	}
	assembled, manifest, refs, err := service.assemble(run, input, node, step, latest, artifacts)
	if err != nil {
		return service.failStep(ctx, run, step, errorCode(err), err, nil)
	}
	step.InputDigest = digestStrings(refs...)
	envelope := taskEnvelope(run, input, compiled, node, step, manifest, refs)
	if err := envelope.Validate(); err != nil {
		return service.failStep(ctx, run, step, "invalid_envelope", err, nil)
	}
	now := service.clock.Now().UTC()
	step.Status = agentdomain.StepRunning
	step.StartedAt = &now
	step.UpdatedAt = now
	started, _ := jobDraft(run, agentdomain.EventStepStarted, step.ID, map[string]any{
		"node_id": node.ID, "public_stage_id": node.PublicStageID, "attempt": step.Attempt, "capability": step.Capability,
	}, now)
	if err := service.store.SaveAgentStepBundle(ctx, agentdomain.StepBundle{Step: step, Manifests: []agentdomain.ContextManifest{manifest}}, started); err != nil {
		return step, nil, err
	}
	service.onJobUpdated(ctx, run.ID)
	remaining := run.RemainingTokens()
	estimatedInput := max(1, utf8.RuneCountInString(assembled)/tokenCharRatio)
	reserve := compiled.RepairReserve + compiled.RevisionReserve
	needed := estimatedInput + reserve
	if !step.OutputLimitMode.OmitsConfiguredMaxOutput() {
		needed += step.MaxOutputTokens
	}
	if needed > remaining {
		return service.failStep(ctx, run, step, "token_budget_exceeded", ErrTokenBudget, nil)
	}
	apiKey, err := service.credentials.Get(ctx, step.CredentialRef)
	if err != nil {
		return service.failStep(ctx, run, step, "credential_unavailable", ErrRuntimeUnavailable, nil)
	}
	provider, err := service.providers.New(agentdomain.ProviderConnection{
		ID: step.ProviderConnectionID, ProjectID: run.ProjectID, Name: step.ProviderName,
		Kind: step.ProviderKind, BaseURL: step.BaseURL, CredentialRef: step.CredentialRef,
	})
	if err != nil {
		return service.failStep(ctx, run, step, "provider_unavailable", ErrRuntimeUnavailable, nil)
	}
	rendered, err := renderPrompt(prompt, input, node, latest, artifacts, assembled)
	if err != nil {
		return service.failStep(ctx, run, step, "prompt_render_failed", err, nil)
	}
	// 有限预算必须在发起计费调用前动态扣除输入与预留；字节数是 tokenizer
	// 未知时的保守输入上界，避免事后才发现超出总预算。
	availableOutput := remaining
	if !run.TokenBudgetUnlimited {
		availableOutput -= len(rendered) + len(capabilityInstruction(step.Capability)) + len(schema.JSON) + reserve + requestTokenSafetyOverhead
		if availableOutput < 1 {
			return service.failStep(ctx, run, step, "token_budget_exceeded", ErrTokenBudget, nil)
		}
	}
	generated, err := provider.Generate(ctx, apiKey, generateRequest(step, availableOutput, run.TokenBudgetUnlimited, capabilityInstruction(step.Capability), rendered, schema, node.Schema.ID), nil)
	step.InputTokens = generated.InputTokens
	step.OutputTokens = generated.OutputTokens
	step.ProviderRequestID = generated.ProviderRequestID
	if err != nil {
		return service.failStep(ctx, run, step, errorCode(err), err, nil)
	}
	if step.InputTokens+step.OutputTokens > remaining {
		return service.failStep(ctx, run, step, "token_budget_exceeded", ErrTokenBudget, nil)
	}
	outputKind := taskKindForNode(compiled.TaskKind, node.Capability)
	decodeOutput := func(content string) (agentdomain.TaskOutput, error) {
		return decodeNodeOutput(outputKind, node, latest, artifacts, content)
	}
	decoded, decodeErr := decodeOutput(generated.Text)
	produced := make([]agentdomain.Artifact, 0, 3)
	if decodeErr != nil {
		var repairInputTokens, repairOutputTokens int
		var repairRequestID string
		decoded, generated.Text, produced, repairInputTokens, repairOutputTokens, repairRequestID, decodeErr = service.repairOutput(
			ctx, run, step, node, schema, generated.Text, generated.ProviderRequestID, decoded, produced, decodeOutput,
			min(compiled.RepairReserve, remaining-step.InputTokens-step.OutputTokens),
		)
		step.InputTokens += repairInputTokens
		step.OutputTokens += repairOutputTokens
		if repairRequestID != "" {
			step.ProviderRequestID = repairRequestID
		}
		if step.InputTokens+step.OutputTokens > remaining {
			return service.failStep(ctx, run, step, "token_budget_exceeded", ErrTokenBudget, produced)
		}
		if decodeErr != nil {
			return service.failStep(ctx, run, step, "invalid_structured_output", fmt.Errorf("%w: %v", ErrInvalidOutput, decodeErr), produced)
		}
	}
	if validateErr := validateCapabilityOutput(decoded, node, input, latest, artifacts); validateErr != nil {
		code, cause := capabilityValidationError(node.Capability, validateErr)
		return service.failStep(ctx, run, step, code, cause, produced)
	}
	normalizedOutput, err := json.Marshal(decoded)
	if err != nil {
		return service.failStep(ctx, run, step, "invalid_structured_output", fmt.Errorf("%w: encode normalized output", ErrInvalidOutput), produced)
	}
	structured := newArtifact(service.ids.NewID(), run.ID, step.ID, agentdomain.ArtifactStructuredOutput, string(normalizedOutput), service.clock.Now())
	primaryContent, quality, findings := outputDetails(decoded, run, step, service.clock.Now())
	primaryContent, quality, findings, qualityArtifacts, qualityErr := service.applyQuality(
		run, input, compiled, node, step, latest, artifacts, decoded, primaryContent, quality, findings,
	)
	if qualityErr != nil {
		code, cause := errorCode(qualityErr), qualityErr
		if code == "invalid_structured_output" {
			code, cause = capabilityValidationError(node.Capability, qualityErr)
		}
		return service.failStep(ctx, run, step, code, cause, produced)
	}
	primaryKind := artifactKind(node.Capability)
	primary := newArtifact(service.ids.NewID(), run.ID, step.ID, primaryKind, primaryContent, service.clock.Now())
	relations := make([]agentdomain.ArtifactRelationRecord, 0, 1)
	if node.Capability == agentdomain.CapabilityBoundedRevision {
		base, baseErr := revisionBaseArtifact(node, latest, artifacts)
		if baseErr != nil {
			return service.failStep(ctx, run, step, "revision_base_missing", baseErr, produced)
		}
		relations = append(relations, agentdomain.ArtifactRelationRecord{
			ArtifactID: primary.ID, Relation: agentdomain.ArtifactRevises,
			RelatedArtifactID: base.ID, BaseDigest: base.SHA256, CreatedAt: service.clock.Now().UTC(),
		})
	}
	produced = append(produced, structured, primary)
	produced = append(produced, qualityArtifacts...)
	completedAt := service.clock.Now().UTC()
	step.Status = agentdomain.StepCompleted
	step.Output = agentdomain.BoundedStepPreview(primaryContent)
	step.OutputDigest = agentdomain.SHA256Hex(normalizedOutput)
	step.PrimaryArtifactID = primary.ID
	step.QualityStatus = quality
	step.CompletedAt = &completedAt
	step.UpdatedAt = completedAt
	completed, _ := jobDraft(run, agentdomain.EventStepCompleted, step.ID, map[string]any{
		"node_id": node.ID, "public_stage_id": node.PublicStageID, "attempt": step.Attempt, "input_tokens": step.InputTokens,
		"output_tokens": step.OutputTokens, "quality_status": step.QualityStatus,
	}, completedAt)
	if err := service.store.SaveAgentStepBundle(context.WithoutCancel(ctx), agentdomain.StepBundle{
		Step: step, Artifacts: produced, Relations: relations, Findings: findings,
	}, completed); err != nil {
		return step, produced, err
	}
	service.onJobUpdated(context.WithoutCancel(ctx), run.ID)
	return step, produced, nil
}

func revisionBaseArtifact(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (agentdomain.Artifact, error) {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || step.PrimaryArtifactID == "" {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.ID == step.PrimaryArtifactID && (artifact.Kind == agentdomain.ArtifactDraft || artifact.Kind == agentdomain.ArtifactPlan) {
				return artifact, nil
			}
		}
	}
	return agentdomain.Artifact{}, errors.New("revision base artifact not found")
}

func (service *Service) repairOutput(
	ctx context.Context,
	run agentdomain.Run,
	step agentdomain.RunStep,
	node FrozenNode,
	schema ports.SchemaAsset,
	original string,
	originalRequestID string,
	decoded agentdomain.TaskOutput,
	produced []agentdomain.Artifact,
	decodeOutput func(string) (agentdomain.TaskOutput, error),
	remaining int,
) (agentdomain.TaskOutput, string, []agentdomain.Artifact, int, int, string, error) {
	if decoded != nil || remaining < 1 {
		return decoded, original, produced, 0, 0, "", ErrInvalidOutput
	}
	availableOutput := remaining
	if !run.TokenBudgetUnlimited {
		// 修复调用已经受独立 RepairReserve 约束，仅扣除待修复正文，避免结构
		// 描述的保守估算吞掉整个小额修复预留。
		availableOutput -= len(original)
	}
	maxOutput := protocolMaxOutput(step, availableOutput, run.TokenBudgetUnlimited)
	omitMax := step.OutputLimitMode.OmitsConfiguredMaxOutput() && step.ProviderKind == agentdomain.ProviderOpenAI
	if availableOutput < 1 || (!omitMax && maxOutput < 1) {
		return nil, original, produced, 0, 0, "", ErrTokenBudget
	}
	apiKey, err := service.credentials.Get(ctx, step.CredentialRef)
	if err != nil {
		return nil, original, produced, 0, 0, "", err
	}
	provider, err := service.providers.New(agentdomain.ProviderConnection{
		ID: step.ProviderConnectionID, ProjectID: run.ProjectID, Name: step.ProviderName,
		Kind: step.ProviderKind, BaseURL: step.BaseURL, CredentialRef: step.CredentialRef,
	})
	if err != nil {
		return nil, original, produced, 0, 0, "", err
	}
	repairStep := step
	repairStep.Temperature = 0
	repairStep.ReasoningEffort = agentdomain.ReasoningNone
	repaired, err := provider.Generate(ctx, apiKey, generateRequest(
		repairStep, availableOutput, false,
		"你只负责修复 JSON 结构，使其符合给定 Schema；不得增加事实、删改正文或改变语义。",
		original, schema, node.Schema.ID,
	), nil)
	if err != nil {
		return nil, original, produced, repaired.InputTokens, repaired.OutputTokens, repaired.ProviderRequestID, err
	}
	artifactContent, _ := json.Marshal(map[string]string{
		"before": original, "after": repaired.Text,
		"initial_provider_request_id": originalRequestID, "repair_provider_request_id": repaired.ProviderRequestID,
	})
	produced = append(produced, newArtifact(service.ids.NewID(), run.ID, step.ID, agentdomain.ArtifactFormatRepair, string(artifactContent), service.clock.Now()))
	decoded, err = decodeOutput(repaired.Text)
	return decoded, repaired.Text, produced, repaired.InputTokens, repaired.OutputTokens, repaired.ProviderRequestID, err
}

func (service *Service) assemble(
	run agentdomain.Run,
	input persistedInput,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) (string, agentdomain.ContextManifest, []string, error) {
	type part struct {
		source  Source
		content string
	}
	parts := make([]part, 0)
	for _, selector := range node.InputSelectors {
		switch {
		case selector == "$task":
			parts = append(parts, part{source: Source{
				Kind: agentdomain.ManifestAuthorInstruction, ID: "author-instruction", Version: "1",
				Priority: 1000, SelectionReason: "当前任务指令", Required: true,
			}, content: input.AuthorInstruction})
		case selector == "$context":
			for _, source := range input.Sources {
				parts = append(parts, part{source: source, content: source.Content})
			}
		case strings.HasPrefix(selector, "node:"):
			nodeID := strings.TrimPrefix(selector, "node:")
			upstream, ok := latest[nodeID]
			if !ok || upstream.Status != agentdomain.StepCompleted {
				return "", agentdomain.ContextManifest{}, nil, fmt.Errorf("missing upstream node %s", nodeID)
			}
			artifact, ok := artifactByID(artifacts, upstream.PrimaryArtifactID)
			if !ok {
				return "", agentdomain.ContextManifest{}, nil, fmt.Errorf("missing upstream artifact %s", upstream.PrimaryArtifactID)
			}
			parts = append(parts, part{source: Source{
				Kind: agentdomain.ManifestUpstreamArtifact, ID: artifact.ID, Version: fmt.Sprintf("attempt-%d", upstream.Attempt),
				Priority: 800, SelectionReason: "工作流上游节点输出", Required: true,
			}, content: artifact.Content})
		default:
			return "", agentdomain.ContextManifest{}, nil, fmt.Errorf("unsupported selector %s", selector)
		}
	}
	manifestID := service.ids.NewID()
	remaining := run.RemainingContextChars()
	selected := make([]string, 0, len(parts))
	refs := make([]string, 0, len(parts))
	items := make([]agentdomain.ContextManifestItem, 0, len(parts))
	truncated := false
	for index, value := range parts {
		content := strings.TrimSpace(value.content)
		characters := utf8.RuneCountInString(content)
		if len(selected) > 0 {
			characters += 2
		}
		included := characters <= remaining
		omission := ""
		if !included {
			if value.source.Required {
				return "", agentdomain.ContextManifest{}, nil, ErrContextBudget
			}
			omission = "上下文字符预算不足"
			truncated = true
		} else {
			remaining -= characters
			selected = append(selected, content)
			refs = append(refs, value.source.ID+"@"+value.source.Version+":"+agentdomain.SHA256Hex([]byte(content)))
		}
		items = append(items, agentdomain.ContextManifestItem{
			ManifestID: manifestID, SourceKind: value.source.Kind, SourceID: value.source.ID,
			SourceVersion: value.source.Version, ContentHash: agentdomain.SHA256Hex([]byte(content)),
			Position: index + 1, Priority: value.source.Priority, SelectionReason: value.source.SelectionReason,
			Provisional: value.source.Provisional, Included: included, OmissionReason: omission,
		})
	}
	assembled := strings.Join(selected, "\n\n")
	manifest := agentdomain.ContextManifest{
		ID: manifestID, Version: 1, RunID: run.ID, StepID: step.ID, Attempt: step.Attempt,
		CanonicalHead: run.CanonicalHead, AssemblerID: node.Assembler.ID, AssemblerVersion: node.Assembler.Version,
		CharacterBudget: run.ContextCharBudget, EstimatedTokenBudget: max(1, utf8.RuneCountInString(assembled)/tokenCharRatio),
		UsedChars: utf8.RuneCountInString(assembled), Truncated: truncated,
		AssembledContentHash: agentdomain.SHA256Hex([]byte(assembled)), Items: items, CreatedAt: service.clock.Now().UTC(),
	}
	if len(refs) == 0 || manifest.Validate() != nil {
		return "", agentdomain.ContextManifest{}, nil, ErrContextBudget
	}
	return assembled, manifest, refs, nil
}

func (service *Service) loadExecutable(
	ctx context.Context,
	runID, canonicalHead string,
) (agentdomain.Run, persistedInput, CompiledWorkflow, error) {
	run, err := service.store.AgentRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, fmt.Errorf("%w: load: %w", ErrExecuteRun, err)
	}
	if run.IsLegacy() {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, fmt.Errorf("%w: legacy run requires workspace adapter", ErrRunNotExecutable)
	}
	if strings.TrimSpace(canonicalHead) != run.CanonicalHead {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, ErrRunStale
	}
	compiled, err := Compile(ctx, service.registries, run.WorkflowBinding.ID, run.WorkflowBinding.Version)
	if err != nil || compiled.Workflow != run.WorkflowBinding {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, fmt.Errorf("%w: frozen workflow unavailable", ErrExecuteRun)
	}
	artifacts, err := service.store.ListAgentArtifacts(ctx, run.ID)
	if err != nil {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, err
	}
	var input persistedInput
	found := false
	for _, artifact := range artifacts {
		if artifact.Kind == agentdomain.ArtifactTaskInput {
			if artifact.SHA256 != run.InputDigest || json.Unmarshal([]byte(artifact.Content), &input) != nil {
				return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, fmt.Errorf("%w: task input digest mismatch", ErrRunStale)
			}
			found = true
			break
		}
	}
	if !found || input.CanonicalHead != run.CanonicalHead {
		return agentdomain.Run{}, persistedInput{}, CompiledWorkflow{}, fmt.Errorf("%w: task input missing", ErrRunStale)
	}
	return run, input, compiled, nil
}

func (service *Service) persistSkipped(
	ctx context.Context,
	run agentdomain.Run,
	step agentdomain.RunStep,
) (agentdomain.RunStep, []agentdomain.Artifact, error) {
	content := `{"skipped":true,"reason":"condition_not_met"}`
	artifact := newArtifact(service.ids.NewID(), run.ID, step.ID, agentdomain.ArtifactQualityReport, content, service.clock.Now())
	now := service.clock.Now().UTC()
	step.Status = agentdomain.StepCompleted
	step.Output = "节点条件未满足，已跳过。"
	step.OutputDigest = artifact.SHA256
	step.PrimaryArtifactID = artifact.ID
	step.QualityStatus = agentdomain.QualityPassed
	step.CompletedAt = &now
	step.UpdatedAt = now
	draft, _ := jobDraft(run, agentdomain.EventStepCompleted, step.ID, map[string]any{"node_id": step.NodeID, "public_stage_id": step.PublicStageID, "skipped": true}, now)
	err := service.store.SaveAgentStepBundle(ctx, agentdomain.StepBundle{Step: step, Artifacts: []agentdomain.Artifact{artifact}}, draft)
	if err == nil {
		service.onJobUpdated(ctx, run.ID)
	}
	return step, []agentdomain.Artifact{artifact}, err
}

func (service *Service) failStep(
	ctx context.Context,
	run agentdomain.Run,
	step agentdomain.RunStep,
	code string,
	cause error,
	artifacts []agentdomain.Artifact,
) (agentdomain.RunStep, []agentdomain.Artifact, error) {
	now := service.clock.Now().UTC()
	step.Status = agentdomain.StepFailed
	if errors.Is(cause, context.Canceled) {
		step.Status = agentdomain.StepCancelled
	}
	step.ErrorCode = code
	step.ErrorMessage = safeErrorMessage(code)
	step.QualityStatus = agentdomain.QualityBlocked
	step.CompletedAt = &now
	step.UpdatedAt = now
	draft, _ := jobDraft(run, agentdomain.EventStepFailed, step.ID, map[string]any{"error_code": code, "public_stage_id": step.PublicStageID, "attempt": step.Attempt}, now)
	if err := service.store.SaveAgentStepBundle(context.WithoutCancel(ctx), agentdomain.StepBundle{Step: step, Artifacts: artifacts}, draft); err != nil {
		return step, artifacts, errors.Join(cause, err)
	}
	service.onJobUpdated(context.WithoutCancel(ctx), run.ID)
	return step, artifacts, cause
}

func (service *Service) failRun(ctx context.Context, run agentdomain.Run, cause error) (agentdomain.Run, error) {
	if errors.Is(cause, context.Canceled) {
		return service.cancelRun(context.WithoutCancel(ctx), run)
	}
	code := errorCode(cause)
	run.ErrorCode = code
	run.QualityStatus = agentdomain.QualityBlocked
	failed, err := run.Transition(agentdomain.RunFailed, safeErrorMessage(code), service.clock.Now())
	if err != nil {
		return run, errors.Join(cause, err)
	}
	draft, _ := jobDraft(failed, agentdomain.EventJobFailed, "", map[string]any{"error_code": code}, service.clock.Now())
	if err := service.store.UpdateAgentRun(ctx, failed, draft); err != nil {
		return failed, errors.Join(cause, err)
	}
	service.onJobUpdated(ctx, failed.ID)
	return failed, cause
}

func (service *Service) cancelRun(ctx context.Context, run agentdomain.Run) (agentdomain.Run, error) {
	now := service.clock.Now().UTC()
	for _, step := range latestSteps(run.Steps) {
		if step.Status == agentdomain.StepCompleted || step.Status == agentdomain.StepFailed || step.Status == agentdomain.StepCancelled {
			continue
		}
		step.Status = agentdomain.StepCancelled
		step.ErrorCode = "cancelled"
		step.ErrorMessage = safeErrorMessage("cancelled")
		step.QualityStatus = agentdomain.QualityBlocked
		step.CompletedAt = &now
		step.UpdatedAt = now
		draft, _ := jobDraft(run, agentdomain.EventStepFailed, step.ID, map[string]any{"reason": "cancelled"}, now)
		if err := service.store.UpdateAgentRunStep(ctx, step, draft); err != nil {
			return run, err
		}
		run = replaceStep(run, step)
	}
	run.ErrorCode = "cancelled"
	run.QualityStatus = agentdomain.QualityBlocked
	cancelled, err := run.Transition(agentdomain.RunCancelled, safeErrorMessage("cancelled"), now)
	if err != nil {
		return run, err
	}
	draft, _ := jobDraft(cancelled, agentdomain.EventJobCancelled, "", map[string]any{"reason": "cancelled"}, now)
	if err := service.store.UpdateAgentRun(ctx, cancelled, draft); err != nil {
		return cancelled, err
	}
	service.onJobUpdated(ctx, cancelled.ID)
	return cancelled, context.Canceled
}

func validateCreateRequest(request CreateRequest, compiled CompiledWorkflow) error {
	if strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.ProjectID) == "" ||
		strings.TrimSpace(request.BranchID) == "" || strings.TrimSpace(request.ChapterID) == "" ||
		request.ChapterBaseRevision < 1 || strings.TrimSpace(request.CanonicalHead) == "" ||
		strings.TrimSpace(request.AuthorInstruction) == "" || strings.TrimSpace(request.Profile.ProfileID) == "" ||
		request.Profile.Version < 1 ||
		(!request.Profile.RunBudgetUnlimited && request.Profile.MaxRunTokens < 1) ||
		(!request.Profile.ContextBudgetUnlimited && request.Profile.MaxContextChars < 1) {
		return agentdomain.ErrInvalidRun
	}
	if request.QualitySpec.Validate() != nil {
		return quality.ErrInvalidInput
	}
	if request.RevisionDirective != nil {
		if request.RevisionDirective.Validate() != nil || request.RevisionDirective.Mode != agentdomain.RevisionAuthorDirected ||
			(compiled.TaskKind != agentdomain.TaskReviseDraft && compiled.TaskKind != agentdomain.TaskReviseBlueprint) {
			return agentdomain.ErrInvalidRevisionDirective
		}
	} else if compiled.TaskKind == agentdomain.TaskReviseDraft || compiled.TaskKind == agentdomain.TaskReviseBlueprint {
		return agentdomain.ErrInvalidRevisionDirective
	}
	for _, source := range request.Sources {
		if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.Version) == "" ||
			strings.TrimSpace(source.Content) == "" || strings.TrimSpace(source.SelectionReason) == "" ||
			source.Priority < 0 || source.Priority > 1000 || !validSourceKind(source.Kind) {
			return agentdomain.ErrInvalidRun
		}
	}
	seenSources := make(map[string]struct{}, len(request.Sources))
	for _, source := range request.Sources {
		key := strings.TrimSpace(source.ID) + "\x00" + strings.TrimSpace(source.Version)
		if _, exists := seenSources[key]; exists {
			return agentdomain.ErrInvalidRun
		}
		seenSources[key] = struct{}{}
	}
	roles := make(map[agentdomain.Role]agentdomain.ResolvedRole, len(request.Profile.Roles))
	for _, role := range request.Profile.Roles {
		if _, exists := roles[role.Role]; exists {
			return agentdomain.ErrInvalidProfile
		}
		roles[role.Role] = role
	}
	writer, hasWriter := roles[agentdomain.RoleWriter]
	reviewer, hasReviewer := roles[agentdomain.RoleContinuityReviewer]
	if hasWriter && hasReviewer && writer.ConnectionID == reviewer.ConnectionID && writer.Model == reviewer.Model {
		return agentdomain.ErrReviewerNotIndependent
	}
	for _, node := range compiled.Nodes {
		role, ok := roles[node.RoleSlot]
		binding := agentdomain.ExecutionBinding{
			RoleSlot: node.RoleSlot, ProviderConnectionID: role.ConnectionID, ProviderName: role.ProviderName,
			ProviderKind: role.ProviderKind, BaseURL: role.BaseURL, CredentialRef: role.CredentialRef,
			Model: role.Model, MaxOutputTokens: role.MaxOutputTokens, ReasoningEffort: role.ReasoningEffort,
			OutputLimitMode: role.OutputLimitMode, Temperature: role.Temperature,
		}
		if !ok || binding.Validate(node.Capability) != nil {
			return agentdomain.ErrInvalidProfile
		}
	}
	return nil
}

func validSourceKind(kind agentdomain.ManifestSourceKind) bool {
	switch kind {
	case agentdomain.ManifestHarnessSpec, agentdomain.ManifestAuthorInstruction,
		agentdomain.ManifestChapterRevision, agentdomain.ManifestStoryAsset,
		agentdomain.ManifestUpstreamArtifact, agentdomain.ManifestAnchor:
		return true
	default:
		return false
	}
}

func taskEnvelope(
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	node FrozenNode,
	step agentdomain.RunStep,
	manifest agentdomain.ContextManifest,
	refs []string,
) agentdomain.TaskEnvelope {
	return agentdomain.TaskEnvelope{
		TaskID: run.ID + ":" + node.ID, RunID: run.ID, StepID: step.ID, Attempt: step.Attempt,
		TaskKind: taskKindForNode(compiled.TaskKind, node.Capability), Capability: node.Capability,
		Workflow: compiled.Workflow, ProjectID: run.ProjectID, BranchID: input.BranchID,
		CanonicalVersionID: run.CanonicalHead, AuthorInstructionRef: "author-instruction",
		InputRefs: refs, InputDigest: step.InputDigest, ContextManifestID: manifest.ID,
		Execution: agentdomain.ExecutionBinding{
			RoleSlot: step.Role, ProviderConnectionID: step.ProviderConnectionID, ProviderName: step.ProviderName,
			ProviderKind: step.ProviderKind, BaseURL: step.BaseURL, CredentialRef: step.CredentialRef,
			Model: step.Model, MaxOutputTokens: step.MaxOutputTokens, ReasoningEffort: step.ReasoningEffort,
			OutputLimitMode: step.OutputLimitMode, Temperature: step.Temperature,
		},
		Prompt: node.Prompt, Schema: node.Schema, Assembler: node.Assembler,
		Budget: agentdomain.TaskBudget{
			MaxRunTokens: run.TokenBudget, RemainingTokens: run.RemainingTokens(),
			RepairReserve: compiled.RepairReserve, RevisionReserve: compiled.RevisionReserve,
			MaxContextChars: run.ContextCharBudget,
		},
		Deadline: input.Deadline,
	}
}

func renderPrompt(
	asset ports.PromptAsset,
	input persistedInput,
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	assembled string,
) (string, error) {
	values := make(map[string]string, len(asset.Variables))
	upstream := make([]string, 0)
	reviewContents := make([]string, 0)
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		id := strings.TrimPrefix(selector, "node:")
		step, ok := latest[id]
		if !ok {
			continue
		}
		artifact, ok := artifactByID(artifacts, step.PrimaryArtifactID)
		if ok {
			upstream = append(upstream, artifact.Content)
			values[exportedVariable(id)] = artifact.Content
			if step.Capability == agentdomain.CapabilityContinuityReview || step.Capability == agentdomain.CapabilityContentReview || step.Capability == agentdomain.CapabilityStyleReview {
				reviewContents = append(reviewContents, artifact.Content)
			}
		}
	}
	for _, variable := range asset.Variables {
		switch variable {
		case "Task":
			values[variable] = input.AuthorInstruction
		case "Context":
			values[variable] = assembled
		default:
			if variable == "Review" && len(reviewContents) > 1 {
				merged, mergeErr := mergeReviewContents(reviewContents)
				if mergeErr != nil {
					return "", mergeErr
				}
				values[variable] = merged
			}
			if values[variable] == "" && len(upstream) == 1 {
				values[variable] = upstream[0]
			}
		}
		if strings.TrimSpace(values[variable]) == "" {
			return "", fmt.Errorf("prompt variable %s has no input", variable)
		}
	}
	parsed, err := template.New(asset.Binding.ID).Option("missingkey=error").Parse(asset.Template)
	if err != nil {
		return "", err
	}
	var rendered strings.Builder
	if err := parsed.Execute(&rendered, values); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func outputDetails(
	output agentdomain.TaskOutput,
	run agentdomain.Run,
	step agentdomain.RunStep,
	now time.Time,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord) {
	quality := agentdomain.QualityPassed
	var content string
	var findings []agentdomain.ReviewFindingRecord
	switch value := output.(type) {
	case *agentdomain.ChapterDraftOutput:
		content = value.Body
	case *agentdomain.ReviseDraftOutput:
		content = value.Body
	case *agentdomain.ReviseBlueprintOutput:
		encoded, _ := json.Marshal(value.Blueprint)
		content = string(encoded)
	case *agentdomain.ReviewBundleOutput:
		quality = value.QualityStatus
		encoded, _ := json.Marshal(value)
		content = string(encoded)
		findings = make([]agentdomain.ReviewFindingRecord, 0, len(value.Findings))
		for _, finding := range value.Findings {
			findings = append(findings, agentdomain.ReviewFindingRecord{
				ReviewFinding: finding, RunID: run.ID, StepID: step.ID, Attempt: step.Attempt,
				CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
			})
		}
	default:
		encoded, _ := json.Marshal(value)
		content = string(encoded)
	}
	return content, quality, findings
}

func taskKindForNode(workflowKind agentdomain.TaskKind, capability agentdomain.Capability) agentdomain.TaskKind {
	switch capability {
	case agentdomain.CapabilityStoryArchitect:
		return agentdomain.TaskStoryBlueprint
	case agentdomain.CapabilityChapterPlanner:
		return agentdomain.TaskChapterPlan
	case agentdomain.CapabilityProseWriter:
		if workflowKind == agentdomain.TaskBaselineDraft {
			return agentdomain.TaskBaselineDraft
		}
		return agentdomain.TaskChapterDraft
	case agentdomain.CapabilityContinuityReview, agentdomain.CapabilityContentReview, agentdomain.CapabilityStyleReview:
		return agentdomain.TaskReviewBundle
	case agentdomain.CapabilityBoundedRevision:
		if workflowKind == agentdomain.TaskStoryBlueprint {
			return agentdomain.TaskReviseBlueprint
		}
		return agentdomain.TaskReviseDraft
	default:
		return workflowKind
	}
}

// decodeNodeOutput 对模型审阅先做确定性证据归一化，再执行领域校验。
// 模型不负责可靠计算 Unicode 偏移，也不能自行决定问题已解决或最终质量状态。
func decodeNodeOutput(
	kind agentdomain.TaskKind,
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	content string,
) (agentdomain.TaskOutput, error) {
	if node.Capability == agentdomain.CapabilityBoundedRevision && kind == agentdomain.TaskReviseDraft {
		return decodeDraftRevisionOutput(node, latest, artifacts, content)
	}
	if node.Capability != agentdomain.CapabilityContinuityReview &&
		node.Capability != agentdomain.CapabilityContentReview &&
		node.Capability != agentdomain.CapabilityStyleReview {
		return agentdomain.DecodeTaskOutput(kind, []byte(content))
	}
	if kind != agentdomain.TaskReviewBundle {
		return nil, agentdomain.ErrInvalidTaskOutput
	}
	var review agentdomain.ReviewBundleOutput
	decoder := json.NewDecoder(bytes.NewReader([]byte(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&review); err != nil {
		return nil, fmt.Errorf("%w: %v", agentdomain.ErrInvalidTaskOutput, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, agentdomain.ErrInvalidTaskOutput
	}
	candidate, _, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return nil, err
	}
	if err := normalizeModelReview(&review, candidate, reviewCandidateRef(node, latest)); err != nil {
		return nil, err
	}
	if err := review.Validate(); err != nil {
		return nil, err
	}
	return &review, nil
}

// normalizeModelReview 将模型审阅归一为可信的应用状态与可追溯证据。
func normalizeModelReview(review *agentdomain.ReviewBundleOutput, candidate, fallbackRef string) error {
	if review == nil {
		return agentdomain.ErrInvalidTaskOutput
	}
	if strings.TrimSpace(review.Summary) == "" {
		if len(review.Findings) == 0 {
			review.Summary = "模型审阅未发现问题。"
		} else {
			review.Summary = fmt.Sprintf("模型审阅返回 %d 个问题。", len(review.Findings))
		}
	}
	qualityStatus := agentdomain.QualityPassed
	seenIDs := make(map[string]struct{}, len(review.Findings))
	for index := range review.Findings {
		finding := &review.Findings[index]
		finding.ID = normalizedReviewFindingID(finding.ID, index, seenIDs)
		finding.Source = agentdomain.FindingModelReview
		finding.Status = agentdomain.FindingOpen
		if err := normalizeFindingEvidence(finding, candidate, fallbackRef); err != nil {
			return fmt.Errorf("review finding %s: %w", finding.ID, err)
		}
		switch finding.Severity {
		case agentdomain.SeverityBlocker:
			qualityStatus = agentdomain.QualityBlocked
		case agentdomain.SeverityMajor:
			if qualityStatus != agentdomain.QualityBlocked {
				qualityStatus = agentdomain.QualityNeedsReview
			}
		case agentdomain.SeverityMinor, agentdomain.SeverityNote:
			if qualityStatus == agentdomain.QualityPassed {
				qualityStatus = agentdomain.QualityNeedsReview
			}
		}
	}
	review.QualityStatus = qualityStatus
	return nil
}

// normalizedReviewFindingID 保留首个有效模型 ID，并确定性修复空值或重复值。
func normalizedReviewFindingID(modelID string, index int, seen map[string]struct{}) string {
	id := strings.TrimSpace(modelID)
	if id != "" {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			return id
		}
	}
	for suffix := index + 1; ; suffix++ {
		id = fmt.Sprintf("model-review-%d", suffix)
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			return id
		}
	}
}

// reviewCandidateRef 根据候选产物能力生成文档级结构引用，供无可靠引文时降级。
func reviewCandidateRef(node FrozenNode, latest map[string]agentdomain.RunStep) string {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, ok := latest[strings.TrimPrefix(selector, "node:")]
		if !ok {
			continue
		}
		switch step.Capability {
		case agentdomain.CapabilityStoryArchitect:
			return "blueprint:document"
		case agentdomain.CapabilityProseWriter:
			return "chapter:document"
		}
	}
	return "candidate:document"
}

func normalizeFindingEvidence(finding *agentdomain.ReviewFinding, candidate, fallbackRef string) error {
	if strings.TrimSpace(fallbackRef) == "" {
		return errors.New("missing fallback evidence reference")
	}
	quote := finding.Quote
	if strings.TrimSpace(quote) == "" {
		if strings.TrimSpace(finding.ConstraintRef) == "" {
			finding.ConstraintRef = fallbackRef
		}
		finding.Quote, finding.ScopeStart, finding.ScopeEnd = "", nil, nil
		return nil
	}
	candidateRunes, quoteRunes := []rune(candidate), []rune(quote)
	starts := quoteRuneStarts(candidateRunes, quoteRunes)
	if len(starts) == 0 {
		if strings.TrimSpace(finding.ConstraintRef) == "" {
			finding.ConstraintRef = fallbackRef
		}
		finding.Quote, finding.ScopeStart, finding.ScopeEnd = "", nil, nil
		return nil
	}
	start, ok := chooseQuoteStart(starts, finding.ScopeStart)
	if !ok {
		if strings.TrimSpace(finding.ConstraintRef) == "" {
			finding.ConstraintRef = fallbackRef
		}
		finding.Quote, finding.ScopeStart, finding.ScopeEnd = "", nil, nil
		return nil
	}
	end := start + len(quoteRunes)
	finding.ScopeStart, finding.ScopeEnd = &start, &end
	return nil
}

func quoteRuneStarts(candidate, quote []rune) []int {
	if len(quote) == 0 || len(quote) > len(candidate) {
		return nil
	}
	starts := make([]int, 0, 1)
	for start := 0; start+len(quote) <= len(candidate); start++ {
		matched := true
		for offset := range quote {
			if candidate[start+offset] != quote[offset] {
				matched = false
				break
			}
		}
		if matched {
			starts = append(starts, start)
		}
	}
	return starts
}

func chooseQuoteStart(starts []int, hint *int) (int, bool) {
	if len(starts) == 1 {
		return starts[0], true
	}
	if hint == nil || len(starts) == 0 {
		return 0, false
	}
	best, bestDistance, tied := starts[0], absInt(starts[0]-*hint), false
	for _, start := range starts[1:] {
		distance := absInt(start - *hint)
		switch {
		case distance < bestDistance:
			best, bestDistance, tied = start, distance, false
		case distance == bestDistance:
			tied = true
		}
	}
	return best, !tied
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// validateCapabilityOutput 执行 JSON Schema 无法表达的评审来源、定位与有限修订约束。
func validateCapabilityOutput(
	output agentdomain.TaskOutput,
	node FrozenNode,
	input persistedInput,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) error {
	switch node.Capability {
	case agentdomain.CapabilityChapterPlanner:
		return validateChapterDirector(output, input)
	case agentdomain.CapabilityContinuityReview, agentdomain.CapabilityContentReview, agentdomain.CapabilityStyleReview:
		return validateModelReview(output, node, latest, artifacts)
	case agentdomain.CapabilityBoundedRevision:
		return validateBoundedRevision(output, node, input, latest, artifacts)
	default:
		return nil
	}
}

func validateChapterDirector(output agentdomain.TaskOutput, input persistedInput) error {
	plan, ok := output.(*agentdomain.ChapterPlanOutput)
	if !ok || len(plan.CanonicalSourceRefs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(input.Sources)+1)
	allowed["author-instruction"] = struct{}{}
	for _, source := range input.Sources {
		allowed[source.ID] = struct{}{}
		allowed[source.ID+"@"+source.Version] = struct{}{}
	}
	for _, ref := range plan.CanonicalSourceRefs {
		if _, exists := allowed[ref]; !exists {
			return fmt.Errorf("chapter director references source outside frozen manifest: %s", ref)
		}
	}
	for _, scene := range plan.Scenes {
		for _, release := range scene.InformationRelease {
			for _, forbidden := range plan.ForbiddenReveals {
				if strings.Contains(release, forbidden) {
					return fmt.Errorf("chapter director reveals forbidden information: %s", forbidden)
				}
			}
		}
	}
	return nil
}

func validateModelReview(output agentdomain.TaskOutput, node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) error {
	review, ok := output.(*agentdomain.ReviewBundleOutput)
	if !ok {
		return agentdomain.ErrInvalidTaskOutput
	}
	candidate, _, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return err
	}
	runes := []rune(candidate)
	for _, finding := range review.Findings {
		if finding.Source != agentdomain.FindingModelReview {
			return fmt.Errorf("review finding %s has invalid source", finding.ID)
		}
		if finding.ScopeStart == nil {
			continue
		}
		if *finding.ScopeEnd > len(runes) || string(runes[*finding.ScopeStart:*finding.ScopeEnd]) != finding.Quote {
			return fmt.Errorf("review finding %s has invalid text range", finding.ID)
		}
	}
	return nil
}

func validateBoundedRevision(
	output agentdomain.TaskOutput,
	node FrozenNode,
	input persistedInput,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) error {
	if input.RevisionDirective != nil && input.RevisionDirective.Mode == agentdomain.RevisionAuthorDirected {
		base, err := revisionBaseArtifact(node, latest, artifacts)
		if err != nil {
			return err
		}
		if base.ID != input.RevisionDirective.BaseArtifactID || base.SHA256 != input.RevisionDirective.BaseDigest {
			return agentdomain.ErrInvalidRevisionDirective
		}
		return validateAuthorDirectedRevision(output, base.Content, *input.RevisionDirective)
	}
	original, reviewJSON, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return err
	}
	decodedReview, err := agentdomain.DecodeTaskOutput(agentdomain.TaskReviewBundle, []byte(reviewJSON))
	if err != nil {
		return err
	}
	review := decodedReview.(*agentdomain.ReviewBundleOutput)
	var revised string
	var resolutions []agentdomain.FindingResolution
	var changes []agentdomain.RevisionChange
	var blueprint *agentdomain.ReviseBlueprintOutput
	switch value := output.(type) {
	case *agentdomain.ReviseDraftOutput:
		revised, resolutions, changes = value.Body, value.Resolutions, value.Changes
	case *agentdomain.ReviseBlueprintOutput:
		blueprint, resolutions, changes = value, value.Resolutions, value.Changes
	default:
		return agentdomain.ErrInvalidTaskOutput
	}
	if err := normalizeResolutionIDs(review.Findings, resolutions, changes); err != nil {
		return err
	}
	if blueprint != nil {
		return validateBlueprintRevision(original, review, blueprint, input.Sources)
	}
	if err := validateResolutionCoverage(review.Findings, resolutions); err != nil {
		return err
	}
	expected := original
	for _, change := range changes {
		if !strings.Contains(expected, change.OriginalText) || !strings.Contains(revised, change.RevisedText) {
			return fmt.Errorf("revision change %s has no matching evidence", change.FindingID)
		}
		expected = strings.Replace(expected, change.OriginalText, change.RevisedText, 1)
	}
	if expected != revised {
		return errors.New("revision changed content outside declared evidence")
	}
	for _, source := range input.Sources {
		if source.Kind == agentdomain.ManifestAnchor && source.Required && strings.Contains(original, source.Content) && !strings.Contains(revised, source.Content) {
			return fmt.Errorf("revision lost required anchor %s", source.ID)
		}
	}
	return nil
}

// normalizeResolutionIDs 修正模型对 finding ID 的无意改名，保持一对一顺序和状态不变。
func normalizeResolutionIDs(
	findings []agentdomain.ReviewFinding,
	resolutions []agentdomain.FindingResolution,
	changes []agentdomain.RevisionChange,
) error {
	if len(findings) != len(resolutions) {
		return errors.New("revision resolutions do not cover review findings")
	}
	wanted := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		wanted[finding.ID] = struct{}{}
	}
	used := make(map[string]struct{}, len(resolutions))
	unmatchedResolutions := make([]int, 0)
	for index, resolution := range resolutions {
		if _, exists := wanted[resolution.FindingID]; exists {
			used[resolution.FindingID] = struct{}{}
			continue
		}
		unmatchedResolutions = append(unmatchedResolutions, index)
	}
	unmatchedFindings := make([]string, 0, len(unmatchedResolutions))
	for _, finding := range findings {
		if _, exists := used[finding.ID]; !exists {
			unmatchedFindings = append(unmatchedFindings, finding.ID)
		}
	}
	if len(unmatchedFindings) != len(unmatchedResolutions) {
		return errors.New("revision resolutions contain duplicate or unknown finding IDs")
	}
	if len(unmatchedResolutions) > 1 {
		return errors.New("revision renamed multiple finding IDs ambiguously")
	}
	for index, resolutionIndex := range unmatchedResolutions {
		oldID, newID := resolutions[resolutionIndex].FindingID, unmatchedFindings[index]
		resolutions[resolutionIndex].FindingID = newID
		for changeIndex := range changes {
			if changes[changeIndex].FindingID == oldID {
				changes[changeIndex].FindingID = newID
			}
		}
	}
	return nil
}

func validateAuthorDirectedRevision(output agentdomain.TaskOutput, original string, directive agentdomain.RevisionDirective) error {
	var revised string
	var changes []agentdomain.RevisionChange
	switch value := output.(type) {
	case *agentdomain.ReviseDraftOutput:
		revised, changes = value.Body, value.Changes
	case *agentdomain.ReviseBlueprintOutput:
		encoded, err := json.Marshal(value.Blueprint)
		if err != nil {
			return err
		}
		revised, changes = string(encoded), value.Changes
	default:
		return agentdomain.ErrInvalidTaskOutput
	}
	targets := make(map[string]struct{}, len(directive.TargetRefs))
	for _, target := range directive.TargetRefs {
		targets[target] = struct{}{}
	}
	expected := original
	for _, change := range changes {
		if _, allowed := targets[change.TargetRef]; !allowed || change.FindingID != "" ||
			!strings.Contains(expected, change.OriginalText) || !strings.Contains(revised, change.RevisedText) {
			return agentdomain.ErrInvalidRevisionDirective
		}
		expected = strings.Replace(expected, change.OriginalText, change.RevisedText, 1)
	}
	if expected != revised {
		return errors.New("author revision changed content outside target refs")
	}
	return nil
}

func validateResolutionCoverage(findings []agentdomain.ReviewFinding, resolutions []agentdomain.FindingResolution) error {
	want := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		want[finding.ID] = struct{}{}
	}
	if len(want) != len(resolutions) {
		return errors.New("revision resolutions do not cover review findings")
	}
	for _, resolution := range resolutions {
		if _, ok := want[resolution.FindingID]; !ok {
			return fmt.Errorf("revision references unknown finding %s", resolution.FindingID)
		}
	}
	return nil
}

func revisionInputs(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (string, string, error) {
	var candidate string
	reviews := make([]string, 0)
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, ok := latest[strings.TrimPrefix(selector, "node:")]
		if !ok {
			continue
		}
		artifact, ok := artifactByID(artifacts, step.PrimaryArtifactID)
		if !ok {
			return "", "", fmt.Errorf("missing upstream artifact %s", step.PrimaryArtifactID)
		}
		if step.Capability == agentdomain.CapabilityContinuityReview || step.Capability == agentdomain.CapabilityContentReview || step.Capability == agentdomain.CapabilityStyleReview {
			reviews = append(reviews, artifact.Content)
		} else {
			candidate = artifact.Content
		}
	}
	if candidate == "" {
		return "", "", errors.New("missing review candidate")
	}
	if node.Capability == agentdomain.CapabilityBoundedRevision && len(reviews) == 0 {
		return "", "", errors.New("missing structured review")
	}
	review, err := mergeReviewContents(reviews)
	if err != nil {
		return "", "", err
	}
	return candidate, review, nil
}

func mergeReviewContents(contents []string) (string, error) {
	if len(contents) == 1 {
		return contents[0], nil
	}
	groups := make([][]agentdomain.ReviewFinding, 0, len(contents))
	quality := agentdomain.QualityPassed
	for _, content := range contents {
		decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskReviewBundle, []byte(content))
		if err != nil {
			return "", err
		}
		review := decoded.(*agentdomain.ReviewBundleOutput)
		groups = append(groups, review.Findings)
		if review.QualityStatus == agentdomain.QualityBlocked || quality != agentdomain.QualityBlocked && review.QualityStatus == agentdomain.QualityNeedsReview {
			quality = review.QualityStatus
		}
	}
	merged, err := agentdomain.MergeFindings(groups...)
	if err != nil {
		return "", err
	}
	if len(merged.Findings) == 0 {
		quality = agentdomain.QualityPassed
	} else if quality == agentdomain.QualityPassed {
		quality = agentdomain.QualityNeedsReview
	}
	result := agentdomain.ReviewBundleOutput{QualityStatus: quality, Summary: fmt.Sprintf("确定性合并 %d 组审阅，发现 %d 个问题、%d 组冲突。", len(contents), len(merged.Findings), len(merged.Conflicts)), Findings: merged.Findings}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func artifactKind(capability agentdomain.Capability) agentdomain.ArtifactKind {
	switch capability {
	case agentdomain.CapabilityStoryArchitect, agentdomain.CapabilityChapterPlanner:
		return agentdomain.ArtifactPlan
	case agentdomain.CapabilityProseWriter:
		return agentdomain.ArtifactDraft
	case agentdomain.CapabilityBoundedRevision:
		return agentdomain.ArtifactRevision
	default:
		return agentdomain.ArtifactReview
	}
}

func shouldRunNode(condition string, latest map[string]agentdomain.RunStep) bool {
	if condition == "" || condition == "always" {
		return true
	}
	qualityIssue := false
	blocker := false
	for _, step := range latest {
		if step.Status != agentdomain.StepCompleted {
			continue
		}
		qualityIssue = qualityIssue || step.QualityStatus == agentdomain.QualityNeedsReview || step.QualityStatus == agentdomain.QualityBlocked
		blocker = blocker || step.QualityStatus == agentdomain.QualityBlocked
	}
	if condition == "on_quality_issue" {
		return qualityIssue
	}
	return condition == "on_blocker" && blocker
}

func aggregateQuality(compiled CompiledWorkflow, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) agentdomain.QualityStatus {
	if result, ok := latest[compiled.ResultNodeID]; ok {
		if artifact, exists := artifactByID(artifacts, result.PrimaryArtifactID); exists && artifact.Kind == agentdomain.ArtifactRevision {
			return result.QualityStatus
		}
	}
	result := agentdomain.QualityPassed
	for _, step := range latest {
		if step.QualityStatus == agentdomain.QualityBlocked {
			return agentdomain.QualityBlocked
		}
		if step.QualityStatus == agentdomain.QualityNeedsReview {
			result = agentdomain.QualityNeedsReview
		}
	}
	return result
}

func latestSteps(steps []agentdomain.RunStep) map[string]agentdomain.RunStep {
	latest := make(map[string]agentdomain.RunStep)
	for _, step := range steps {
		current, exists := latest[step.NodeID]
		if !exists || step.Attempt > current.Attempt {
			latest[step.NodeID] = step
		}
	}
	return latest
}

func replaceStep(run agentdomain.Run, replacement agentdomain.RunStep) agentdomain.Run {
	for index := range run.Steps {
		if run.Steps[index].ID == replacement.ID {
			run.Steps[index] = replacement
			return run
		}
	}
	run.Steps = append(run.Steps, replacement)
	return run
}

// protocolMaxOutput 计算本次请求实际发送的上限：方案预算模式在 OpenAI 上省略，
// 在强制要求上限的协议上使用剩余预算或技术封顶。
func protocolMaxOutput(step agentdomain.RunStep, remaining int, budgetUnlimited bool) int {
	if step.OutputLimitMode.OmitsConfiguredMaxOutput() {
		if budgetUnlimited && step.ProviderKind == agentdomain.ProviderOpenAI {
			return 0
		}
		cap := remaining
		if cap > agentdomain.ProviderTechnicalMaxOutput {
			cap = agentdomain.ProviderTechnicalMaxOutput
		}
		if cap < 1 {
			return 0
		}
		return cap
	}
	return min(step.MaxOutputTokens, remaining)
}

func generateRequest(
	step agentdomain.RunStep,
	remaining int,
	budgetUnlimited bool,
	system, input string,
	schema ports.SchemaAsset,
	schemaName string,
) ports.ModelGenerateRequest {
	return ports.ModelGenerateRequest{
		Model:           step.Model,
		System:          system,
		Input:           input,
		MaxOutputTokens: protocolMaxOutput(step, remaining, budgetUnlimited),
		ReasoningEffort: step.ReasoningEffort.Normalized(),
		Temperature:     step.Temperature,
		JSONSchema:      append([]byte(nil), schema.JSON...),
		SchemaName:      schemaName,
	}
}

func newArtifact(id, runID, stepID string, kind agentdomain.ArtifactKind, content string, now time.Time) agentdomain.Artifact {
	return agentdomain.Artifact{
		ID: id, RunID: runID, StepID: stepID, Kind: kind, Content: content,
		SHA256: agentdomain.SHA256Hex([]byte(content)), CreatedAt: now.UTC(),
	}
}

func jobDraft(
	run agentdomain.Run,
	eventType agentdomain.JobEventType,
	stepID string,
	payload any,
	now time.Time,
) (agentdomain.JobEventDraft, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return agentdomain.JobEventDraft{}, err
	}
	return agentdomain.NewJobEventDraft(
		run.ID, agentdomain.JobKindAgent, run.ProjectID, eventType, stepID, string(encoded), now,
	)
}

func digestStrings(values ...string) string {
	encoded, _ := json.Marshal(values)
	return agentdomain.SHA256Hex(encoded)
}

func artifactByID(artifacts []agentdomain.Artifact, id string) (agentdomain.Artifact, bool) {
	for _, artifact := range artifacts {
		if artifact.ID == id {
			return artifact, true
		}
	}
	return agentdomain.Artifact{}, false
}

func cloneSources(sources []Source) []Source {
	result := append([]Source(nil), sources...)
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Required != result[right].Required {
			return result[left].Required
		}
		if result[left].Priority != result[right].Priority {
			return result[left].Priority > result[right].Priority
		}
		return result[left].ID < result[right].ID
	})
	return result
}

func exportedVariable(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' || r == '.' })
	for index := range parts {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, "")
}

func capabilityInstruction(capability agentdomain.Capability) string {
	return "执行结构化创作任务。严格遵守输入约束并只返回指定 JSON Schema。职责：" + string(capability)
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrTokenBudget):
		return "token_budget_exceeded"
	case errors.Is(err, ErrContextBudget):
		return "context_budget_exceeded"
	case errors.Is(err, ErrInvalidReviewProof):
		return "invalid_review_evidence"
	case errors.Is(err, ErrInvalidRevision):
		return "invalid_revision_boundary"
	case errors.Is(err, ErrInvalidOutput):
		return "invalid_structured_output"
	case errors.Is(err, providers.ErrAuthentication):
		return "provider_authentication_failed"
	case errors.Is(err, providers.ErrModelNotFound):
		return "provider_model_not_found"
	case errors.Is(err, providers.ErrQuotaExceeded):
		return "provider_quota_exceeded"
	case errors.Is(err, providers.ErrRateLimited):
		return "provider_rate_limited"
	case errors.Is(err, providers.ErrNetwork):
		return "provider_network_failed"
	case errors.Is(err, providers.ErrContentRejected):
		return "provider_content_rejected"
	case errors.Is(err, providers.ErrIncompatible):
		return "provider_incompatible"
	case errors.Is(err, providers.ErrSchemaRejected):
		return "provider_schema_rejected"
	case errors.Is(err, providers.ErrResponseIncomplete):
		return "provider_response_incomplete"
	case errors.Is(err, providers.ErrResponseFailed):
		return "provider_response_failed"
	case errors.Is(err, providers.ErrEmptyResponse):
		return "provider_empty_response"
	case errors.Is(err, providers.ErrInvalidStream):
		return "provider_invalid_stream"
	case errors.Is(err, agentdomain.ErrInvalidTaskOutput), errors.Is(err, quality.ErrInvalidReconciliation):
		return "invalid_structured_output"
	case errors.Is(err, ErrQualityPolicy), errors.Is(err, quality.ErrPolicyNotFound):
		return "quality_policy_unavailable"
	default:
		return "provider_error"
	}
}

func safeErrorMessage(code string) string {
	switch code {
	case "cancelled":
		return "作者已取消智能创作任务。"
	case "timeout":
		return "模型调用超时，可在确认输入未变化后重试。"
	case "token_budget_exceeded":
		return "运行预算不足，未继续调用模型。"
	case "context_budget_exceeded":
		return "必要约束无法装入上下文预算。"
	case "invalid_structured_output":
		return "模型输出未通过结构化契约校验。"
	case "invalid_review_evidence":
		return "审阅问题缺少可定位的原文或约束证据。"
	case "invalid_revision_boundary":
		return "修订稿修改了评审未覆盖的内容，或变更证据无法对应到修订字段。"
	case "provider_authentication_failed":
		return "API Key 无效或无权访问当前模型。"
	case "provider_model_not_found":
		return "模型名称不存在或当前账户不可用。"
	case "provider_quota_exceeded":
		return "模型额度不足或账单受限。"
	case "provider_rate_limited":
		return "模型服务正在限流，请稍后重试。"
	case "provider_network_failed":
		return "模型网络连接失败，请检查网络或代理设置。"
	case "provider_content_rejected":
		return "请求被模型内容政策拒绝。"
	case "provider_incompatible":
		return "接口地址与所选模型服务不兼容。"
	case "provider_schema_rejected":
		return "模型服务拒绝了结构化输出格式，请更新应用或切换兼容模型。"
	case "provider_response_incomplete":
		return "模型输出达到上限而未完成，请提高最大输出 Token 后重试。"
	case "provider_response_failed":
		return "模型服务未能完成生成，请稍后重试。"
	case "provider_empty_response":
		return "模型返回了空内容，请检查模型能力或生成参数。"
	case "provider_invalid_stream":
		return "模型流式响应提前中断或格式不兼容。"
	case "credential_unavailable":
		return "模型凭据不可用，请检查连接设置。"
	case "asset_missing", "prompt_render_failed", "invalid_envelope", "quality_policy_unavailable":
		return "冻结的智能体资源无法执行。"
	default:
		return "模型调用失败，请检查连接、额度或服务状态。"
	}
}

func capabilityValidationError(capability agentdomain.Capability, cause error) (string, error) {
	switch capability {
	case agentdomain.CapabilityContinuityReview, agentdomain.CapabilityContentReview, agentdomain.CapabilityStyleReview:
		return "invalid_review_evidence", fmt.Errorf("%w: %w: %v", ErrInvalidOutput, ErrInvalidReviewProof, cause)
	case agentdomain.CapabilityBoundedRevision:
		return "invalid_revision_boundary", fmt.Errorf("%w: %w: %v", ErrInvalidOutput, ErrInvalidRevision, cause)
	default:
		return errorCode(cause), cause
	}
}
