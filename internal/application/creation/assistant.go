package creation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"github.com/lishen802/novel-studio/internal/quality"
)

const assistantActionTTL = time.Hour

type Explanation struct {
	Text      string
	TargetIDs []string
}

type ProposeRequest struct {
	ProjectID      string
	BranchID       string
	ChapterID      string
	CanonicalHead  string
	Profile        agentdomain.ResolvedCreativeProfile
	Operation      assistantdomain.Operation
	CandidateID    string
	Paths          []string
	Instruction    string
	Locks          map[string]bool
	IdempotencyKey string
}

type ActionConfirmRequest struct {
	ProjectID      string
	ActionID       string
	ArgumentsHash  string
	IdempotencyKey string
}

type ActionResult struct {
	Action    assistantdomain.Action
	Candidate creationdomain.Candidate
}

type AssistantProjection struct {
	Session assistantdomain.Session
	Pending []assistantdomain.Action
}

type actionArguments struct {
	CandidateID   string                              `json:"candidateId"`
	Paths         []string                            `json:"paths,omitempty"`
	Instruction   string                              `json:"instruction,omitempty"`
	Locks         map[string]bool                     `json:"locks,omitempty"`
	BranchID      string                              `json:"branchId"`
	ChapterID     string                              `json:"chapterId"`
	CanonicalHead string                              `json:"canonicalHead"`
	Profile       agentdomain.ResolvedCreativeProfile `json:"profile"`
}

// Explain 返回只读比较说明，不创建付费或写入 Action。
func (service *Service) Explain(ctx context.Context, projectID string, candidateIDs []string) (Explanation, error) {
	if len(candidateIDs) == 0 {
		return Explanation{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return Explanation{}, err
	}
	texts := make([]string, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		candidate, err := service.store.Candidate(ctx, id)
		if err != nil {
			return Explanation{}, err
		}
		if candidate.SessionID != session.ID {
			return Explanation{}, creationdomain.ErrBlankValue
		}
		texts = append(texts, candidate.ID)
	}
	return Explanation{
		Text:      "只读比较当前候选差异，未形成待确认操作。",
		TargetIDs: texts,
	}, nil
}

// AssistantProjection 返回当前作品对话会话与待确认 Action。
func (service *Service) AssistantProjection(ctx context.Context, projectID, branchID string) (AssistantProjection, error) {
	session, err := service.store.AssistantSession(ctx, projectID, branchID)
	if err != nil {
		if isMissingSession(err) {
			return AssistantProjection{}, nil
		}
		return AssistantProjection{}, err
	}
	actions, err := service.store.ListActions(ctx, session.ID)
	if err != nil {
		return AssistantProjection{}, err
	}
	pending := make([]assistantdomain.Action, 0)
	now := service.clock.Now()
	for _, action := range actions {
		if action.State == assistantdomain.StatePendingConfirmation && !action.ExpiresAt.IsZero() && !now.Before(action.ExpiresAt) {
			expired, expireErr := action.Expire(now)
			if expireErr == nil {
				_ = service.store.SaveAction(ctx, expired)
				action = expired
			}
		}
		if action.State == assistantdomain.StatePendingConfirmation {
			pending = append(pending, action)
		}
	}
	return AssistantProjection{Session: session, Pending: pending}, nil
}

// ProposeAction 把付费、改锁或写入操作冻结为待确认 Action，不调用模型。
func (service *Service) ProposeAction(ctx context.Context, request ProposeRequest) (assistantdomain.Action, error) {
	if !assistantdomain.RequiresAction(request.Operation) || !assistantdomain.AllowedAsAssistantOperation(request.Operation) {
		return assistantdomain.Action{}, assistantdomain.ErrInvalidOperation
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" || strings.TrimSpace(request.CandidateID) == "" {
		return assistantdomain.Action{}, creationdomain.ErrBlankValue
	}
	session, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	assistantSession, err := service.ensureAssistantSession(ctx, session)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if existing, loadErr := service.store.ActionByIdempotency(ctx, assistantSession.ID, request.IdempotencyKey); loadErr == nil {
		return existing, nil
	}
	candidate, err := service.store.Candidate(ctx, request.CandidateID)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if err := validatePropose(request, candidate); err != nil {
		return assistantdomain.Action{}, err
	}
	args := actionArguments{
		CandidateID: candidate.ID, Paths: append([]string(nil), request.Paths...),
		Instruction: strings.TrimSpace(request.Instruction), Locks: request.Locks,
		BranchID: request.BranchID, ChapterID: request.ChapterID, CanonicalHead: request.CanonicalHead,
		Profile: request.Profile,
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	now := service.clock.Now()
	action, err := assistantdomain.NewAction(
		service.ids.NewID(), assistantSession.ID, session.ProjectID, session.BranchID,
		request.Operation, assistantdomain.RiskFor(request.Operation),
		[]string{"candidate:" + candidate.ID},
		[]string{fmt.Sprintf("%s@%d", candidate.ID, candidate.Version)},
		string(encoded), hashBytes(encoded), request.IdempotencyKey, "",
		now, now.Add(assistantActionTTL),
	)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if err := service.store.SaveAction(ctx, action); err != nil {
		return assistantdomain.Action{}, err
	}
	return action, nil
}

// ConfirmAction 校验摘要与幂等键后执行 Action；outcome_unknown 禁止重放。
func (service *Service) ConfirmAction(ctx context.Context, request ActionConfirmRequest) (ActionResult, error) {
	action, err := service.store.Action(ctx, request.ActionID)
	if err != nil {
		return ActionResult{}, err
	}
	if action.ProjectID != request.ProjectID {
		return ActionResult{}, creationdomain.ErrBlankValue
	}
	now := service.clock.Now()
	if action.State == assistantdomain.StateSucceeded {
		result := ActionResult{Action: action}
		if len(action.ResultRefs) > 0 {
			if candidate, loadErr := service.store.Candidate(ctx, action.ResultRefs[0]); loadErr == nil {
				result.Candidate = candidate
			}
		}
		return result, nil
	}
	if action.State == assistantdomain.StateOutcomeUnknown || !action.CanReplay() && action.State != assistantdomain.StatePendingConfirmation {
		if action.State == assistantdomain.StateOutcomeUnknown {
			return ActionResult{}, assistantdomain.ErrReplayForbidden
		}
		return ActionResult{}, assistantdomain.ErrInvalidTransition
	}
	if !action.ExpiresAt.IsZero() && !now.Before(action.ExpiresAt) {
		expired, expireErr := action.Expire(now)
		if expireErr != nil {
			return ActionResult{}, expireErr
		}
		if err := service.store.SaveAction(ctx, expired); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{}, assistantdomain.ErrExpired
	}
	executing, err := action.Confirm(request.ArgumentsHash, request.IdempotencyKey, now)
	if err != nil {
		return ActionResult{}, err
	}
	if err := service.store.SaveAction(ctx, executing); err != nil {
		return ActionResult{}, err
	}
	result, err := service.executeAction(ctx, executing)
	if err != nil {
		unknown, markErr := executing.MarkOutcomeUnknown("execute_failed", service.clock.Now())
		if markErr == nil {
			_ = service.store.SaveAction(ctx, unknown)
		}
		return ActionResult{}, err
	}
	return result, nil
}

// RejectAction 拒绝待确认 Action，不执行写入或付费调用。
func (service *Service) RejectAction(ctx context.Context, projectID, actionID string) (assistantdomain.Action, error) {
	action, err := service.store.Action(ctx, actionID)
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if action.ProjectID != projectID {
		return assistantdomain.Action{}, creationdomain.ErrBlankValue
	}
	rejected, err := action.Reject(service.clock.Now())
	if err != nil {
		return assistantdomain.Action{}, err
	}
	if err := service.store.SaveAction(ctx, rejected); err != nil {
		return assistantdomain.Action{}, err
	}
	return rejected, nil
}

func (service *Service) ensureAssistantSession(ctx context.Context, creationSession creationdomain.Session) (assistantdomain.Session, error) {
	session, err := service.store.AssistantSession(ctx, creationSession.ProjectID, creationSession.BranchID)
	if err == nil {
		return session, nil
	}
	if !isMissingSession(err) {
		return assistantdomain.Session{}, err
	}
	session, err = assistantdomain.NewSession(service.ids.NewID(), creationSession.ProjectID, creationSession.BranchID, service.clock.Now())
	if err != nil {
		return assistantdomain.Session{}, err
	}
	if err := service.store.SaveAssistantSession(ctx, session); err != nil {
		return assistantdomain.Session{}, err
	}
	return session, nil
}

func validatePropose(request ProposeRequest, candidate creationdomain.Candidate) error {
	switch request.Operation {
	case assistantdomain.OpReviseFields:
		if strings.TrimSpace(request.Instruction) == "" || len(request.Paths) == 0 {
			return creationdomain.ErrBlankValue
		}
		for _, path := range request.Paths {
			if !assistantdomain.AllowedFieldPath(string(candidate.Kind), path) {
				return assistantdomain.ErrFieldPath
			}
			if candidate.Locked(path) {
				return creationdomain.ErrFieldLocked
			}
		}
	case assistantdomain.OpEditLocks:
		if len(request.Locks) == 0 {
			return creationdomain.ErrBlankValue
		}
		for path := range request.Locks {
			if _, err := candidate.FieldString(path); err != nil && !request.Locks[path] {
				return err
			}
			if request.Locks[path] {
				if _, err := candidate.FieldString(path); err != nil {
					return err
				}
			}
		}
	case assistantdomain.OpRegenerateCandidate, assistantdomain.OpCombineDirections:
		if strings.TrimSpace(request.BranchID) == "" || strings.TrimSpace(request.CanonicalHead) == "" {
			return creationdomain.ErrBlankValue
		}
	}
	return nil
}

func (service *Service) executeAction(ctx context.Context, action assistantdomain.Action) (ActionResult, error) {
	var args actionArguments
	if err := json.Unmarshal([]byte(action.Arguments), &args); err != nil {
		return ActionResult{}, err
	}
	candidate, err := service.store.Candidate(ctx, args.CandidateID)
	if err != nil {
		return ActionResult{}, err
	}
	if !baseVersionMatches(action.BaseVersions, candidate) {
		conflicted, conflictErr := action.Conflict("base_changed", service.clock.Now())
		if conflictErr == nil {
			_ = service.store.SaveAction(ctx, conflicted)
			action = conflicted
		}
		return ActionResult{}, creationdomain.ErrInvalidTransition
	}
	switch action.Operation {
	case assistantdomain.OpEditLocks:
		return service.executeEditLocks(ctx, action, candidate, args)
	case assistantdomain.OpReviseFields:
		return service.executeReviseFields(ctx, action, candidate, args)
	case assistantdomain.OpRegenerateCandidate, assistantdomain.OpCombineDirections:
		return service.executeRegenerate(ctx, action, candidate, args)
	default:
		return ActionResult{}, assistantdomain.ErrInvalidOperation
	}
}

func (service *Service) executeEditLocks(
	ctx context.Context,
	action assistantdomain.Action,
	candidate creationdomain.Candidate,
	args actionArguments,
) (ActionResult, error) {
	now := service.clock.Now()
	expected := candidate
	updated := candidate
	var err error
	for path, locked := range args.Locks {
		if locked {
			updated, err = updated.LockField(path, now)
		} else {
			updated, err = updated.UnlockField(path, now)
		}
		if err != nil {
			return ActionResult{}, err
		}
	}
	if err := service.store.ApplyCandidateChanges(ctx,
		[]creationdomain.Candidate{expected}, []creationdomain.Candidate{updated}, nil,
	); err != nil {
		return ActionResult{}, err
	}
	succeeded, err := action.Succeed([]string{updated.ID}, now)
	if err != nil {
		return ActionResult{}, err
	}
	if err := service.store.SaveAction(ctx, succeeded); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: succeeded, Candidate: updated}, nil
}

func (service *Service) executeReviseFields(
	ctx context.Context,
	action assistantdomain.Action,
	candidate creationdomain.Candidate,
	args actionArguments,
) (ActionResult, error) {
	directive, err := buildAuthorDirective(candidate, args)
	if err != nil {
		return ActionResult{}, err
	}
	// 物化不可变基线：模型产物的候选引用原始 Artifact，人工版本使用内容摘要作为基线身份。
	baseDigest := candidate.ContentSHA256
	if !agentdomain.ValidSHA256(baseDigest) {
		sum := sha256.Sum256([]byte(candidate.PayloadJSON))
		baseDigest = hex.EncodeToString(sum[:])
	}
	baseArtifactID := candidate.ArtifactID
	if strings.TrimSpace(baseArtifactID) == "" {
		baseArtifactID = "base:" + baseDigest
	}
	base := &agentdomain.BaseCandidate{
		ArtifactID: baseArtifactID, Digest: baseDigest, Content: candidate.PayloadJSON, Blueprint: true,
	}
	captured := service.generation()
	runID := service.ids.NewID()
	lease, release, err := service.coordinator.BeginRun(ctx, runID)
	if err != nil {
		return ActionResult{}, err
	}
	defer release()
	created, err := service.kernel.CreateRun(lease.Context(), agentkernel.CreateRequest{
		RunID: runID, ProjectID: action.ProjectID, BranchID: args.BranchID, ChapterID: args.ChapterID,
		ChapterBaseRevision: 1, CanonicalHead: args.CanonicalHead, AuthorInstruction: args.Instruction,
		WorkflowID: agentassets.StoryAuthorReviseWorkflowID, WorkflowVersion: agentassets.StoryAuthorReviseWorkflowVersion,
		Profile: args.Profile, Sources: []agentkernel.Source{}, QualitySpec: quality.Spec{},
		RevisionDirective: &directive, BaseCandidate: base, Deadline: service.clock.Now().Add(defaultRunTimeout),
	})
	if err != nil {
		return ActionResult{}, err
	}
	completed, err := service.kernel.ExecuteRun(lease.Context(), created.ID, args.CanonicalHead)
	if err != nil {
		return ActionResult{}, err
	}
	storeCtx := context.WithoutCancel(lease.Context())
	if service.generation() != captured || lease.Stale() {
		return ActionResult{}, ErrStaleStore
	}
	artifacts, err := service.kernel.Artifacts(storeCtx, completed.ID)
	if err != nil {
		return ActionResult{}, err
	}
	payload, artifactID, err := service.resultPayload(storeCtx, completed, artifacts, authorReviseBinding())
	if err != nil {
		return ActionResult{}, err
	}
	next, err := candidate.Regenerate(service.ids.NewID(), completed.ID, artifactID, payload, service.clock.Now())
	if err != nil {
		return ActionResult{}, err
	}
	superseded, err := candidate.MarkSuperseded(service.clock.Now())
	if err != nil {
		return ActionResult{}, err
	}
	if err := service.store.ApplyCandidateChanges(ctx,
		[]creationdomain.Candidate{candidate}, []creationdomain.Candidate{superseded}, []creationdomain.Candidate{next},
	); err != nil {
		return ActionResult{}, err
	}
	succeeded, err := action.Succeed([]string{next.ID}, service.clock.Now())
	if err != nil {
		return ActionResult{}, err
	}
	if err := service.store.SaveAction(ctx, succeeded); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: succeeded, Candidate: next}, nil
}

func (service *Service) executeRegenerate(
	ctx context.Context,
	action assistantdomain.Action,
	candidate creationdomain.Candidate,
	args actionArguments,
) (ActionResult, error) {
	runID := service.ids.NewID()
	captured := service.generation()
	lease, release, err := service.coordinator.BeginRun(ctx, runID)
	if err != nil {
		return ActionResult{}, err
	}
	defer release()
	session, err := service.loadSession(ctx, action.ProjectID)
	if err != nil {
		return ActionResult{}, err
	}
	request := GenerateRequest{
		ProjectID: action.ProjectID, BranchID: args.BranchID, ChapterID: args.ChapterID,
		CanonicalHead: args.CanonicalHead, Stage: candidate.Stage, Profile: args.Profile,
	}
	// 短锁内检查并占用运行槽；模型执行在锁外进行，不阻塞其他命令。
	var frozen string
	latest := session
	run, err := func() (agentdomain.Run, error) {
		unlock := service.lockProject(action.ProjectID)
		unlock.Lock()
		defer unlock.Unlock()
		loaded, err := service.loadSession(ctx, action.ProjectID)
		if err != nil {
			return agentdomain.Run{}, err
		}
		if strings.TrimSpace(loaded.ActiveRunID) != "" {
			return agentdomain.Run{}, ErrRunActive
		}
		candidates, err := service.store.ListCandidates(ctx, session.ID)
		if err != nil {
			return agentdomain.Run{}, err
		}
		sources, err := generateSources(loaded, candidates, candidate.Stage)
		if err != nil {
			return agentdomain.Run{}, err
		}
		latest = loaded
		frozen = liveUpstreamDigest(loaded, candidates, candidate.Stage)
		return service.startStageRun(lease.Context(), runID, action.ID, loaded, request, candidate.Stage, sources)
	}()
	if err != nil {
		return ActionResult{}, err
	}
	result, err := service.executeStageRun(lease, latest, request, candidate.Stage, run.ID, frozen, captured)
	if service.generation() != captured || lease.Stale() {
		return ActionResult{}, ErrStaleStore
	}
	if err != nil {
		service.clearRunSlot(context.WithoutCancel(lease.Context()), action.ProjectID, run.ID)
		return ActionResult{}, err
	}
	service.clearRunSlot(context.WithoutCancel(lease.Context()), action.ProjectID, run.ID)
	succeeded, err := action.Succeed([]string{result.Candidate.ID}, service.clock.Now())
	if err != nil {
		return ActionResult{}, err
	}
	if err := service.store.SaveAction(ctx, succeeded); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: succeeded, Candidate: result.Candidate}, nil
}

func buildAuthorDirective(candidate creationdomain.Candidate, args actionArguments) (agentdomain.RevisionDirective, error) {
	targets := make([]string, 0, len(args.Paths))
	for _, path := range args.Paths {
		key := strings.TrimPrefix(path, "/")
		if key == "" {
			return agentdomain.RevisionDirective{}, assistantdomain.ErrFieldPath
		}
		targets = append(targets, key)
	}
	preserve := make([]string, 0)
	for path, locked := range candidate.LockMap {
		if !locked {
			continue
		}
		key := strings.TrimPrefix(path, "/")
		overlap := false
		for _, target := range targets {
			if target == key {
				overlap = true
				break
			}
		}
		if !overlap {
			preserve = append(preserve, key)
		}
	}
	digest := candidate.ContentSHA256
	if !agentdomain.ValidSHA256(digest) {
		sum := sha256.Sum256([]byte(candidate.PayloadJSON))
		digest = hex.EncodeToString(sum[:])
	}
	directive := agentdomain.RevisionDirective{
		Version: 1, Mode: agentdomain.RevisionAuthorDirected,
		BaseArtifactID: candidate.ArtifactID, BaseDigest: digest,
		TargetRefs: targets, PreserveRefs: preserve, AuthorInstructionRef: args.Instruction,
	}
	if err := directive.Validate(); err != nil {
		return agentdomain.RevisionDirective{}, err
	}
	return directive, nil
}

func baseVersionMatches(baseVersions []string, candidate creationdomain.Candidate) bool {
	want := fmt.Sprintf("%s@%d", candidate.ID, candidate.Version)
	for _, item := range baseVersions {
		if item == want {
			return true
		}
	}
	return false
}

func hashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
