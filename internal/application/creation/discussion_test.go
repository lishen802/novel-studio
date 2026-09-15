package creation

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestDiscussionPreviewDoesNotCallModel(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	view, err := service.PreviewDiscussionAuthorization(context.Background(), "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/protagonist_arc"}, IdempotencyKey: "discuss-preview",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if view.ActionID == "" || view.State != string(assistantdomain.StatePendingConfirmation) || kernel.createN != 0 || kernel.executeN != 0 {
		t.Fatalf("preview started a run: %+v create=%d execute=%d", view, kernel.createN, kernel.executeN)
	}
	if view.Authorization.Model == "" || view.Authorization.Cost.EstimatedCalls < 1 {
		t.Fatalf("preview missing frozen model or budget: %+v", view.Authorization)
	}
}

func TestDiscussionConfirmStartsAsyncAndReusesAuthorization(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	kernel.inputTokens = 80
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, EntityRef: "proposal:arc", FieldPaths: []string{"/protagonist_arc"}, IdempotencyKey: "discuss-auth",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "这个人物的动机哪里不够可信？", MessageIdempotencyKey: "discuss-msg-1",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if first.RunID == "" || first.Author.Text == "" {
		t.Fatalf("confirm did not return a running turn: %+v", first)
	}
	replay, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "这个人物的动机哪里不够可信？", MessageIdempotencyKey: "discuss-msg-1",
	}, discussionProfile())
	if err != nil || replay.Author.ID != first.Author.ID {
		t.Fatalf("double confirm created another author message: %+v err=%v", replay, err)
	}
	if _, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "换一个问题", MessageIdempotencyKey: "discuss-msg-1",
	}, discussionProfile()); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("same key different text error = %v", err)
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("first discussion turn did not answer")
	}
	if kernel.createN != 1 || kernel.lastCreate.WorkflowID != agentassets.StoryCandidateDiscussionWorkflowID || kernel.lastCreate.ChapterID != "" {
		t.Fatalf("unexpected kernel create: %#v n=%d", kernel.lastCreate, kernel.createN)
	}
	if kernel.lastCreate.Profile.RunBudgetUnlimited || kernel.lastCreate.Profile.MaxRunTokens != first.Authorization.MaxTurnTokens {
		t.Fatalf("run budget exceeded authorization: profile=%+v auth=%+v", kernel.lastCreate.Profile, first.Authorization)
	}
	if len(kernel.lastCreate.Sources) != 1 || !strings.Contains(kernel.lastCreate.Sources[0].Content, "/protagonist_arc") || strings.Contains(kernel.lastCreate.Sources[0].Content, "core_conflict") {
		t.Fatalf("discussion sources escaped authorized scope: %#v", kernel.lastCreate.Sources)
	}
	auth, err := store.AuthorizationByAction(ctx, preview.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.SendDiscussionMessage(ctx, "project-1", DiscussionMessageRequest{
		AuthorizationID: auth.ID, CandidateID: base.ID, EntityRef: "proposal:arc",
		FieldPaths: []string{"/protagonist_arc"}, Text: "比较这两个冲突方向的长期风险。", IdempotencyKey: "discuss-msg-2",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if second.Authorization.ID != auth.ID {
		t.Fatal("second turn did not reuse the same authorization")
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("second discussion turn did not answer")
	}
	if kernel.createN != 2 {
		t.Fatalf("second turn createN = %d", kernel.createN)
	}
	candidates, err := store.ListCandidates(ctx, base.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(candidates, creationdomain.StageProposal)
	if !ok || current.ID != base.ID || current.Version != 1 {
		t.Fatalf("discussion mutated candidate: %+v", current)
	}
	workbench, err := service.Workbench(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if workbench.Assistant.Authorization == nil || workbench.Assistant.Authorization.RemainingCalls != 6 {
		t.Fatalf("workbench authorization = %+v", workbench.Assistant.Authorization)
	}
}

func TestDiscussionRejectsSuggestionsOutsideAuthorization(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	base := seedProposalCandidate(t, service, store)
	_, suggestions, err := decodeDiscussionOutput(discussionPayload, base, []string{"/title"})
	if err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 0 {
		t.Fatalf("out-of-scope suggestions = %#v", suggestions)
	}
}

func TestDiscussionAuthorizationBecomesStaleWhenLocksChange(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, Message: "标题是否准确？", IdempotencyKey: "lock-change",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LockCandidateField(ctx, "project-1", base.ID, "/protagonist_arc"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "标题是否准确？", MessageIdempotencyKey: "lock-change-message",
	}, discussionProfile()); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
		t.Fatalf("lock change error = %v", err)
	}
	if kernel.createN != 0 {
		t.Fatalf("lock change started %d runs", kernel.createN)
	}
}

func TestDiscussionPendingAuthorizationIsProjectedAfterReload(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, Message: "标题是否准确？", IdempotencyKey: "pending-reload",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	workbench, err := service.Workbench(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if workbench.Assistant.PendingDiscussion == nil || workbench.Assistant.PendingDiscussion.ActionID != preview.ActionID || workbench.Assistant.PendingDiscussion.DraftMessage != "标题是否准确？" {
		t.Fatalf("pending discussion = %#v", workbench.Assistant.PendingDiscussion)
	}
}

func TestDiscussionRejectsTargetChangeAndPendingRevisionDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	revision, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{
		CandidateID: base.ID, Paths: []string{"/title"}, Instruction: "标题更冷", IdempotencyKey: "pending-revision",
	}, revisionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if revision.State != string(assistantdomain.StatePendingConfirmation) {
		t.Fatalf("revision = %+v", revision)
	}
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, EntityRef: "proposal:arc", FieldPaths: []string{"/protagonist_arc"}, IdempotencyKey: "discuss-pending",
	}, discussionProfile())
	if err != nil || kernel.createN != 0 {
		t.Fatalf("pending revision blocked read-only preview: %+v err=%v calls=%d", preview, err, kernel.createN)
	}
	turn, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "这条世界规则会限制后面的剧情吗？", MessageIdempotencyKey: "discuss-pending-1",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("discussion did not finish beside pending revision")
	}
	auth, err := store.Authorization(ctx, turn.Authorization.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SendDiscussionMessage(ctx, "project-1", DiscussionMessageRequest{
		AuthorizationID: auth.ID, CandidateID: base.ID, EntityRef: "character:other",
		FieldPaths: []string{"/protagonist_arc"}, Text: "换一个对象继续问", IdempotencyKey: "discuss-pending-2",
	}, discussionProfile()); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
		t.Fatalf("retarget error = %v", err)
	}
}

func TestDiscussionSuggestionBecomesRevisionActionWithoutAutoExecute(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/protagonist_arc"}, IdempotencyKey: "discuss-suggest",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "动机哪里不够可信？", MessageIdempotencyKey: "discuss-suggest-1",
	}, discussionProfile()); err != nil {
		t.Fatal(err)
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("discussion suggestion did not arrive")
	}
	assistant, err := store.AssistantSession(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := store.ListAssistantMessages(ctx, assistant.ID)
	if err != nil {
		t.Fatal(err)
	}
	var reply assistantdomain.Message
	for _, message := range messages {
		if message.Role == assistantdomain.RoleAssistant && len(message.Suggestions) > 0 {
			reply = message
		}
	}
	if reply.ID == "" {
		t.Fatal("missing suggestion reply")
	}
	creates := kernel.createN
	first, err := service.PreviewDiscussionSuggestion(ctx, "project-1", DiscussionSuggestionRequest{
		MessageID: reply.ID, SuggestionID: reply.Suggestions[0].ID,
	}, revisionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if first.State != string(assistantdomain.StatePendingConfirmation) || kernel.createN != creates {
		t.Fatalf("suggestion auto-started revision: %+v createN=%d", first, kernel.createN)
	}
	second, err := service.PreviewDiscussionSuggestion(ctx, "project-1", DiscussionSuggestionRequest{
		MessageID: reply.ID, SuggestionID: reply.Suggestions[0].ID,
	}, revisionProfile())
	if err != nil || second.ActionID != first.ActionID {
		t.Fatalf("duplicate suggestion created another action: %+v err=%v", second, err)
	}
	if _, err := service.LockCandidateField(ctx, "project-1", base.ID, "/protagonist_arc"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewDiscussionSuggestion(ctx, "project-1", DiscussionSuggestionRequest{
		MessageID: reply.ID, SuggestionID: reply.Suggestions[0].ID, IdempotencyKey: "locked-suggest",
	}, revisionProfile()); !errors.Is(err, creationdomain.ErrFieldLocked) {
		t.Fatalf("locked suggestion error = %v", err)
	}
}

func TestDiscussionStaleAfterCandidateEditAndRecoverDoesNotReplay(t *testing.T) {
	ctx := context.Background()
	var generation atomic.Uint64
	generation.Store(1)
	kernel := newFakeKernel()
	kernel.block = make(chan struct{})
	kernel.started = make(chan struct{})
	service, store, _ := newTestService(t, kernel, generation.Load)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, IdempotencyKey: "discuss-recover",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "标题还成不成立？", MessageIdempotencyKey: "discuss-recover-1",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-kernel.started:
	default:
		if !waitFor(t, func() bool {
			select {
			case <-kernel.started:
				return true
			default:
				return false
			}
		}) {
			t.Fatal("discussion run did not start")
		}
	}
	generation.Add(1)
	close(kernel.block)
	if !waitFor(t, func() bool { return !service.coordinator.Live(turn.RunID) }) {
		t.Fatal("blocked discussion run did not drain")
	}
	if _, err := service.Recover(ctx, "project-1"); err != nil {
		t.Fatal(err)
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("recover did not materialize the public reply")
	}
	if kernel.createN != 1 {
		t.Fatalf("recover replayed payment: createN=%d", kernel.createN)
	}
	auth, err := store.AuthorizationByAction(ctx, preview.ActionID)
	if err != nil || auth.UsedCalls != 1 || auth.ReservedCalls != 0 {
		t.Fatalf("recovered budget = %+v err=%v", auth, err)
	}
	if _, err := service.SaveFields(ctx, "project-1", SaveFieldsRequest{CandidateID: base.ID, Changes: map[string]string{"/title": "闭城之冬"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SendDiscussionMessage(ctx, "project-1", DiscussionMessageRequest{
		AuthorizationID: auth.ID, CandidateID: base.ID, FieldPaths: []string{"/title"},
		Text: "新标题之后还能继续吗？", IdempotencyKey: "discuss-recover-2",
	}, discussionProfile()); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
		t.Fatalf("edited candidate error = %v", err)
	}
	stale, err := store.Authorization(ctx, auth.ID)
	if err != nil || stale.State != assistantdomain.AuthStale {
		t.Fatalf("stale auth = %+v err=%v", stale, err)
	}
}

func TestDiscussionRevokeStopsNewTurns(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, IdempotencyKey: "discuss-revoke",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "先讨论标题", MessageIdempotencyKey: "discuss-revoke-1",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if !waitForDiscussionReply(t, store) {
		t.Fatal("revoke setup did not finish")
	}
	if _, err := service.RevokeDiscussionAuthorization(ctx, "project-1", turn.Authorization.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SendDiscussionMessage(ctx, "project-1", DiscussionMessageRequest{
		AuthorizationID: turn.Authorization.ID, CandidateID: base.ID, FieldPaths: []string{"/title"},
		Text: "撤销后不应再发送", IdempotencyKey: "discuss-revoke-2",
	}, discussionProfile()); err == nil {
		t.Fatal("revoked authorization still accepted a new turn")
	}
	assistant, err := store.AssistantSession(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := store.ListAssistantMessages(ctx, assistant.ID)
	if err != nil || len(messages) == 0 {
		t.Fatalf("revoke deleted history: %v %#v", err, messages)
	}
}

func TestDiscussionUnknownProviderResultKeepsReservation(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	kernel.failExecute = true
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, Message: "标题是否准确？", IdempotencyKey: "unknown-provider",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ConfirmDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
		Message: "标题是否准确？", MessageIdempotencyKey: "unknown-provider-message",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool { return !service.coordinator.Live(turn.RunID) }) {
		t.Fatal("discussion run did not stop")
	}
	auth, err := store.Authorization(ctx, turn.Authorization.ID)
	if err != nil || auth.ReservedCalls != 1 || auth.UsedCalls != 0 {
		t.Fatalf("unknown provider result budget = %+v err=%v", auth, err)
	}
	assistant, err := store.AssistantSession(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	message, err := store.AssistantMessageByIdempotency(ctx, assistant.ID, "unknown-provider-message")
	if err != nil || message.Status != assistantdomain.MessageOutcomeUnknown {
		t.Fatalf("unknown provider message = %+v err=%v", message, err)
	}
}

func TestDiscussionRecoveryReleasesOrphanReservation(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	base := seedProposalCandidate(t, service, store)
	preview, err := service.PreviewDiscussionAuthorization(ctx, "project-1", DiscussionAuthorizationRequest{
		CandidateID: base.ID, FieldPaths: []string{"/title"}, Message: "标题是否准确？", IdempotencyKey: "orphan-reservation",
	}, discussionProfile())
	if err != nil {
		t.Fatal(err)
	}
	action, err := store.Action(ctx, preview.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	input, err := decodeDiscussionInput(action)
	if err != nil {
		t.Fatal(err)
	}
	now := testClock{}.Now()
	auth, err := assistantdomain.NewAuthorization(input.AuthorizationID, action.SessionID, "project-1", "branch-1", action.ID, base.ID, base.ContentSHA256, string(base.Stage), "", base.Version, []string{"/title"}, discussionProfileID(input.Profile), plannerConnection(input.Profile), input.Planner.Provider, input.Planner.Model, plannerEffort(input.Profile), 8, 16000, 4000, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	auth, _ = auth.Activate(now)
	auth, _ = auth.ReserveTurn(now)
	if err := store.SaveAuthorization(ctx, auth); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Recover(ctx, "project-1"); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Authorization(ctx, auth.ID)
	if err != nil || recovered.ReservedCalls != 0 || recovered.RemainingCalls() != 8 {
		t.Fatalf("recovered orphan reservation = %+v err=%v", recovered, err)
	}
}

func discussionProfile() agentdomain.ResolvedCreativeProfile {
	return generateRequest(creationdomain.StageProposal).Profile
}

func revisionProfile() agentdomain.ResolvedCreativeProfile {
	profile := discussionProfile()
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	return profile
}

func waitForDiscussionReply(t *testing.T, store *memoryStore) bool {
	t.Helper()
	return waitFor(t, func() bool {
		assistant, err := store.AssistantSession(context.Background(), "project-1", "branch-1")
		if err != nil {
			return false
		}
		messages, err := store.ListAssistantMessages(context.Background(), assistant.ID)
		if err != nil {
			return false
		}
		answered := 0
		running := 0
		for _, message := range messages {
			if message.Kind != assistantdomain.KindDiscussion {
				continue
			}
			if message.Role == assistantdomain.RoleAssistant && message.Status == assistantdomain.MessageAnswered {
				answered++
			}
			if message.Role == assistantdomain.RoleAuthor && (message.Status == assistantdomain.MessagePending || message.Status == assistantdomain.MessageRunning) {
				running++
			}
		}
		return answered > 0 && running == 0
	})
}
