package creation

import (
	"context"
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
	"github.com/lishen802/novel-studio/internal/quality"
)

const directionActionContract = "story_direction_action@1"

type DirectionActionRequest struct {
	CandidateID    string   `json:"candidateId"`
	TargetCardID   string   `json:"targetCardId,omitempty"`
	SourceCardIDs  []string `json:"sourceCardIds,omitempty"`
	Instruction    string   `json:"instruction"`
	IdempotencyKey string   `json:"idempotencyKey"`
}

type DirectionActionCommand struct {
	ActionID       string `json:"actionId"`
	ArgumentsHash  string `json:"argumentsHash"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type DirectionActionView struct {
	ActionID          string      `json:"actionId"`
	ArgumentsHash     string      `json:"argumentsHash"`
	IdempotencyKey    string      `json:"idempotencyKey"`
	Mode              string      `json:"mode"`
	CandidateID       string      `json:"candidateId"`
	CandidateVersion  int         `json:"candidateVersion"`
	TargetCardID      string      `json:"targetCardId,omitempty"`
	SourceCardIDs     []string    `json:"sourceCardIds,omitempty"`
	Instruction       string      `json:"instruction"`
	State             string      `json:"state"`
	Cost              CostPreview `json:"cost"`
	RunID             string      `json:"runId,omitempty"`
	ResultCandidateID string      `json:"resultCandidateId,omitempty"`
	ResultCardID      string      `json:"resultCardId,omitempty"`
	Summary           string      `json:"summary,omitempty"`
	Stale             bool        `json:"stale"`
}

type directionActionInput struct {
	Contract       string                              `json:"contract"`
	Mode           string                              `json:"mode"`
	Request        DirectionActionRequest              `json:"request"`
	Base           creationdomain.Candidate            `json:"base"`
	Sources        []agentkernel.Source                `json:"sources"`
	SessionVersion int64                               `json:"sessionVersion"`
	UpstreamDigest string                              `json:"upstreamDigest"`
	ProjectDigest  string                              `json:"projectDigest"`
	Profile        agentdomain.ResolvedCreativeProfile `json:"profile"`
	Cost           CostPreview                         `json:"cost"`
	RunID          string                              `json:"runId"`
}

// PreviewRegenerateDirection 冻结目标卡及完整三卡基线，不调用模型。
func (service *Service) PreviewRegenerateDirection(ctx context.Context, projectID string, request DirectionActionRequest, profile agentdomain.ResolvedCreativeProfile) (DirectionActionView, error) {
	return service.previewDirectionAction(ctx, projectID, "regenerate_one", request, profile)
}

// PreviewCombineDirections 冻结至少两张已保留卡的精确候选版本，不调用模型。
func (service *Service) PreviewCombineDirections(ctx context.Context, projectID string, request DirectionActionRequest, profile agentdomain.ResolvedCreativeProfile) (DirectionActionView, error) {
	return service.previewDirectionAction(ctx, projectID, "combine", request, profile)
}

func (service *Service) previewDirectionAction(ctx context.Context, projectID, mode string, request DirectionActionRequest, profile agentdomain.ResolvedCreativeProfile) (DirectionActionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	request.CandidateID = strings.TrimSpace(request.CandidateID)
	request.TargetCardID = strings.TrimSpace(request.TargetCardID)
	request.Instruction = strings.TrimSpace(request.Instruction)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.SourceCardIDs = normalizedCardIDs(request.SourceCardIDs)
	if request.CandidateID == "" || request.IdempotencyKey == "" || utf8.RuneCountInString(request.Instruction) > 4000 {
		return DirectionActionView{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return DirectionActionView{}, err
	}
	if session.Stage != creationdomain.StageDirections || session.ActiveRunID != "" {
		return DirectionActionView{}, ErrInvalidStage
	}
	assistant, err := service.ensureAssistantSession(ctx, session)
	if err != nil {
		return DirectionActionView{}, err
	}
	if existing, loadErr := service.store.ActionByIdempotency(ctx, assistant.ID, request.IdempotencyKey); loadErr == nil {
		input, decodeErr := decodeDirectionAction(existing)
		if decodeErr != nil || input.Mode != mode || !reflect.DeepEqual(input.Request, request) {
			return DirectionActionView{}, ErrOperationConflict
		}
		return service.directionActionView(ctx, session, existing, input)
	} else if !isMissingSession(loadErr) {
		return DirectionActionView{}, loadErr
	}
	base, err := service.store.Candidate(ctx, request.CandidateID)
	if err != nil {
		return DirectionActionView{}, err
	}
	if base.SessionID != session.ID || base.Kind != creationdomain.KindDirectionSet || base.Status == creationdomain.StatusSuperseded || base.Status == creationdomain.StatusDiscarded || base.Status == creationdomain.StatusStale || base.Blocked() {
		return DirectionActionView{}, creationdomain.ErrCandidateVersionConflict
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return DirectionActionView{}, err
	}
	current, ok := currentCandidateOfKind(candidates, creationdomain.StageDirections, creationdomain.KindDirectionSet)
	if !ok || current.ID != base.ID {
		return DirectionActionView{}, creationdomain.ErrCandidateVersionConflict
	}
	if err := validateDirectionActionRequest(mode, request, base, session.KeptDirectionIDs); err != nil {
		return DirectionActionView{}, err
	}
	binding := directionOperationBinding()
	compiled, err := service.compile(ctx, binding)
	if err != nil {
		return DirectionActionView{}, err
	}
	if !profile.RunBudgetUnlimited && profile.MaxRunTokens < compiled.RequiredTokenBudget() {
		return DirectionActionView{}, agentkernel.ErrTokenBudget
	}
	cost := CostPreview{Stage: creationdomain.StageDirections, WorkflowID: binding.WorkflowID, WorkflowVersion: binding.Version, EstimatedCalls: 1, MaxRunTokens: profile.MaxRunTokens, RunBudgetUnlimited: profile.RunBudgetUnlimited}
	foundPlanner := false
	for _, role := range profile.Roles {
		if role.Role == agentdomain.RolePlanner {
			foundPlanner = true
			cost.RoleModels = append(cost.RoleModels, RoleModelView{Role: string(role.Role), Provider: role.ProviderName, Model: role.Model})
		}
	}
	if !foundPlanner {
		return DirectionActionView{}, ErrUnavailable
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return DirectionActionView{}, err
	}
	projectJSON, _ := json.Marshal(project)
	sources, err := directionActionSources(session, base, mode, request)
	if err != nil {
		return DirectionActionView{}, err
	}
	input := directionActionInput{
		Contract: directionActionContract, Mode: mode, Request: request, Base: base, Sources: sources,
		SessionVersion: session.ContentVersion, UpstreamDigest: liveUpstreamDigest(session, candidates, creationdomain.StageDirections), ProjectDigest: hashBytes(projectJSON), Profile: profile,
		Cost: cost, RunID: service.ids.NewID(),
	}
	encoded, _ := json.Marshal(input)
	operation := assistantdomain.OpRegenerateCandidate
	if mode == "combine" {
		operation = assistantdomain.OpCombineDirections
	}
	targets := []string{fmt.Sprintf("candidate:%s@%d", base.ID, base.Version)}
	for _, id := range request.SourceCardIDs {
		targets = append(targets, "direction:"+id)
	}
	if request.TargetCardID != "" {
		targets = append(targets, "direction:"+request.TargetCardID)
	}
	now := service.clock.Now()
	action, err := assistantdomain.NewAction(service.ids.NewID(), assistant.ID, projectID, session.BranchID, operation, assistantdomain.RiskPaid, targets, []string{fmt.Sprintf("%s@%d", base.ID, base.Version)}, string(encoded), hashBytes(encoded), request.IdempotencyKey, "金额暂不可估算，按模型服务商实际用量计费", now, now.Add(assistantActionTTL))
	if err != nil {
		return DirectionActionView{}, err
	}
	if err := service.store.SaveAction(ctx, action); err != nil {
		return DirectionActionView{}, err
	}
	return service.directionActionView(ctx, session, action, input)
}

func validateDirectionActionRequest(mode string, request DirectionActionRequest, base creationdomain.Candidate, kept []string) error {
	switch mode {
	case "regenerate_one":
		if request.TargetCardID == "" {
			return creationdomain.ErrBlankValue
		}
		_, err := directionCardByID(base, request.TargetCardID)
		return err
	case "combine":
		if len(request.SourceCardIDs) < 2 || request.Instruction == "" {
			return creationdomain.ErrBlankValue
		}
		keptSet := make(map[string]bool, len(kept))
		for _, id := range kept {
			keptSet[id] = true
		}
		for _, id := range request.SourceCardIDs {
			if !keptSet[id] {
				return creationdomain.ErrDirectionCard
			}
			if _, err := directionCardByID(base, id); err != nil {
				return err
			}
		}
		return nil
	default:
		return assistantdomain.ErrInvalidOperation
	}
}

func normalizedCardIDs(ids []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func directionActionSources(session creationdomain.Session, base creationdomain.Candidate, mode string, request DirectionActionRequest) ([]agentkernel.Source, error) {
	contextPayload := map[string]any{"mode": mode, "base_candidate_id": base.ID, "base_version": base.Version, "direction_set": json.RawMessage(base.PayloadJSON)}
	if mode == "regenerate_one" {
		contextPayload["target_card_id"] = request.TargetCardID
	} else {
		cards := make([]agentdomain.DirectionCard, 0, len(request.SourceCardIDs))
		for _, id := range request.SourceCardIDs {
			card, err := directionCardByID(base, id)
			if err != nil {
				return nil, err
			}
			cards = append(cards, card)
		}
		contextPayload["source_cards"] = cards
	}
	encoded, _ := json.Marshal(contextPayload)
	task := request.Instruction
	if task == "" {
		task = "在保持另外两张方向卡不变的前提下，重新设计目标方向卡。"
	}
	return []agentkernel.Source{
		{Kind: agentdomain.ManifestAuthorInstruction, ID: "direction_action", Version: fmt.Sprintf("%s:%d", base.ID, base.Version), Content: task, Priority: 1100, SelectionReason: "作者确认的方向操作", Required: true},
		{Kind: agentdomain.ManifestStoryAsset, ID: "direction_set", Version: fmt.Sprint(base.Version), Content: string(encoded), Priority: 1000, SelectionReason: "冻结的方向卡与精确版本", Provisional: true, Required: true},
		{Kind: agentdomain.ManifestAuthorInstruction, ID: "intent", Version: "1", Content: session.IntentText, Priority: 900, SelectionReason: "作者创作意图", Required: true},
	}, nil
}

func decodeDirectionAction(action assistantdomain.Action) (directionActionInput, error) {
	var input directionActionInput
	if json.Unmarshal([]byte(action.Arguments), &input) != nil || input.Contract != directionActionContract || hashBytes([]byte(action.Arguments)) != action.ArgumentsHash {
		return input, ErrUnavailable
	}
	return input, nil
}

func (service *Service) directionActionCurrent(ctx context.Context, session creationdomain.Session, input directionActionInput) error {
	if session.ContentVersion != input.SessionVersion {
		return creationdomain.ErrSessionVersionConflict
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	current, ok := currentCandidateOfKind(candidates, creationdomain.StageDirections, creationdomain.KindDirectionSet)
	if !ok || !reflect.DeepEqual(current, input.Base) {
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
	return validateDirectionActionRequest(input.Mode, input.Request, input.Base, session.KeptDirectionIDs)
}

// ConfirmDirectionAction 确认冻结摘要后异步执行，重复确认只返回原操作状态。
func (service *Service) ConfirmDirectionAction(ctx context.Context, projectID string, command DirectionActionCommand, profile agentdomain.ResolvedCreativeProfile) (DirectionActionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	action, err := service.store.Action(ctx, command.ActionID)
	if err != nil {
		return DirectionActionView{}, err
	}
	if action.ProjectID != projectID || action.ArgumentsHash != command.ArgumentsHash || action.IdempotencyKey != command.IdempotencyKey {
		return DirectionActionView{}, ErrOperationConflict
	}
	input, err := decodeDirectionAction(action)
	if err != nil {
		return DirectionActionView{}, err
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return DirectionActionView{}, err
	}
	if action.State != assistantdomain.StatePendingConfirmation {
		return service.directionActionView(ctx, session, action, input)
	}
	if !reflect.DeepEqual(profile, input.Profile) {
		return DirectionActionView{}, ErrConfirmationRequired
	}
	if err := service.directionActionCurrent(ctx, session, input); err != nil {
		return DirectionActionView{}, err
	}
	if session.ActiveRunID != "" {
		return DirectionActionView{}, ErrRunActive
	}
	executing, err := action.Confirm(command.ArgumentsHash, command.IdempotencyKey, service.clock.Now())
	if err != nil {
		return DirectionActionView{}, err
	}
	commandName := "regenerate_direction"
	if input.Mode == "combine" {
		commandName = "combine_directions"
	}
	now := service.clock.Now()
	operation, created, err := service.store.ReserveOperation(ctx, creationdomain.Operation{ID: action.ID, ProjectID: projectID, BranchID: session.BranchID, Command: commandName, IdempotencyKey: action.IdempotencyKey, ArgumentsHash: action.ArgumentsHash, FrozenInputDigest: input.Base.ContentSHA256, Status: creationdomain.OperationReserved, RunID: input.RunID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return DirectionActionView{}, err
	}
	if !created {
		return DirectionActionView{}, assistantdomain.ErrReplayForbidden
	}
	if err := service.store.SaveAction(ctx, executing); err != nil {
		return DirectionActionView{}, err
	}
	captured := service.generation()
	lease, release, err := service.coordinator.BeginRun(context.WithoutCancel(ctx), input.RunID)
	if err != nil {
		service.failDirectionAction(ctx, executing, operation, "启动失败")
		return DirectionActionView{}, err
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
		service.failDirectionAction(lease.Context(), executing, operation, "运行槽冲突")
		return DirectionActionView{}, err
	}
	_, err = service.kernel.CreateRun(lease.Context(), agentkernel.CreateRequest{
		RunID: input.RunID, ProjectID: projectID, BranchID: session.BranchID, CanonicalHead: input.ProjectDigest,
		AuthorInstruction: directionActionInstruction(input), WorkflowID: agentassets.StoryDirectionOperationWorkflowID,
		WorkflowVersion: directionOperationBinding().Version, Profile: input.Profile, Sources: input.Sources,
		QualitySpec: quality.Spec{}, Deadline: now.Add(defaultRunTimeout),
	})
	if err != nil {
		service.failDirectionAction(lease.Context(), executing, operation, "启动失败")
		service.clearRunSlot(lease.Context(), projectID, input.RunID)
		return DirectionActionView{}, err
	}
	operation.Status = creationdomain.OperationRunning
	if err := service.store.UpdateOperation(lease.Context(), operation); err != nil {
		return DirectionActionView{}, err
	}
	go func() {
		defer release()
		_, executeErr := service.kernel.ExecuteRun(lease.Context(), input.RunID, input.ProjectDigest)
		storeCtx := context.WithoutCancel(lease.Context())
		if lease.Stale() || captured != service.generation() {
			return
		}
		if executeErr != nil {
			service.failDirectionAction(storeCtx, executing, operation, "运行失败或已取消")
		} else {
			service.completeDirectionAction(storeCtx, executing, operation, input)
		}
		service.clearRunSlot(storeCtx, projectID, input.RunID)
	}()
	launched = true
	return service.directionActionView(ctx, session, executing, input)
}

func directionActionInstruction(input directionActionInput) string {
	if input.Mode == "combine" {
		return "组合指定的已保留方向卡。作者要求：" + input.Request.Instruction
	}
	if input.Request.Instruction == "" {
		return "只重做目标方向卡，保持另外两张卡不变。"
	}
	return "只重做目标方向卡。作者要求：" + input.Request.Instruction
}

func (service *Service) failDirectionAction(ctx context.Context, action assistantdomain.Action, operation creationdomain.Operation, code string) {
	failed, err := action.Fail(code, service.clock.Now())
	if err == nil {
		_ = service.store.SaveAction(ctx, failed)
	}
	service.finishOperation(ctx, operation, creationdomain.OperationFailed, "", code)
}

func (service *Service) completeDirectionAction(ctx context.Context, action assistantdomain.Action, operation creationdomain.Operation, input directionActionInput) {
	candidate, err := service.materializeDirectionAction(ctx, action.ProjectID, input)
	if err != nil {
		service.failDirectionAction(ctx, action, operation, "方向结果无法核验")
		return
	}
	succeeded, err := action.Succeed([]string{candidate.ID}, service.clock.Now())
	if err != nil || service.store.SaveAction(ctx, succeeded) != nil {
		return
	}
	service.finishOperation(ctx, operation, creationdomain.OperationSucceeded, candidate.ID, "")
	if session, loadErr := service.loadSession(ctx, action.ProjectID); loadErr == nil {
		_ = service.refreshImpact(ctx, session)
	}
}

func (service *Service) materializeDirectionAction(ctx context.Context, projectID string, input directionActionInput) (creationdomain.Candidate, error) {
	candidates, err := service.store.ListCandidates(ctx, input.Base.SessionID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	for _, item := range candidates {
		if item.RunID == input.RunID {
			return item, nil
		}
	}
	run, err := service.kernel.Run(ctx, input.RunID)
	if err != nil || run.Status != agentdomain.RunCompleted {
		return creationdomain.Candidate{}, ErrUnavailable
	}
	artifacts, err := service.kernel.Artifacts(ctx, input.RunID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	payload, artifactID, err := service.resultPayload(ctx, run, artifacts, directionOperationBinding())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirection, []byte(payload))
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	card := decoded.(*agentdomain.StoryDirectionOutput).Direction
	now := service.clock.Now()
	if input.Mode == "regenerate_one" {
		current, ok := currentCandidateOfKind(candidates, creationdomain.StageDirections, creationdomain.KindDirectionSet)
		if !ok || !reflect.DeepEqual(current, input.Base) {
			return creationdomain.Candidate{}, creationdomain.ErrCandidateVersionConflict
		}
		baseOutputAny, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirections, []byte(input.Base.PayloadJSON))
		if err != nil {
			return creationdomain.Candidate{}, err
		}
		baseOutput := baseOutputAny.(*agentdomain.StoryDirectionsOutput)
		found := false
		for index := range baseOutput.Directions {
			if baseOutput.Directions[index].ID == input.Request.TargetCardID {
				card.ID = input.Request.TargetCardID
				baseOutput.Directions[index] = card
				found = true
				break
			}
		}
		if !found || baseOutput.Validate() != nil {
			return creationdomain.Candidate{}, creationdomain.ErrDirectionCard
		}
		merged, _ := json.Marshal(baseOutput)
		next, err := input.Base.Regenerate(service.ids.NewID(), input.RunID, artifactID, string(merged), now)
		if err != nil {
			return creationdomain.Candidate{}, err
		}
		next = next.WithUpstream(input.UpstreamDigest)
		superseded, err := input.Base.MarkSuperseded(now)
		if err != nil {
			return creationdomain.Candidate{}, err
		}
		if err := service.store.ApplyCandidateChanges(ctx, []creationdomain.Candidate{input.Base}, []creationdomain.Candidate{superseded}, []creationdomain.Candidate{next}); err != nil {
			return creationdomain.Candidate{}, err
		}
		if session, loadErr := service.loadSession(ctx, projectID); loadErr == nil {
			if followed, followErr := session.FollowDirectionCandidate(input.Base.ID, next.ID, now); followErr == nil && followed.SelectedDirectionCandidateID != session.SelectedDirectionCandidateID {
				_ = service.saveSession(ctx, &followed)
			}
		}
		return next, nil
	}
	card.ID = "combined-" + input.RunID
	cardJSON, _ := json.Marshal(card)
	version := 1
	for _, item := range candidates {
		if item.Kind == creationdomain.KindDirection && item.Version >= version {
			version = item.Version + 1
		}
	}
	combined, err := creationdomain.NewCandidate(service.ids.NewID(), input.Base.SessionID, creationdomain.StageDirections, creationdomain.KindDirection, version, input.Base.ID, input.RunID, artifactID, string(cardJSON), "story_direction.schema", 1, now)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	combined = combined.WithUpstream(input.UpstreamDigest)
	if err := service.store.SaveCandidate(ctx, combined); err != nil {
		if errors.Is(err, creationdomain.ErrCandidateResultExists) {
			for _, item := range candidates {
				if item.RunID == input.RunID {
					return item, nil
				}
			}
		}
		return creationdomain.Candidate{}, err
	}
	return combined, nil
}

// DirectionActions 恢复方向操作；进程中断后只对账既有运行，不自动重放模型调用。
func (service *Service) DirectionActions(ctx context.Context, projectID string, recoverRun bool) ([]DirectionActionView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	assistant, err := service.store.AssistantSession(ctx, projectID, session.BranchID)
	if isMissingSession(err) {
		return []DirectionActionView{}, nil
	}
	if err != nil {
		return nil, err
	}
	actions, err := service.store.ListActions(ctx, assistant.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].CreatedAt.After(actions[j].CreatedAt) })
	views := []DirectionActionView{}
	for _, action := range actions {
		input, decodeErr := decodeDirectionAction(action)
		if decodeErr != nil {
			continue
		}
		if recoverRun && action.State == assistantdomain.StateExecuting && !service.coordinator.Live(input.RunID) {
			operation, operationErr := service.store.Operation(ctx, action.ID)
			run, runErr := service.kernel.Run(ctx, input.RunID)
			if operationErr == nil && runErr == nil && run.Status == agentdomain.RunCompleted {
				service.completeDirectionAction(ctx, action, operation, input)
			} else if operationErr == nil {
				service.failDirectionAction(ctx, action, operation, "运行已中断，请重新发起操作")
			}
			service.clearRunSlot(ctx, projectID, input.RunID)
			action, _ = service.store.Action(ctx, action.ID)
		}
		view, viewErr := service.directionActionView(ctx, session, action, input)
		if viewErr != nil {
			return nil, viewErr
		}
		views = append(views, view)
		if len(views) >= 20 {
			break
		}
	}
	return views, nil
}

func (service *Service) directionActionView(ctx context.Context, session creationdomain.Session, action assistantdomain.Action, input directionActionInput) (DirectionActionView, error) {
	view := DirectionActionView{
		ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
		Mode: input.Mode, CandidateID: input.Base.ID, CandidateVersion: input.Base.Version,
		TargetCardID: input.Request.TargetCardID, SourceCardIDs: append([]string{}, input.Request.SourceCardIDs...),
		Instruction: input.Request.Instruction, State: string(action.State), Cost: input.Cost,
		RunID: input.RunID, Summary: action.ErrorCode,
	}
	view.Stale = service.directionActionCurrent(ctx, session, input) != nil && action.State == assistantdomain.StatePendingConfirmation
	if len(action.ResultRefs) > 0 {
		view.ResultCandidateID = action.ResultRefs[0]
		if candidate, err := service.store.Candidate(ctx, action.ResultRefs[0]); err == nil {
			if input.Mode == "regenerate_one" {
				view.ResultCardID = input.Request.TargetCardID
			} else {
				var card agentdomain.DirectionCard
				if json.Unmarshal([]byte(candidate.PayloadJSON), &card) == nil {
					view.ResultCardID = card.ID
				}
			}
		}
	}
	return view, nil
}

func currentCandidateOfKind(candidates []creationdomain.Candidate, stage creationdomain.Stage, kind creationdomain.CandidateKind) (creationdomain.Candidate, bool) {
	var current creationdomain.Candidate
	found := false
	for _, candidate := range candidates {
		if candidate.Stage != stage || candidate.Kind != kind || candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
			continue
		}
		if !found || candidate.Version > current.Version {
			current = candidate
			found = true
		}
	}
	return current, found
}
