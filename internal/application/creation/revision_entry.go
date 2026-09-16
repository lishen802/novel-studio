package creation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type RevisionRequest struct {
	CandidateID    string   `json:"candidateId"`
	Paths          []string `json:"paths"`
	Instruction    string   `json:"instruction"`
	IdempotencyKey string   `json:"idempotencyKey"`
}
type RevisionCommand struct {
	ActionID       string `json:"actionId"`
	ArgumentsHash  string `json:"argumentsHash"`
	IdempotencyKey string `json:"idempotencyKey"`
	// Paths 为空表示采用整组差异；非空时只采用给定路径，其余保持基线原文，
	// 结果作为新的作者候选并标记待复核，不冒充已通过独立审阅的整组结果。
	Paths []string `json:"paths"`
}
type RevisionView struct {
	ActionID       string             `json:"actionId"`
	ArgumentsHash  string             `json:"argumentsHash"`
	IdempotencyKey string             `json:"idempotencyKey"`
	CandidateID    string             `json:"candidateId"`
	Version        int                `json:"version"`
	State          string             `json:"state"`
	Instruction    string             `json:"instruction"`
	Targets        []TextFieldView    `json:"targets"`
	Preserved      []TextFieldView    `json:"preserved"`
	Cost           CostPreview        `json:"cost"`
	RunID          string             `json:"runId"`
	Quality        string             `json:"quality"`
	Summary        string             `json:"summary"`
	Stale          bool               `json:"stale"`
	CanApply       bool               `json:"canApply"`
	Applied        bool               `json:"applied"`
	Diffs          []RevisionDiffView `json:"diffs"`
}
type RevisionDiffView struct {
	// Path 是稳定字段路径，供作者选择部分采纳时原样回传。
	Path   string `json:"path"`
	Label  string `json:"label"`
	Group  string `json:"group"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type revisionInput struct {
	Sources        []agentkernel.Source                `json:"sources"`
	Contract       string                              `json:"contract"`
	Request        RevisionRequest                     `json:"request"`
	Base           creationdomain.Candidate            `json:"base"`
	SessionVersion int64                               `json:"sessionVersion"`
	UpstreamDigest string                              `json:"upstreamDigest"`
	ProjectDigest  string                              `json:"projectDigest"`
	Profile        agentdomain.ResolvedCreativeProfile `json:"profile"`
	Cost           CostPreview                         `json:"cost"`
	RunID          string                              `json:"runId"`
	BaseID         string                              `json:"baseId"`
	ResultID       string                              `json:"resultId"`
}

func revisionBinding() stageBinding {
	return stageBinding{WorkflowID: agentassets.StoryCandidateRevisionWorkflowID, Version: agentassets.StoryCandidateRevisionWorkflowVersion, SchemaID: "story_candidate_revision.schema", SchemaVersion: 2}
}

// revisionBindingFor 返回当前统一的候选修订合同。
func revisionBindingFor(_ creationdomain.Candidate) stageBinding {
	return revisionBinding()
}

// PreviewRevision 冻结作者要求、候选、保持项、模型和费用范围；预览不调用模型。
func (service *Service) PreviewRevision(ctx context.Context, projectID string, request RevisionRequest, profile agentdomain.ResolvedCreativeProfile) (RevisionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	request.Instruction = strings.TrimSpace(request.Instruction)
	if request.Instruction == "" || utf8.RuneCountInString(request.Instruction) > 4000 || len(request.Paths) == 0 || len(request.Paths) > 100 || request.IdempotencyKey == "" {
		return RevisionView{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return RevisionView{}, err
	}
	assistant, err := service.ensureAssistantSession(ctx, session)
	if err != nil {
		return RevisionView{}, err
	}
	if existing, e := service.store.ActionByIdempotency(ctx, assistant.ID, request.IdempotencyKey); e == nil {
		input, e := decodeRevisionInput(existing)
		if e != nil || !reflect.DeepEqual(input.Request, request) {
			return RevisionView{}, ErrOperationConflict
		}
		return service.revisionView(ctx, session, existing, input)
	} else if !isMissingSession(e) {
		return RevisionView{}, e
	}
	if session.ActiveRunID != "" {
		return RevisionView{}, ErrRunActive
	}
	base, err := service.CandidatePayload(ctx, projectID, request.CandidateID)
	if err != nil {
		return RevisionView{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return RevisionView{}, err
	}
	current, ok := currentCandidate(candidates, base.Stage)
	if !ok || current.ID != base.ID || base.Status == creationdomain.StatusStale {
		return RevisionView{}, creationdomain.ErrCandidateVersionConflict
	}
	envelope := revisionEnvelope(base)
	if err := envelope.Validate(); err != nil {
		return RevisionView{}, err
	}
	allowed := map[string]bool{}
	for _, field := range candidateTextFields(base).Fields {
		allowed[field.Path] = !field.Locked
	}
	seen := map[string]bool{}
	for _, path := range request.Paths {
		if !allowed[path] || seen[path] {
			return RevisionView{}, creationdomain.ErrFieldLocked
		}
		seen[path] = true
	}
	binding := revisionBindingFor(base)
	compiled, err := service.compile(ctx, binding)
	if err != nil {
		return RevisionView{}, err
	}
	if !profile.RunBudgetUnlimited && profile.MaxRunTokens < compiled.RequiredTokenBudget() {
		return RevisionView{}, agentkernel.ErrTokenBudget
	}
	cost := CostPreview{Stage: base.Stage, WorkflowID: compiled.Workflow.ID, WorkflowVersion: binding.Version, EstimatedCalls: 2, MaxRunTokens: profile.MaxRunTokens, RunBudgetUnlimited: profile.RunBudgetUnlimited}
	for _, role := range []agentdomain.Role{agentdomain.RolePlanner, agentdomain.RoleContinuityReviewer} {
		found := false
		for _, resolved := range profile.Roles {
			if resolved.Role == role {
				found = true
				cost.RoleModels = append(cost.RoleModels, RoleModelView{Role: string(role), Provider: resolved.ProviderName, Model: resolved.Model})
			}
		}
		if !found {
			return RevisionView{}, ErrUnavailable
		}
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return RevisionView{}, err
	}
	projectJSON, _ := json.Marshal(project)
	sources, err := generateSources(session, candidates, base.Stage)
	if err != nil {
		return RevisionView{}, err
	}
	input := revisionInput{Sources: sources, Contract: "story_revision@1", Request: request, Base: base, SessionVersion: session.ContentVersion, UpstreamDigest: liveUpstreamDigest(session, candidates, base.Stage), ProjectDigest: hashBytes(projectJSON), Profile: profile, Cost: cost, RunID: service.ids.NewID(), BaseID: service.ids.NewID(), ResultID: service.ids.NewID()}
	encoded, _ := json.Marshal(input)
	now := service.clock.Now()
	action, err := assistantdomain.NewAction(service.ids.NewID(), assistant.ID, projectID, session.BranchID, assistantdomain.OpReviseFields, assistantdomain.RiskPaid, []string{base.ID}, []string{fmt.Sprintf("%s@%d", base.ID, base.Version)}, string(encoded), hashBytes(encoded), request.IdempotencyKey, "金额暂不可估算，按模型服务商实际用量计费", now, now.Add(assistantActionTTL))
	if err != nil {
		return RevisionView{}, err
	}
	if err := service.store.SaveAction(ctx, action); err != nil {
		return RevisionView{}, err
	}
	return service.revisionView(ctx, session, action, input)
}
func decodeRevisionInput(action assistantdomain.Action) (revisionInput, error) {
	var input revisionInput
	if json.Unmarshal([]byte(action.Arguments), &input) != nil || input.Contract != "story_revision@1" || hashBytes([]byte(action.Arguments)) != action.ArgumentsHash {
		return input, ErrUnavailable
	}
	return input, nil
}
func revisionEnvelope(base creationdomain.Candidate) agentdomain.StoryCandidateRevisionOutput {
	return agentdomain.StoryCandidateRevisionOutput{CandidateKind: string(base.Kind), PayloadSchemaRef: fmt.Sprintf("%s@%d", base.PayloadSchemaID, base.PayloadSchemaVersion), Payload: json.RawMessage(base.PayloadJSON)}
}

// revisionCurrent 只核对冻结基线，不把旧授权换绑到最新候选。
func (service *Service) revisionCurrent(ctx context.Context, session creationdomain.Session, input revisionInput) error {
	if session.ContentVersion != input.SessionVersion {
		return creationdomain.ErrSessionVersionConflict
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	current, ok := currentCandidate(candidates, input.Base.Stage)
	if !ok || !reflect.DeepEqual(current, input.Base) || liveUpstreamDigest(session, candidates, input.Base.Stage) != input.UpstreamDigest {
		return creationdomain.ErrCandidateVersionConflict
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(project)
	if hashBytes(encoded) != input.ProjectDigest {
		return creationdomain.ErrCandidateVersionConflict
	}
	return nil
}

// ConfirmRevision 预留唯一操作与运行槽后异步执行；同一授权只启动一次。
func (service *Service) ConfirmRevision(ctx context.Context, projectID string, command RevisionCommand, profile agentdomain.ResolvedCreativeProfile) (RevisionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	action, err := service.store.Action(ctx, command.ActionID)
	if err != nil {
		return RevisionView{}, err
	}
	if action.ProjectID != projectID || action.ArgumentsHash != command.ArgumentsHash || action.IdempotencyKey != command.IdempotencyKey {
		return RevisionView{}, ErrOperationConflict
	}
	input, err := decodeRevisionInput(action)
	if err != nil {
		return RevisionView{}, err
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return RevisionView{}, err
	}
	if action.State != assistantdomain.StatePendingConfirmation {
		return service.revisionView(ctx, session, action, input)
	}
	if !reflect.DeepEqual(profile, input.Profile) {
		return RevisionView{}, ErrConfirmationRequired
	}
	if err := service.revisionCurrent(ctx, session, input); err != nil {
		return RevisionView{}, err
	}
	if session.ActiveRunID != "" {
		return RevisionView{}, ErrRunActive
	}
	executing, err := action.Confirm(command.ArgumentsHash, command.IdempotencyKey, service.clock.Now())
	if err != nil {
		return RevisionView{}, err
	}
	now := service.clock.Now()
	operation, created, err := service.store.ReserveOperation(ctx, creationdomain.Operation{ID: action.ID, ProjectID: projectID, BranchID: session.BranchID, Command: "revise_candidate", IdempotencyKey: action.IdempotencyKey, ArgumentsHash: action.ArgumentsHash, FrozenInputDigest: input.UpstreamDigest, Status: creationdomain.OperationReserved, RunID: input.RunID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return RevisionView{}, err
	}
	if !created {
		return RevisionView{}, assistantdomain.ErrReplayForbidden
	}
	if err := service.store.SaveAction(ctx, executing); err != nil {
		return RevisionView{}, err
	}
	captured := service.generation()
	lease, release, err := service.coordinator.BeginRun(context.WithoutCancel(ctx), input.RunID)
	if err != nil {
		service.failRevision(ctx, executing, operation, "启动失败")
		return RevisionView{}, err
	}
	launched := false
	defer func() {
		if !launched {
			release()
		}
	}()
	session.ActiveRunID = input.RunID
	session.ActiveOperationID = operation.ID
	if err := service.store.SaveRunSlot(lease.Context(), session); err != nil {
		service.failRevision(lease.Context(), executing, operation, "运行槽冲突")
		return RevisionView{}, err
	}
	baseJSON, _ := json.Marshal(revisionEnvelope(input.Base))
	base := agentdomain.BaseCandidate{ArtifactID: input.BaseID, Digest: hashBytes(baseJSON), Content: string(baseJSON), Blueprint: true}
	preserve := []string{}
	for path, locked := range input.Base.LockMap {
		if locked {
			preserve = append(preserve, path)
		}
	}
	sort.Strings(preserve)
	directive := agentdomain.RevisionDirective{Version: 1, Mode: agentdomain.RevisionAuthorDirected, BaseArtifactID: base.ArtifactID, BaseDigest: base.Digest, TargetRefs: input.Request.Paths, PreserveRefs: preserve, AuthorInstructionRef: input.Request.Instruction}
	_, err = service.kernel.CreateRun(lease.Context(), agentkernel.CreateRequest{RunID: input.RunID, ProjectID: projectID, BranchID: session.BranchID, CanonicalHead: input.ProjectDigest, AuthorInstruction: input.Request.Instruction, WorkflowID: agentassets.StoryCandidateRevisionWorkflowID, WorkflowVersion: revisionBindingFor(input.Base).Version, Profile: input.Profile, Sources: input.Sources, BaseCandidate: &base, RevisionDirective: &directive, Deadline: now.Add(defaultRunTimeout)})
	if err != nil {
		service.failRevision(lease.Context(), executing, operation, "启动失败")
		service.clearRunSlot(lease.Context(), projectID, input.RunID)
		return RevisionView{}, err
	}
	operation.Status = creationdomain.OperationRunning
	if err := service.store.UpdateOperation(lease.Context(), operation); err != nil {
		return RevisionView{}, err
	}
	go func() {
		defer release()
		_, executeErr := service.kernel.ExecuteRun(lease.Context(), input.RunID, input.ProjectDigest)
		storeCtx := context.WithoutCancel(lease.Context())
		if lease.Stale() || captured != service.generation() {
			return
		}
		if executeErr != nil {
			service.failRevision(storeCtx, executing, operation, "运行失败或已取消")
		} else {
			service.completeRevision(storeCtx, executing, operation, input)
		}
		service.clearRunSlot(storeCtx, projectID, input.RunID)
	}()
	launched = true
	return service.revisionView(ctx, session, executing, input)
}
func (service *Service) failRevision(ctx context.Context, action assistantdomain.Action, operation creationdomain.Operation, code string) {
	failed, err := action.Fail(code, service.clock.Now())
	if err == nil {
		_ = service.store.SaveAction(ctx, failed)
	}
	service.finishOperation(ctx, operation, creationdomain.OperationFailed, "", code)
}
func (service *Service) completeRevision(ctx context.Context, action assistantdomain.Action, operation creationdomain.Operation, input revisionInput) {
	_, _, _, err := service.revisionResult(ctx, input)
	if err != nil {
		service.failRevision(ctx, action, operation, "修订结果无法核验")
		return
	}
	succeeded, err := action.Succeed([]string{input.RunID}, service.clock.Now())
	if err != nil {
		return
	}
	if service.store.SaveAction(ctx, succeeded) != nil {
		return
	}
	service.finishOperation(ctx, operation, creationdomain.OperationSucceeded, "", "")
}

// revisionResult 使用冻结工作流出口读取修订，独立审阅未完成时不返回可采用结果。
func (service *Service) revisionResult(ctx context.Context, input revisionInput) (agentdomain.Run, string, []creationdomain.TextRevisionDiff, error) {
	run, err := service.kernel.Run(ctx, input.RunID)
	if err != nil {
		return run, "", nil, err
	}
	if run.Status != agentdomain.RunCompleted {
		return run, "", nil, ErrUnavailable
	}
	artifacts, err := service.kernel.Artifacts(ctx, input.RunID)
	if err != nil {
		return run, "", nil, err
	}
	payload, _, err := service.resultPayload(ctx, run, artifacts, revisionBinding())
	if err != nil {
		return run, "", nil, err
	}
	var output agentdomain.StoryCandidateRevisionOutput
	if json.Unmarshal([]byte(payload), &output) != nil || output.Validate() != nil {
		return run, "", nil, ErrUnavailable
	}
	expected := revisionEnvelope(input.Base)
	if output.CandidateKind != expected.CandidateKind || output.PayloadSchemaRef != expected.PayloadSchemaRef {
		return run, "", nil, ErrUnavailable
	}
	diffs, err := input.Base.ReviewTextRevision(string(output.Payload), input.Request.Paths)
	return run, string(output.Payload), diffs, err
}

// RevisionActions 恢复公开操作状态，不自动重放付费调用。
func (service *Service) RevisionActions(ctx context.Context, projectID string, recoverRun bool) ([]RevisionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	assistant, err := service.store.AssistantSession(ctx, projectID, session.BranchID)
	if isMissingSession(err) {
		return []RevisionView{}, nil
	}
	if err != nil {
		return nil, err
	}
	actions, err := service.store.ListActions(ctx, assistant.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].CreatedAt.After(actions[j].CreatedAt) })
	views := []RevisionView{}
	for _, action := range actions {
		input, err := decodeRevisionInput(action)
		if err != nil {
			continue
		}
		if recoverRun && action.State == assistantdomain.StateExecuting && !service.coordinator.Live(input.RunID) {
			operation, e := service.store.Operation(ctx, action.ID)
			if e == nil {
				run, runErr := service.kernel.Run(ctx, input.RunID)
				if runErr == nil && run.Status == agentdomain.RunCompleted {
					service.completeRevision(ctx, action, operation, input)
				} else {
					service.failRevision(ctx, action, operation, "运行已中断，请重新提出修改")
				}
				service.clearRunSlot(ctx, projectID, input.RunID)
				action, _ = service.store.Action(ctx, action.ID)
			}
		}
		view, err := service.revisionView(ctx, session, action, input)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
		if len(views) >= 20 {
			break
		}
	}
	return views, nil
}
func (service *Service) revisionView(ctx context.Context, session creationdomain.Session, action assistantdomain.Action, input revisionInput) (RevisionView, error) {
	view := RevisionView{ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey, CandidateID: input.Base.ID, Version: input.Base.Version, State: string(action.State), Instruction: input.Request.Instruction, Targets: []TextFieldView{}, Preserved: []TextFieldView{}, Cost: input.Cost, Diffs: []RevisionDiffView{}, Summary: action.ErrorCode}
	fields := candidateTextFields(input.Base).Fields
	targets := map[string]bool{}
	for _, path := range input.Request.Paths {
		targets[path] = true
	}
	labels := map[string]TextFieldView{}
	for _, field := range fields {
		labels[field.Path] = field
		if targets[field.Path] {
			view.Targets = append(view.Targets, field)
		} else if field.Locked {
			view.Preserved = append(view.Preserved, field)
		}
	}
	view.Stale = service.revisionCurrent(ctx, session, input) != nil
	if action.State == assistantdomain.StatePendingConfirmation && !service.clock.Now().Before(action.ExpiresAt) {
		view.State = "expired"
	}
	if action.State == assistantdomain.StateExecuting || action.State == assistantdomain.StateSucceeded || action.State == assistantdomain.StateFailed {
		view.RunID = input.RunID
	}
	if candidate, e := service.store.Candidate(ctx, input.ResultID); e == nil && candidate.RunID == input.RunID {
		view.Applied = true
	}
	if action.State == assistantdomain.StateSucceeded {
		run, _, diffs, e := service.revisionResult(ctx, input)
		if e != nil {
			return view, e
		}
		view.Quality = string(run.QualityStatus)
		view.Summary = "修订与独立审阅已完成，采用后仍需确认本阶段候选。"
		if run.QualityStatus != agentdomain.QualityPassed {
			view.Summary = "独立审阅发现问题，此结果暂不可采用。"
		}
		view.Summary += service.revisionReviewSummary(ctx, run)
		for _, diff := range diffs {
			label := labels[diff.Path]
			view.Diffs = append(view.Diffs, RevisionDiffView{Path: diff.Path, Label: label.Label, Group: label.Group, Before: diff.Before, After: diff.After})
		}
		view.CanApply = !view.Stale && !view.Applied && run.QualityStatus == agentdomain.QualityPassed && len(diffs) > 0
	}
	return view, nil
}

// ApplyRevision 显式采用已经审阅的文本差异，生成新候选而非写入正式资产。
// 命令携带 Paths 时只采用选中的差异子集：未选路径保持基线原文，结果标记待复核。
func (service *Service) ApplyRevision(ctx context.Context, projectID string, command RevisionCommand) (creationdomain.Candidate, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	action, err := service.store.Action(ctx, command.ActionID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if action.ProjectID != projectID || action.ArgumentsHash != command.ArgumentsHash || action.IdempotencyKey != command.IdempotencyKey || action.State != assistantdomain.StateSucceeded {
		return creationdomain.Candidate{}, ErrOperationConflict
	}
	input, err := decodeRevisionInput(action)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if len(command.Paths) == 0 {
		// 整组采用按结果工件重放。
		if existing, e := service.store.Candidate(ctx, input.ResultID); e == nil && existing.RunID == input.RunID {
			return existing, nil
		}
	} else {
		// 部分采用按派生子集 ID 重放：同一子集在基线被替代后仍返回同一候选，
		// 不同子集走后续校验而不是被当成同一次采用。
		selected, e := normalizeRevisionPaths(command.Paths)
		if e != nil {
			return creationdomain.Candidate{}, e
		}
		if existing, e := service.store.Candidate(ctx, partialRevisionResultID(input.ResultID, selected)); e == nil {
			return existing, nil
		}
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if session.ActiveRunID != "" && session.ActiveRunID != input.RunID {
		return creationdomain.Candidate{}, ErrRunActive
	}
	if err := service.revisionCurrent(ctx, session, input); err != nil {
		return creationdomain.Candidate{}, err
	}
	run, payload, diffs, err := service.revisionResult(ctx, input)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if run.QualityStatus != agentdomain.QualityPassed || len(diffs) == 0 {
		return creationdomain.Candidate{}, ErrUnavailable
	}
	artifacts, err := service.kernel.Artifacts(ctx, run.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	_, artifactID, err := service.resultPayload(ctx, run, artifacts, revisionBinding())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	var next creationdomain.Candidate
	if len(command.Paths) > 0 {
		next, err = service.adoptPartialRevision(ctx, input, diffs, command.Paths)
	} else {
		next, err = input.Base.Regenerate(input.ResultID, run.ID, artifactID, payload, service.clock.Now())
		if err == nil {
			next.QualityStatus = creationdomain.QualityPassed
			next.UpstreamDigest = input.UpstreamDigest
		}
	}
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	superseded, err := input.Base.MarkSuperseded(service.clock.Now())
	if err != nil {
		return next, err
	}
	expected, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return next, err
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return next, err
	}
	if err := service.revisionCurrent(ctx, session, input); err != nil {
		return next, err
	}
	guard := creationdomain.CandidateCommitGuard{SessionID: session.ID, ContentVersion: session.ContentVersion, ProjectID: project.ID, ProjectUpdatedAt: project.UpdatedAt}
	if err := service.store.ApplyCandidateChanges(ctx, expected, []creationdomain.Candidate{superseded}, []creationdomain.Candidate{next}, guard); err != nil {
		if errors.Is(err, creationdomain.ErrCandidateResultExists) {
			return service.store.Candidate(ctx, input.ResultID)
		}
		return next, err
	}
	return next, service.refreshImpact(ctx, session)
}

// adoptPartialRevision 只把作者选中的差异合入基线，确定性合成新的作者候选。
// 结果未经过对应整组结果的独立审阅，因此标记待复核而不是复用整组通过的结论。
func (service *Service) adoptPartialRevision(
	ctx context.Context,
	input revisionInput,
	diffs []creationdomain.TextRevisionDiff,
	paths []string,
) (creationdomain.Candidate, error) {
	selected, err := normalizeRevisionPaths(paths)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	changes, err := creationdomain.AdoptTextRevisionDiffs(diffs, selected)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	// 以冻结基线为准应用选中的差异，未选路径保持基线原文。
	synthesized, err := input.Base.EditTextFields(changes)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	resultID := partialRevisionResultID(input.ResultID, selected)
	if existing, e := service.store.Candidate(ctx, resultID); e == nil {
		return existing, nil
	}
	next, err := input.Base.SaveDraft(resultID, synthesized, service.clock.Now())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	next.QualityStatus = creationdomain.QualityNeedsReview
	next.UpstreamDigest = input.UpstreamDigest
	return next, nil
}

// partialRevisionResultID 按采用子集派生稳定的结果 ID：同一子集重放返回同一候选，
// 不同子集产生不同候选，不会把参数不同的请求当成同一次采用。
func partialRevisionResultID(resultID string, selected []string) string {
	sum := sha256.Sum256([]byte(strings.Join(selected, "\n")))
	return resultID + "-partial-" + hex.EncodeToString(sum[:])[:8]
}

// normalizeRevisionPaths 校验部分采纳的路径集合：非空、无空白歧义、无重复，并按路径排序。
// 排序保证同一子集无论界面顺序如何都派生同一个结果 ID。
func normalizeRevisionPaths(paths []string) ([]string, error) {
	if len(paths) == 0 || len(paths) > 1000 {
		return nil, creationdomain.ErrRevisionScope
	}
	normalized := append([]string(nil), paths...)
	sort.Strings(normalized)
	for i, path := range normalized {
		if path == "" || path != strings.TrimSpace(path) || (i > 0 && path == normalized[i-1]) {
			return nil, creationdomain.ErrRevisionScope
		}
	}
	return normalized, nil
}

// revisionReviewSummary 只展示完成的独立审阅工件中的可读结论。
func (service *Service) revisionReviewSummary(ctx context.Context, run agentdomain.Run) string {
	artifacts, err := service.kernel.Artifacts(ctx, run.ID)
	if err != nil {
		return ""
	}
	for _, step := range run.Steps {
		if step.Capability != agentdomain.CapabilityContinuityReview || step.Status != agentdomain.StepCompleted {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.ID != step.PrimaryArtifactID {
				continue
			}
			var review agentdomain.ReviewBundleOutput
			if json.Unmarshal([]byte(artifact.Content), &review) != nil || review.Validate() != nil {
				continue
			}
			lines := []string{"", review.Summary}
			for _, finding := range review.Findings {
				lines = append(lines, finding.Message)
				if finding.Suggestion != "" {
					lines = append(lines, "建议："+finding.Suggestion)
				}
			}
			return strings.Join(lines, "\n")
		}
	}
	return ""
}
