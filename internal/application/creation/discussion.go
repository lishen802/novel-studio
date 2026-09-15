package creation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

const (
	discussionContract           = "story_discussion@1"
	discussionAuthorizationTTL   = 4 * time.Hour
	defaultDiscussionCalls       = 8
	defaultDiscussionTurnTokens  = 8000
	defaultDiscussionTotalTokens = 16000
)

type DiscussionAuthorizationRequest struct {
	CandidateID    string   `json:"candidateId"`
	EntityRef      string   `json:"entityRef"`
	FieldPaths     []string `json:"fieldPaths"`
	Message        string   `json:"message"`
	MaxCalls       int      `json:"maxCalls"`
	MaxTotalTokens int      `json:"maxTotalTokens"`
	MaxTurnTokens  int      `json:"maxTurnTokens"`
	IdempotencyKey string   `json:"idempotencyKey"`
}

type DiscussionAuthorizationCommand struct {
	ActionID              string `json:"actionId"`
	ArgumentsHash         string `json:"argumentsHash"`
	IdempotencyKey        string `json:"idempotencyKey"`
	Message               string `json:"message"`
	MessageIdempotencyKey string `json:"messageIdempotencyKey"`
}

type DiscussionMessageRequest struct {
	AuthorizationID string   `json:"authorizationId"`
	CandidateID     string   `json:"candidateId"`
	EntityRef       string   `json:"entityRef"`
	FieldPaths      []string `json:"fieldPaths"`
	Text            string   `json:"text"`
	IdempotencyKey  string   `json:"idempotencyKey"`
}

type DiscussionSuggestionRequest struct {
	MessageID      string `json:"messageId"`
	SuggestionID   string `json:"suggestionId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type DiscussionAuthorizationView struct {
	ID               string      `json:"id"`
	ActionID         string      `json:"actionId"`
	State            string      `json:"state"`
	CandidateID      string      `json:"candidateId"`
	CandidateVersion int         `json:"candidateVersion"`
	EntityRef        string      `json:"entityRef"`
	FieldPaths       []string    `json:"fieldPaths"`
	RemainingCalls   int         `json:"remainingCalls"`
	RemainingTokens  int         `json:"remainingTokens"`
	MaxCalls         int         `json:"maxCalls"`
	MaxTotalTokens   int         `json:"maxTotalTokens"`
	MaxTurnTokens    int         `json:"maxTurnTokens"`
	ExpiresAt        string      `json:"expiresAt"`
	Provider         string      `json:"provider"`
	Model            string      `json:"model"`
	Cost             CostPreview `json:"cost"`
	Stale            bool        `json:"stale"`
}

type DiscussionTurnView struct {
	Authorization  DiscussionAuthorizationView `json:"authorization"`
	ActionID       string                      `json:"actionId"`
	ArgumentsHash  string                      `json:"argumentsHash"`
	IdempotencyKey string                      `json:"idempotencyKey"`
	State          string                      `json:"state"`
	RunID          string                      `json:"runId"`
	Author         AssistantMessageView        `json:"author"`
	Reply          *AssistantMessageView       `json:"reply,omitempty"`
	DraftMessage   string                      `json:"draftMessage,omitempty"`
}

type discussionAuthInput struct {
	Contract        string                              `json:"contract"`
	Request         DiscussionAuthorizationRequest      `json:"request"`
	Base            creationdomain.Candidate            `json:"base"`
	SessionVersion  int64                               `json:"sessionVersion"`
	UpstreamDigest  string                              `json:"upstreamDigest"`
	ProjectDigest   string                              `json:"projectDigest"`
	Profile         agentdomain.ResolvedCreativeProfile `json:"profile"`
	Cost            CostPreview                         `json:"cost"`
	Planner         RoleModelView                       `json:"planner"`
	AuthorizationID string                              `json:"authorizationId"`
	ExpiresAtUnix   int64                               `json:"expiresAtUnix"`
	MaxCalls        int                                 `json:"maxCalls"`
	MaxTotalTokens  int                                 `json:"maxTotalTokens"`
	MaxTurnTokens   int                                 `json:"maxTurnTokens"`
}

type discussionModelOutput struct {
	Answer            string   `json:"answer"`
	FollowUpQuestions []string `json:"follow_up_questions"`
	Suggestions       []struct {
		ID            string   `json:"id"`
		Operation     string   `json:"operation"`
		Paths         []string `json:"paths"`
		Instruction   string   `json:"instruction"`
		PreservePaths []string `json:"preserve_paths"`
		Rationale     string   `json:"rationale"`
	} `json:"suggestions"`
}

// PreviewDiscussionAuthorization 冻结讨论目标、模型与预算，不创建运行或调用模型。
func (service *Service) PreviewDiscussionAuthorization(ctx context.Context, projectID string, request DiscussionAuthorizationRequest, profile agentdomain.ResolvedCreativeProfile) (DiscussionTurnView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Message = strings.TrimSpace(request.Message)
	if request.IdempotencyKey == "" || strings.TrimSpace(request.CandidateID) == "" || utf8.RuneCountInString(request.Message) > 4000 {
		return DiscussionTurnView{}, creationdomain.ErrBlankValue
	}
	normalizeDiscussionBudget(&request)
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	assistant, err := service.ensureAssistantSession(ctx, session)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if existing, e := service.store.ActionByIdempotency(ctx, assistant.ID, request.IdempotencyKey); e == nil {
		input, e := decodeDiscussionInput(existing)
		if e != nil || !reflect.DeepEqual(input.Request, request) {
			return DiscussionTurnView{}, ErrOperationConflict
		}
		return service.discussionPreviewView(ctx, session, existing, input)
	} else if !isMissingSession(e) {
		return DiscussionTurnView{}, e
	}
	base, planner, cost, err := service.discussionPreviewParts(ctx, session, request, profile)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	project, err := service.store.Project(ctx)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	projectJSON, _ := json.Marshal(project)
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	now := service.clock.Now()
	input := discussionAuthInput{
		Contract: discussionContract, Request: request, Base: base, SessionVersion: session.ContentVersion,
		UpstreamDigest: liveUpstreamDigest(session, candidates, base.Stage), ProjectDigest: hashBytes(projectJSON),
		Profile: profile, Cost: cost, Planner: planner, AuthorizationID: service.ids.NewID(),
		ExpiresAtUnix: now.Add(discussionAuthorizationTTL).Unix(), MaxCalls: request.MaxCalls,
		MaxTotalTokens: request.MaxTotalTokens, MaxTurnTokens: request.MaxTurnTokens,
	}
	encoded, _ := json.Marshal(input)
	action, err := assistantdomain.NewAction(service.ids.NewID(), assistant.ID, projectID, session.BranchID,
		assistantdomain.OpAuthorizeDiscussion, assistantdomain.RiskPaid, []string{base.ID},
		[]string{fmt.Sprintf("%s@%d", base.ID, base.Version)}, string(encoded), hashBytes(encoded),
		request.IdempotencyKey, "金额暂不可估算，按模型服务商实际用量计费", now, now.Add(assistantActionTTL))
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if err := service.store.SaveAction(ctx, action); err != nil {
		return DiscussionTurnView{}, err
	}
	return service.discussionPreviewView(ctx, session, action, input)
}

func (service *Service) discussionPreviewParts(ctx context.Context, session creationdomain.Session, request DiscussionAuthorizationRequest, profile agentdomain.ResolvedCreativeProfile) (creationdomain.Candidate, RoleModelView, CostPreview, error) {
	base, err := service.CandidatePayload(ctx, session.ProjectID, request.CandidateID)
	if err != nil {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, err
	}
	current, ok := currentCandidate(candidates, base.Stage)
	if !ok || current.ID != base.ID || base.Status == creationdomain.StatusStale {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, creationdomain.ErrCandidateVersionConflict
	}
	if err := validateDiscussionPaths(base, request.FieldPaths); err != nil {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, err
	}
	planner, err := plannerRole(profile)
	if err != nil {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, err
	}
	compiled, err := service.compile(ctx, discussionBinding())
	if err != nil {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, err
	}
	if !profile.RunBudgetUnlimited && profile.MaxRunTokens < compiled.RequiredTokenBudget() {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, agentkernel.ErrTokenBudget
	}
	if request.MaxTurnTokens < compiled.RequiredTokenBudget() || (!profile.RunBudgetUnlimited && request.MaxTurnTokens > profile.MaxRunTokens) {
		return creationdomain.Candidate{}, RoleModelView{}, CostPreview{}, agentkernel.ErrTokenBudget
	}
	cost := CostPreview{
		Stage: base.Stage, WorkflowID: compiled.Workflow.ID, WorkflowVersion: compiled.Workflow.Version,
		EstimatedCalls: request.MaxCalls, MaxRunTokens: request.MaxTurnTokens, RunBudgetUnlimited: false,
		RoleModels: []RoleModelView{planner},
	}
	return base, planner, cost, nil
}

// ConfirmDiscussionAuthorization 确认预算后启动第一轮只读讨论，立即返回运行引用。
func (service *Service) ConfirmDiscussionAuthorization(ctx context.Context, projectID string, command DiscussionAuthorizationCommand, profile agentdomain.ResolvedCreativeProfile) (DiscussionTurnView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	command.Message = strings.TrimSpace(command.Message)
	command.MessageIdempotencyKey = strings.TrimSpace(command.MessageIdempotencyKey)
	if command.Message == "" || command.MessageIdempotencyKey == "" || utf8.RuneCountInString(command.Message) > 4000 {
		return DiscussionTurnView{}, creationdomain.ErrBlankValue
	}
	action, err := service.store.Action(ctx, command.ActionID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if action.ProjectID != projectID || action.ArgumentsHash != command.ArgumentsHash || action.IdempotencyKey != command.IdempotencyKey {
		return DiscussionTurnView{}, ErrOperationConflict
	}
	input, err := decodeDiscussionInput(action)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if existing, existingErr := service.store.AssistantMessageByIdempotency(ctx, action.SessionID, command.MessageIdempotencyKey); existingErr == nil {
		if existing.Text != command.Message || existing.CandidateID != input.Base.ID || existing.EntityRef != input.Request.EntityRef || !slices.Contains(input.Request.FieldPaths, existing.FieldPath) {
			return DiscussionTurnView{}, ErrOperationConflict
		}
		auth, authErr := service.store.Authorization(ctx, existing.AuthorizationID)
		if authErr != nil {
			return DiscussionTurnView{}, authErr
		}
		if action, authErr = service.completeDiscussionAuthorizationAction(ctx, action, auth.ID, existing.RunID); authErr != nil {
			return DiscussionTurnView{}, authErr
		}
		return service.discussionTurnView(ctx, session, action, auth, existing)
	} else if !isMissingSession(existingErr) {
		return DiscussionTurnView{}, existingErr
	}
	if action.State != assistantdomain.StatePendingConfirmation {
		auth, authErr := service.store.AuthorizationByAction(ctx, action.ID)
		if authErr != nil {
			if action.State != assistantdomain.StateExecuting || !isMissingSession(authErr) {
				return DiscussionTurnView{}, authErr
			}
			now := service.clock.Now()
			auth, authErr = assistantdomain.NewAuthorization(
				input.AuthorizationID, action.SessionID, projectID, session.BranchID, action.ID,
				input.Base.ID, input.Base.ContentSHA256, string(input.Base.Stage), input.Request.EntityRef,
				input.Base.Version, input.Request.FieldPaths, discussionProfileID(input.Profile),
				plannerConnection(input.Profile), input.Planner.Provider, input.Planner.Model, plannerEffort(input.Profile),
				input.MaxCalls, input.MaxTotalTokens, input.MaxTurnTokens, now, time.Unix(input.ExpiresAtUnix, 0).UTC(),
			)
			if authErr != nil {
				return DiscussionTurnView{}, authErr
			}
			auth, authErr = auth.Activate(now)
			if authErr != nil {
				return DiscussionTurnView{}, authErr
			}
			if authErr = service.store.SaveAuthorization(ctx, auth); authErr != nil {
				return DiscussionTurnView{}, authErr
			}
		}
		turn, turnErr := service.existingDiscussionTurn(ctx, session, action, auth, command.MessageIdempotencyKey, command.Message)
		if !errors.Is(turnErr, assistantdomain.ErrReplayForbidden) || auth.State != assistantdomain.AuthActive {
			return turn, turnErr
		}
		if auth.ReservedCalls > 0 {
			auth, authErr = auth.ReleaseTurn(service.clock.Now())
			if authErr != nil {
				return DiscussionTurnView{}, authErr
			}
			if authErr = service.store.SaveAuthorization(ctx, auth); authErr != nil {
				return DiscussionTurnView{}, authErr
			}
		}
		return service.startDiscussionTurn(ctx, session, action, auth, input, command.Message, command.MessageIdempotencyKey)
	}
	if !reflect.DeepEqual(profile, input.Profile) {
		return DiscussionTurnView{}, ErrConfirmationRequired
	}
	if err := service.discussionCurrent(ctx, session, input); err != nil {
		return DiscussionTurnView{}, err
	}
	if session.ActiveRunID != "" {
		return DiscussionTurnView{}, ErrRunActive
	}
	if input.Request.Message != "" && command.Message != input.Request.Message {
		return DiscussionTurnView{}, ErrOperationConflict
	}
	executing, err := action.Confirm(command.ArgumentsHash, command.IdempotencyKey, service.clock.Now())
	if err != nil {
		return DiscussionTurnView{}, err
	}
	now := service.clock.Now()
	auth, err := assistantdomain.NewAuthorization(
		input.AuthorizationID, executing.SessionID, projectID, session.BranchID, executing.ID,
		input.Base.ID, input.Base.ContentSHA256, string(input.Base.Stage), input.Request.EntityRef,
		input.Base.Version, input.Request.FieldPaths, discussionProfileID(input.Profile),
		plannerConnection(input.Profile), input.Planner.Provider, input.Planner.Model, plannerEffort(input.Profile),
		input.MaxCalls, input.MaxTotalTokens, input.MaxTurnTokens, now, time.Unix(input.ExpiresAtUnix, 0).UTC(),
	)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	auth, err = auth.Activate(now)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if err := service.store.SaveAuthorization(ctx, auth); err != nil {
		return DiscussionTurnView{}, err
	}
	turn, err := service.startDiscussionTurn(ctx, session, executing, auth, input, command.Message, command.MessageIdempotencyKey)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	return turn, nil
}

// SendDiscussionMessage 在已确认授权内发送后续只读讨论，不再次确认费用。
func (service *Service) SendDiscussionMessage(ctx context.Context, projectID string, request DiscussionMessageRequest, profile agentdomain.ResolvedCreativeProfile) (DiscussionTurnView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	request.Text = strings.TrimSpace(request.Text)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.Text == "" || request.IdempotencyKey == "" || utf8.RuneCountInString(request.Text) > 4000 {
		return DiscussionTurnView{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	auth, err := service.store.Authorization(ctx, request.AuthorizationID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if auth.ProjectID != projectID {
		return DiscussionTurnView{}, creationdomain.ErrBlankValue
	}
	action, err := service.store.Action(ctx, auth.ActionID)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	input, err := decodeDiscussionInput(action)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if existing, e := service.store.AssistantMessageByIdempotency(ctx, action.SessionID, request.IdempotencyKey); e == nil {
		if existing.Text != request.Text || existing.CandidateID != request.CandidateID ||
			existing.AuthorizationID != request.AuthorizationID || existing.EntityRef != strings.TrimSpace(request.EntityRef) ||
			existing.FieldPath != primaryFieldPath(request.FieldPaths) {
			return DiscussionTurnView{}, ErrOperationConflict
		}
		return service.discussionTurnView(ctx, session, action, auth, existing)
	} else if !isMissingSession(e) {
		return DiscussionTurnView{}, e
	}
	if err := auth.CanStartTurn(service.clock.Now()); err != nil {
		if errors.Is(err, assistantdomain.ErrExpired) {
			if expired, expireErr := auth.Expire(service.clock.Now()); expireErr == nil {
				_ = service.store.SaveAuthorization(ctx, expired)
			}
		}
		return DiscussionTurnView{}, err
	}
	if !auth.MatchesTarget(request.CandidateID, auth.CandidateSHA256, auth.CandidateVersion) ||
		strings.TrimSpace(request.EntityRef) != auth.EntityRef ||
		!sameFieldPaths(request.FieldPaths, auth.FieldPaths) {
		return DiscussionTurnView{}, creationdomain.ErrCandidateVersionConflict
	}
	if !auth.MatchesProfile(discussionProfileID(profile), plannerConnection(profile), plannerModel(profile)) {
		stale, markErr := auth.MarkStale(service.clock.Now())
		if markErr == nil {
			_ = service.store.SaveAuthorization(ctx, stale)
		}
		return DiscussionTurnView{}, ErrConfirmationRequired
	}
	if err := service.discussionCurrent(ctx, session, input); err != nil {
		stale, markErr := auth.MarkStale(service.clock.Now())
		if markErr == nil {
			_ = service.store.SaveAuthorization(ctx, stale)
		}
		return DiscussionTurnView{}, err
	}
	if session.ActiveRunID != "" {
		return DiscussionTurnView{}, ErrRunActive
	}
	return service.startDiscussionTurn(ctx, session, action, auth, input, request.Text, request.IdempotencyKey)
}

// RevokeDiscussionAuthorization 阻止新消息，不删除历史，也不宣称 Provider 已停止。
func (service *Service) RevokeDiscussionAuthorization(ctx context.Context, projectID, authorizationID string) (WorkbenchView, error) {
	mutex := service.lockProject(projectID)
	mutex.Lock()
	defer mutex.Unlock()
	auth, err := service.store.Authorization(ctx, authorizationID)
	if err != nil {
		return WorkbenchView{}, err
	}
	if auth.ProjectID != projectID {
		return WorkbenchView{}, creationdomain.ErrBlankValue
	}
	revoked, err := auth.Revoke(service.clock.Now())
	if err != nil {
		return WorkbenchView{}, err
	}
	if err := service.store.SaveAuthorization(ctx, revoked); err != nil {
		return WorkbenchView{}, err
	}
	return service.Workbench(ctx, projectID, auth.BranchID)
}

// PreviewDiscussionSuggestion 把讨论建议转成现有修订提案，不启动修订运行。
func (service *Service) PreviewDiscussionSuggestion(ctx context.Context, projectID string, request DiscussionSuggestionRequest, profile agentdomain.ResolvedCreativeProfile) (RevisionView, error) {
	request.MessageID = strings.TrimSpace(request.MessageID)
	request.SuggestionID = strings.TrimSpace(request.SuggestionID)
	if request.MessageID == "" || request.SuggestionID == "" {
		return RevisionView{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return RevisionView{}, err
	}
	assistant, err := service.store.AssistantSession(ctx, projectID, session.BranchID)
	if err != nil {
		return RevisionView{}, err
	}
	messages, err := service.store.ListAssistantMessages(ctx, assistant.ID)
	if err != nil {
		return RevisionView{}, err
	}
	var found assistantdomain.Message
	for _, message := range messages {
		if message.ID == request.MessageID {
			found = message
			break
		}
	}
	if found.ID == "" || found.Role != assistantdomain.RoleAssistant {
		return RevisionView{}, creationdomain.ErrBlankValue
	}
	var suggestion assistantdomain.Suggestion
	for _, item := range found.Suggestions {
		if item.ID == request.SuggestionID {
			suggestion = item
			break
		}
	}
	if suggestion.ID == "" || suggestion.Operation != assistantdomain.OpReviseFields {
		return RevisionView{}, creationdomain.ErrBlankValue
	}
	base, err := service.store.Candidate(ctx, found.CandidateID)
	if err != nil {
		return RevisionView{}, err
	}
	if found.AuthorizationID != "" {
		auth, authErr := service.store.Authorization(ctx, found.AuthorizationID)
		if authErr == nil && (auth.CandidateSHA256 != base.ContentSHA256 || auth.CandidateVersion != base.Version) {
			return RevisionView{}, creationdomain.ErrCandidateVersionConflict
		}
	}
	if err := validateDiscussionPaths(base, suggestion.Paths); err != nil {
		return RevisionView{}, err
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		key = fmt.Sprintf("discussion:%s:suggestion:%s", found.RunID, suggestion.ID)
	}
	return service.PreviewRevision(ctx, projectID, RevisionRequest{
		CandidateID: base.ID, Paths: suggestion.Paths, Instruction: suggestion.Instruction, IdempotencyKey: key,
	}, profile)
}

func (service *Service) startDiscussionTurn(
	ctx context.Context,
	session creationdomain.Session,
	action assistantdomain.Action,
	auth assistantdomain.Authorization,
	input discussionAuthInput,
	text, messageKey string,
) (DiscussionTurnView, error) {
	now := service.clock.Now()
	sources, err := service.discussionSources(ctx, session, auth, input, text)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	reserved, err := auth.ReserveTurn(now)
	if err != nil {
		return DiscussionTurnView{}, err
	}
	if err := service.store.SaveAuthorization(ctx, reserved); err != nil {
		return DiscussionTurnView{}, err
	}
	runID := service.ids.NewID()
	author, err := assistantdomain.NewDiscussionMessage(service.ids.NewID(), action.SessionID, assistantdomain.RoleAuthor,
		assistantdomain.MessagePending, text, string(input.Base.Stage), input.Base.ID, input.Request.EntityRef,
		primaryFieldPath(input.Request.FieldPaths), action.ID, reserved.ID, messageKey, now)
	if err != nil {
		released, _ := reserved.ReleaseTurn(now)
		_ = service.store.SaveAuthorization(ctx, released)
		return DiscussionTurnView{}, err
	}
	author, err = author.MarkRunning(runID, runID, now)
	if err != nil {
		released, _ := reserved.ReleaseTurn(now)
		_ = service.store.SaveAuthorization(ctx, released)
		return DiscussionTurnView{}, err
	}
	if err := service.store.SaveAssistantMessage(ctx, author); err != nil {
		released, _ := reserved.ReleaseTurn(now)
		_ = service.store.SaveAuthorization(ctx, released)
		return DiscussionTurnView{}, err
	}
	operation, created, err := service.store.ReserveOperation(ctx, creationdomain.Operation{
		ID: runID, ProjectID: session.ProjectID, BranchID: session.BranchID, Command: "discuss_candidate",
		IdempotencyKey: messageKey, ArgumentsHash: hashBytes([]byte(text + "\x00" + reserved.ID)),
		FrozenInputDigest: input.UpstreamDigest, Status: creationdomain.OperationReserved, RunID: runID,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		_ = service.failDiscussionTurn(ctx, author, reserved, creationdomain.Operation{}, "启动记录失败", discussionFailureLocal, 0)
		return DiscussionTurnView{}, err
	}
	if !created {
		_ = service.failDiscussionTurn(ctx, author, reserved, operation, "重复的运行请求", discussionFailureLocal, 0)
		return DiscussionTurnView{}, assistantdomain.ErrReplayForbidden
	}
	captured := service.generation()
	lease, release, err := service.coordinator.BeginRun(context.WithoutCancel(ctx), runID)
	if err != nil {
		_ = service.failDiscussionTurn(ctx, author, reserved, operation, "启动失败", discussionFailureLocal, 0)
		return DiscussionTurnView{}, err
	}
	launched := false
	defer func() {
		if !launched {
			release()
		}
	}()
	session.ActiveRunID = runID
	session.ActiveOperationID = operation.ID
	if err := service.store.SaveRunSlot(lease.Context(), session); err != nil {
		_ = service.failDiscussionTurn(lease.Context(), author, reserved, operation, "运行槽冲突", discussionFailureLocal, 0)
		return DiscussionTurnView{}, err
	}
	operation.Status = creationdomain.OperationRunning
	if err := service.store.UpdateOperation(lease.Context(), operation); err != nil {
		_ = service.failDiscussionTurn(lease.Context(), author, reserved, operation, "启动记录失败", discussionFailureLocal, 0)
		_ = service.clearRunSlot(lease.Context(), session.ProjectID, runID)
		return DiscussionTurnView{}, err
	}
	action, err = service.completeDiscussionAuthorizationAction(ctx, action, reserved.ID, runID)
	if err != nil {
		_ = service.failDiscussionTurn(lease.Context(), author, reserved, operation, "授权状态失败", discussionFailureLocal, 0)
		_ = service.clearRunSlot(lease.Context(), session.ProjectID, runID)
		return DiscussionTurnView{}, err
	}
	turnProfile := input.Profile
	turnProfile.RunBudgetUnlimited = false
	turnProfile.MaxRunTokens = reserved.MaxTurnTokens
	_, err = service.kernel.CreateRun(lease.Context(), agentkernel.CreateRequest{
		RunID: runID, ProjectID: session.ProjectID, BranchID: session.BranchID, CanonicalHead: input.ProjectDigest,
		AuthorInstruction: text, WorkflowID: agentassets.StoryCandidateDiscussionWorkflowID,
		WorkflowVersion: agentassets.StoryCandidateDiscussionWorkflowVersion, Profile: turnProfile,
		Sources: sources, Deadline: now.Add(defaultRunTimeout),
	})
	if err != nil {
		_ = service.failDiscussionTurn(lease.Context(), author, reserved, operation, "启动失败", discussionFailureLocal, 0)
		_ = service.clearRunSlot(lease.Context(), session.ProjectID, runID)
		return DiscussionTurnView{}, err
	}
	go func() {
		defer release()
		completed, executeErr := service.kernel.ExecuteRun(lease.Context(), runID, input.ProjectDigest)
		storeCtx := context.WithoutCancel(lease.Context())
		if lease.Stale() || captured != service.generation() {
			return
		}
		mutex := service.lockProject(session.ProjectID)
		mutex.Lock()
		defer mutex.Unlock()
		if lease.Stale() || captured != service.generation() {
			return
		}
		latestAuth, authErr := service.store.Authorization(storeCtx, reserved.ID)
		if authErr != nil || latestAuth.ID == "" {
			return
		}
		latestMsg, msgErr := service.store.AssistantMessageByIdempotency(storeCtx, author.SessionID, author.IdempotencyKey)
		if msgErr != nil || latestMsg.ID == "" {
			return
		}
		if executeErr != nil {
			mode := discussionFailureUnknown
			if discussionProviderReached(completed) {
				mode = discussionFailureReached
			}
			if err := service.failDiscussionTurn(storeCtx, latestMsg, latestAuth, operation, "运行失败或已取消", mode, completed.InputTokens+completed.OutputTokens); err != nil {
				return
			}
		} else {
			if err := service.completeDiscussionTurn(storeCtx, latestMsg, latestAuth, operation, input, completed); err != nil {
				return
			}
		}
		_ = service.clearRunSlot(storeCtx, session.ProjectID, runID)
	}()
	launched = true
	auth, _ = service.store.Authorization(ctx, reserved.ID)
	return service.discussionTurnView(ctx, session, action, auth, author)
}

func (service *Service) completeDiscussionTurn(
	ctx context.Context,
	author assistantdomain.Message,
	auth assistantdomain.Authorization,
	operation creationdomain.Operation,
	input discussionAuthInput,
	run agentdomain.Run,
) error {
	if author.Status != assistantdomain.MessageRunning && author.Status != assistantdomain.MessagePending && author.Status != assistantdomain.MessageOutcomeUnknown && author.Status != assistantdomain.MessageAnswered {
		return assistantdomain.ErrInvalidTransition
	}
	artifacts, err := service.kernel.Artifacts(ctx, run.ID)
	if err != nil {
		return err
	}
	payload, _, err := service.resultPayload(ctx, run, artifacts, discussionBinding())
	if err != nil {
		return service.failDiscussionTurn(ctx, author, auth, operation, "结果无法核验", discussionFailureReached, run.InputTokens+run.OutputTokens)
	}
	output, suggestions, err := decodeDiscussionOutput(payload, input.Base, auth.FieldPaths)
	if err != nil {
		return service.failDiscussionTurn(ctx, author, auth, operation, "结果无法核验", discussionFailureReached, run.InputTokens+run.OutputTokens)
	}
	now := service.clock.Now()
	reply, err := assistantdomain.NewDiscussionMessage(service.ids.NewID(), author.SessionID, assistantdomain.RoleAssistant,
		assistantdomain.MessageAnswered, formatDiscussionReply(output), author.Stage, author.CandidateID, author.EntityRef,
		author.FieldPath, author.ActionID, auth.ID, author.IdempotencyKey+":assistant", now)
	if err != nil {
		return service.failDiscussionTurn(ctx, author, auth, operation, "回复无法保存", discussionFailureReached, run.InputTokens+run.OutputTokens)
	}
	reply = reply.WithReply(author.ID, auth.ID, run.ID, operation.ID).WithSuggestions(suggestions)
	if err := service.store.SaveAssistantMessage(ctx, reply); err != nil {
		return err
	}
	if auth.ReservedCalls > 0 {
		settled, settleErr := auth.SettleTurn(run.InputTokens+run.OutputTokens, now)
		if settleErr != nil {
			return settleErr
		}
		if err := service.store.SaveAuthorization(ctx, settled); err != nil {
			return err
		}
		auth = settled
	}
	if author.Status != assistantdomain.MessageAnswered {
		answered, answerErr := author.MarkAnswered(now)
		if answerErr != nil {
			return answerErr
		}
		if err := service.store.UpdateAssistantMessage(ctx, answered); err != nil {
			return err
		}
	}
	if operation.Status != creationdomain.OperationSucceeded {
		operation.Status = creationdomain.OperationSucceeded
		operation.CandidateID = ""
		operation.ErrorCode = ""
		operation.UpdatedAt = now
		if err := service.store.UpdateOperation(ctx, operation); err != nil {
			return err
		}
	}
	return nil
}

type discussionFailureMode int

const (
	discussionFailureLocal discussionFailureMode = iota
	discussionFailureReached
	discussionFailureUnknown
)

func (service *Service) failDiscussionTurn(ctx context.Context, author assistantdomain.Message, auth assistantdomain.Authorization, operation creationdomain.Operation, code string, mode discussionFailureMode, actualTokens int) error {
	now := service.clock.Now()
	if author.ID != "" {
		var next assistantdomain.Message
		var err error
		if mode == discussionFailureUnknown {
			if author.Status == assistantdomain.MessageOutcomeUnknown {
				next = author
			} else {
				next, err = author.MarkOutcomeUnknown(code, now)
			}
		} else if author.Status == assistantdomain.MessageFailed {
			next = author
		} else {
			next, err = author.MarkFailed(code, now)
		}
		if err != nil {
			return err
		}
		if next.UpdatedAt != author.UpdatedAt || next.Status != author.Status {
			if err := service.store.UpdateAssistantMessage(ctx, next); err != nil {
				return err
			}
		}
	}
	if auth.ID != "" && auth.ReservedCalls > 0 {
		var next assistantdomain.Authorization
		var err error
		switch mode {
		case discussionFailureLocal:
			next, err = auth.ReleaseTurn(now)
		case discussionFailureReached:
			next, err = auth.SettleTurn(actualTokens, now)
		default:
			next, err = auth.KeepReservation(now)
		}
		if err != nil {
			return err
		}
		if err := service.store.SaveAuthorization(ctx, next); err != nil {
			return err
		}
	}
	if operation.ID != "" && mode != discussionFailureUnknown && operation.Status != creationdomain.OperationFailed {
		operation.Status = creationdomain.OperationFailed
		operation.ErrorCode = code
		operation.UpdatedAt = now
		if err := service.store.UpdateOperation(ctx, operation); err != nil {
			return err
		}
	}
	return nil
}

// discussionProviderReached 只用持久化请求 ID 或用量证明 Provider 已收到调用。
func discussionProviderReached(run agentdomain.Run) bool {
	if run.InputTokens+run.OutputTokens > 0 {
		return true
	}
	for _, step := range run.Steps {
		if strings.TrimSpace(step.ProviderRequestID) != "" || step.InputTokens+step.OutputTokens > 0 {
			return true
		}
	}
	return false
}

// completeDiscussionAuthorizationAction 让已落库的授权证据驱动 Action 收敛，供启动和崩溃恢复共用。
func (service *Service) completeDiscussionAuthorizationAction(ctx context.Context, action assistantdomain.Action, authorizationID, runID string) (assistantdomain.Action, error) {
	if action.State == assistantdomain.StateSucceeded {
		return action, nil
	}
	var err error
	if action.State == assistantdomain.StatePendingConfirmation {
		confirmedAt := service.clock.Now()
		// 已存在的活动授权或运行消息证明作者在有效期内完成过确认；恢复时不能因当前时间已过期而丢失该事实。
		if !action.ExpiresAt.IsZero() && !confirmedAt.Before(action.ExpiresAt) {
			confirmedAt = action.ExpiresAt.Add(-time.Nanosecond)
		}
		action, err = action.Confirm(action.ArgumentsHash, action.IdempotencyKey, confirmedAt)
		if err != nil {
			return assistantdomain.Action{}, err
		}
	}
	action, err = action.Succeed([]string{authorizationID, runID}, service.clock.Now())
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if err := service.store.SaveAction(ctx, action); err != nil {
		return assistantdomain.Action{}, err
	}
	return action, nil
}

func (service *Service) recoverDiscussionTurns(ctx context.Context, session creationdomain.Session) error {
	assistant, err := service.store.AssistantSession(ctx, session.ProjectID, session.BranchID)
	if isMissingSession(err) {
		return nil
	}
	if err != nil {
		return err
	}
	messages, err := service.store.ListAssistantMessages(ctx, assistant.ID)
	if err != nil {
		return err
	}
	linkedAuthorizations := map[string]bool{}
	for _, message := range messages {
		if message.Kind != assistantdomain.KindDiscussion || message.Role != assistantdomain.RoleAuthor || message.RunID == "" {
			continue
		}
		linkedAuthorizations[message.AuthorizationID] = true
		if service.coordinator.Live(message.RunID) {
			continue
		}
		auth := assistantdomain.Authorization{}
		if message.AuthorizationID != "" {
			auth, err = service.store.Authorization(ctx, message.AuthorizationID)
			if err != nil {
				return err
			}
		}
		operation, operationErr := service.store.Operation(ctx, message.OperationID)
		if operationErr != nil && !errors.Is(operationErr, ErrOperationNotFound) {
			return operationErr
		}
		action, actionErr := service.store.Action(ctx, message.ActionID)
		if actionErr != nil {
			return actionErr
		}
		if action, actionErr = service.completeDiscussionAuthorizationAction(ctx, action, auth.ID, message.RunID); actionErr != nil {
			return actionErr
		}
		if message.Status == assistantdomain.MessageAnswered && operationErr == nil && operation.Status == creationdomain.OperationSucceeded && auth.ReservedCalls == 0 {
			continue
		}
		if message.Status == assistantdomain.MessageFailed && operationErr == nil && operation.Status == creationdomain.OperationFailed && auth.ReservedCalls == 0 {
			continue
		}
		run, runErr := service.kernel.Run(ctx, message.RunID)
		if runErr != nil {
			if isMissingSession(runErr) {
				if err := service.failDiscussionTurn(ctx, message, auth, operation, "运行未创建", discussionFailureLocal, 0); err != nil {
					return err
				}
				_ = service.clearRunSlot(ctx, session.ProjectID, message.RunID)
				continue
			}
			return runErr
		}
		if errors.Is(operationErr, ErrOperationNotFound) {
			return operationErr
		}
		input, inputErr := decodeDiscussionInput(action)
		if inputErr != nil {
			return inputErr
		}
		switch run.Status {
		case agentdomain.RunCompleted:
			if err := service.completeDiscussionTurn(ctx, message, auth, operation, input, run); err != nil {
				return err
			}
			_ = service.clearRunSlot(ctx, session.ProjectID, run.ID)
		case agentdomain.RunFailed, agentdomain.RunCancelled:
			mode := discussionFailureUnknown
			if discussionProviderReached(run) {
				mode = discussionFailureReached
			}
			if err := service.failDiscussionTurn(ctx, message, auth, operation, "运行已中断", mode, run.InputTokens+run.OutputTokens); err != nil {
				return err
			}
			_ = service.clearRunSlot(ctx, session.ProjectID, run.ID)
		case agentdomain.RunQueued:
			if err := service.failDiscussionTurn(ctx, message, auth, operation, "运行未开始", discussionFailureLocal, 0); err != nil {
				return err
			}
			_ = service.clearRunSlot(ctx, session.ProjectID, run.ID)
		default:
			if err := service.failDiscussionTurn(ctx, message, auth, operation, "运行已中断，请核对结果", discussionFailureUnknown, 0); err != nil {
				return err
			}
			_ = service.clearRunSlot(ctx, session.ProjectID, run.ID)
		}
	}
	auths, err := service.store.ListAuthorizations(ctx, assistant.ID)
	if err != nil {
		return err
	}
	for _, auth := range auths {
		if auth.ReservedCalls == 0 || linkedAuthorizations[auth.ID] {
			continue
		}
		released, releaseErr := auth.ReleaseTurn(service.clock.Now())
		if releaseErr != nil {
			return releaseErr
		}
		if err := service.store.SaveAuthorization(ctx, released); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) discussionCurrent(ctx context.Context, session creationdomain.Session, input discussionAuthInput) error {
	if session.ContentVersion != input.SessionVersion {
		return creationdomain.ErrSessionVersionConflict
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	current, ok := currentCandidate(candidates, input.Base.Stage)
	if !ok || current.ID != input.Base.ID || current.Version != input.Base.Version || current.ContentSHA256 != input.Base.ContentSHA256 ||
		!reflect.DeepEqual(current.LockMap, input.Base.LockMap) ||
		liveUpstreamDigest(session, candidates, input.Base.Stage) != input.UpstreamDigest {
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

func (service *Service) discussionSources(ctx context.Context, _ creationdomain.Session, auth assistantdomain.Authorization, input discussionAuthInput, authorMessage string) ([]agentkernel.Source, error) {
	history, err := service.visibleDiscussionHistory(ctx, auth)
	if err != nil {
		return nil, err
	}
	fields, err := authorizedDiscussionFields(input.Base, auth.FieldPaths)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]any{
		"stage": input.Base.Stage, "candidate_id": input.Base.ID, "candidate_kind": input.Base.Kind,
		"candidate_version": input.Base.Version, "entity_ref": auth.EntityRef, "field_paths": auth.FieldPaths,
		"authorized_fields": fields, "visible_history": history,
		"constraints":    []string{"只提供分析、追问或字段修改建议", "不得修改候选、锁或正式档案", "不得使用未授权字段"},
		"author_message": authorMessage,
	})
	return []agentkernel.Source{{
		Kind: agentdomain.ManifestStoryAsset, ID: "discussion-target:" + input.Base.ID,
		Version: fmt.Sprintf("%d", input.Base.Version), Content: string(payload),
		Priority: 1200, Required: true, SelectionReason: "当前讨论冻结对象与可见历史", Provisional: true,
	}}, nil
}

func (service *Service) visibleDiscussionHistory(ctx context.Context, auth assistantdomain.Authorization) ([]map[string]string, error) {
	messages, err := service.store.ListAssistantMessages(ctx, auth.SessionID)
	if err != nil {
		return nil, err
	}
	history := make([]map[string]string, 0)
	for _, message := range messages {
		if message.AuthorizationID != auth.ID || message.CandidateID != auth.CandidateID || message.Status != assistantdomain.MessageAnswered {
			continue
		}
		history = append(history, map[string]string{"role": string(message.Role), "text": message.Text})
	}
	return history, nil
}

func (service *Service) existingDiscussionTurn(ctx context.Context, session creationdomain.Session, action assistantdomain.Action, auth assistantdomain.Authorization, messageKey, text string) (DiscussionTurnView, error) {
	existing, err := service.store.AssistantMessageByIdempotency(ctx, action.SessionID, messageKey)
	if err != nil {
		if !isMissingSession(err) {
			return DiscussionTurnView{}, err
		}
		return DiscussionTurnView{}, assistantdomain.ErrReplayForbidden
	}
	if existing.Text != text {
		return DiscussionTurnView{}, ErrOperationConflict
	}
	return service.discussionTurnView(ctx, session, action, auth, existing)
}

func (service *Service) discussionPreviewView(ctx context.Context, session creationdomain.Session, action assistantdomain.Action, input discussionAuthInput) (DiscussionTurnView, error) {
	view := DiscussionTurnView{
		ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
		State: string(action.State), DraftMessage: input.Request.Message,
		Authorization: DiscussionAuthorizationView{
			ActionID: action.ID, State: string(action.State), CandidateID: input.Base.ID, CandidateVersion: input.Base.Version,
			EntityRef: input.Request.EntityRef, FieldPaths: append([]string(nil), input.Request.FieldPaths...),
			RemainingCalls: input.MaxCalls, RemainingTokens: input.MaxTotalTokens, MaxCalls: input.MaxCalls,
			MaxTotalTokens: input.MaxTotalTokens, MaxTurnTokens: input.MaxTurnTokens,
			ExpiresAt: time.Unix(input.ExpiresAtUnix, 0).UTC().Format(time.RFC3339),
			Provider:  input.Planner.Provider, Model: input.Planner.Model, Cost: input.Cost,
			Stale: service.discussionCurrent(ctx, session, input) != nil,
		},
	}
	return view, nil
}

func (service *Service) discussionTurnView(ctx context.Context, session creationdomain.Session, action assistantdomain.Action, auth assistantdomain.Authorization, author assistantdomain.Message) (DiscussionTurnView, error) {
	input, _ := decodeDiscussionInput(action)
	view := DiscussionTurnView{
		ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
		State: string(auth.State), RunID: author.RunID, Author: projectAssistantMessage(author, string(action.State)),
		Authorization: projectAuthorization(auth, input.Cost, service.discussionCurrent(ctx, session, input) != nil, service.clock.Now()),
	}
	messages, err := service.store.ListAssistantMessages(ctx, author.SessionID)
	if err != nil {
		return view, err
	}
	for _, message := range messages {
		if message.ReplyToID == author.ID && message.Role == assistantdomain.RoleAssistant {
			reply := projectAssistantMessage(message, string(action.State))
			view.Reply = &reply
			break
		}
	}
	return view, nil
}

func decodeDiscussionInput(action assistantdomain.Action) (discussionAuthInput, error) {
	var input discussionAuthInput
	if json.Unmarshal([]byte(action.Arguments), &input) != nil || input.Contract != discussionContract || hashBytes([]byte(action.Arguments)) != action.ArgumentsHash {
		return input, ErrUnavailable
	}
	return input, nil
}

func decodeDiscussionOutput(payload string, base creationdomain.Candidate, authorizedPaths []string) (discussionModelOutput, []assistantdomain.Suggestion, error) {
	var output discussionModelOutput
	if json.Unmarshal([]byte(payload), &output) != nil || strings.TrimSpace(output.Answer) == "" || utf8.RuneCountInString(output.Answer) > 4000 {
		return output, nil, ErrUnavailable
	}
	candidatePaths := map[string]bool{}
	for _, field := range candidateTextFields(base).Fields {
		candidatePaths[field.Path] = true
	}
	allowed := map[string]bool{}
	for _, path := range authorizedPaths {
		allowed[path] = candidatePaths[path] && !base.Locked(path)
	}
	suggestions := make([]assistantdomain.Suggestion, 0, len(output.Suggestions))
	seen := map[string]bool{}
	for _, item := range output.Suggestions {
		if strings.TrimSpace(item.ID) == "" || seen[item.ID] || item.Operation != string(assistantdomain.OpReviseFields) {
			continue
		}
		if strings.TrimSpace(item.Instruction) == "" || utf8.RuneCountInString(item.Instruction) > 4000 {
			continue
		}
		if err := validateSuggestionPaths(allowed, candidatePaths, item.Paths, item.PreservePaths); err != nil {
			continue
		}
		seen[item.ID] = true
		suggestions = append(suggestions, assistantdomain.Suggestion{
			ID: item.ID, Operation: assistantdomain.OpReviseFields, Paths: append([]string(nil), item.Paths...),
			Instruction: strings.TrimSpace(item.Instruction), PreservePaths: append([]string(nil), item.PreservePaths...),
			Rationale: strings.TrimSpace(item.Rationale),
		})
	}
	return output, suggestions, nil
}

func sameFieldPaths(left, right []string) bool {
	a := append([]string(nil), left...)
	b := append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	return slices.Equal(a, b)
}

func validateSuggestionPaths(allowed, candidatePaths map[string]bool, paths, preserve []string) error {
	if len(paths) == 0 {
		return creationdomain.ErrBlankValue
	}
	used := map[string]bool{}
	for _, path := range paths {
		if !allowed[path] || used[path] {
			return creationdomain.ErrFieldLocked
		}
		used[path] = true
	}
	for _, path := range preserve {
		if used[path] || !candidatePaths[path] {
			return creationdomain.ErrBlankValue
		}
	}
	return nil
}

func validateDiscussionPaths(base creationdomain.Candidate, paths []string) error {
	if len(paths) == 0 {
		return creationdomain.ErrBlankValue
	}
	allowed := map[string]bool{}
	for _, field := range candidateTextFields(base).Fields {
		allowed[field.Path] = !field.Locked
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if !allowed[path] || seen[path] {
			return creationdomain.ErrFieldLocked
		}
		seen[path] = true
	}
	return nil
}

func normalizeDiscussionBudget(request *DiscussionAuthorizationRequest) {
	if request.MaxCalls < 1 {
		request.MaxCalls = defaultDiscussionCalls
	}
	if request.MaxTurnTokens < 1 {
		request.MaxTurnTokens = defaultDiscussionTurnTokens
	}
	if request.MaxTotalTokens < 1 {
		request.MaxTotalTokens = defaultDiscussionTotalTokens
	}
	if request.MaxTurnTokens > request.MaxTotalTokens {
		request.MaxTurnTokens = request.MaxTotalTokens
	}
	if request.MaxCalls > 20 {
		request.MaxCalls = 20
	}
}

// authorizedDiscussionFields 只提取授权路径对应的当前文本，避免把候选其余字段送入讨论上下文。
func authorizedDiscussionFields(base creationdomain.Candidate, paths []string) ([]map[string]any, error) {
	fields := candidateTextFields(base).Fields
	byPath := make(map[string]TextFieldView, len(fields))
	for _, field := range fields {
		byPath[field.Path] = field
	}
	result := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		field, ok := byPath[path]
		if !ok {
			return nil, creationdomain.ErrFieldMissing
		}
		result = append(result, map[string]any{"path": field.Path, "label": field.Label, "value": field.Value, "locked": field.Locked})
	}
	return result, nil
}

func plannerRole(profile agentdomain.ResolvedCreativeProfile) (RoleModelView, error) {
	for _, role := range profile.Roles {
		if role.Role == agentdomain.RolePlanner {
			return RoleModelView{Role: string(role.Role), Provider: role.ProviderName, Model: role.Model}, nil
		}
	}
	return RoleModelView{}, ErrUnavailable
}

func plannerConnection(profile agentdomain.ResolvedCreativeProfile) string {
	for _, role := range profile.Roles {
		if role.Role == agentdomain.RolePlanner {
			return role.ConnectionID
		}
	}
	return ""
}

func plannerModel(profile agentdomain.ResolvedCreativeProfile) string {
	for _, role := range profile.Roles {
		if role.Role == agentdomain.RolePlanner {
			return role.Model
		}
	}
	return ""
}

func plannerEffort(profile agentdomain.ResolvedCreativeProfile) string {
	for _, role := range profile.Roles {
		if role.Role == agentdomain.RolePlanner {
			return string(role.ReasoningEffort)
		}
	}
	return ""
}

func discussionProfileID(profile agentdomain.ResolvedCreativeProfile) string {
	return fmt.Sprintf("%s@%d", profile.ProfileID, profile.Version)
}

func primaryFieldPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func lockedPaths(base creationdomain.Candidate) []string {
	paths := make([]string, 0)
	for path, locked := range base.LockMap {
		if locked {
			paths = append(paths, path)
		}
	}
	return paths
}

func formatDiscussionReply(output discussionModelOutput) string {
	text := strings.TrimSpace(output.Answer)
	if len(output.FollowUpQuestions) == 0 {
		return text
	}
	return text + "\n\n还可以继续确认：\n- " + strings.Join(output.FollowUpQuestions, "\n- ")
}

func projectAuthorization(auth assistantdomain.Authorization, cost CostPreview, stale bool, now time.Time) DiscussionAuthorizationView {
	state := auth.State
	if state == assistantdomain.AuthActive && !auth.ExpiresAt.IsZero() && !now.UTC().Before(auth.ExpiresAt) {
		state = assistantdomain.AuthExpired
	}
	return DiscussionAuthorizationView{
		ID: auth.ID, ActionID: auth.ActionID, State: string(state), CandidateID: auth.CandidateID,
		CandidateVersion: auth.CandidateVersion, EntityRef: auth.EntityRef, FieldPaths: append([]string(nil), auth.FieldPaths...),
		RemainingCalls: auth.RemainingCalls(), RemainingTokens: auth.RemainingTokens(), MaxCalls: auth.MaxCalls,
		MaxTotalTokens: auth.MaxTotalTokens, MaxTurnTokens: auth.MaxTurnTokens,
		ExpiresAt: auth.ExpiresAt.Format(time.RFC3339), Provider: auth.ProviderName, Model: auth.Model,
		Cost: cost, Stale: stale || auth.State == assistantdomain.AuthStale,
	}
}

func projectAssistantMessage(message assistantdomain.Message, actionState string) AssistantMessageView {
	suggestions := make([]DiscussionSuggestionView, 0, len(message.Suggestions))
	for _, item := range message.Suggestions {
		suggestions = append(suggestions, DiscussionSuggestionView{
			ID: item.ID, Paths: append([]string(nil), item.Paths...), Instruction: item.Instruction,
			PreservePaths: append([]string(nil), item.PreservePaths...), Rationale: item.Rationale,
		})
	}
	return AssistantMessageView{
		ID: message.ID, Role: string(message.Role), Kind: string(message.Kind), Status: string(message.Status),
		Text: message.Text, Stage: message.Stage, CandidateID: message.CandidateID, EntityRef: message.EntityRef,
		FieldPath: message.FieldPath, ActionID: message.ActionID, ActionState: actionState,
		AuthorizationID: message.AuthorizationID, RunID: message.RunID, ReplyToID: message.ReplyToID,
		ErrorCode: message.ErrorCode, Suggestions: suggestions, CreatedAt: message.CreatedAt.Format(time.RFC3339Nano),
	}
}
