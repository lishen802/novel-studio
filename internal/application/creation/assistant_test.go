package creation

import (
	"context"
	"errors"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestExplainDoesNotCreateAction(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	candidate := seedProposalCandidate(t, service, store)
	explanation, err := service.Explain(context.Background(), "project-1", []string{candidate.ID})
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Text == "" {
		t.Fatal("expected read-only explanation")
	}
	count, err := store.PendingActionCount(context.Background(), "project-1")
	if err != nil || count != 0 || kernel.createN != 0 {
		t.Fatalf("pending=%d createN=%d err=%v", count, kernel.createN, err)
	}
}

func TestProposePaidActionDoesNotCallKernel(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	candidate := seedProposalCandidate(t, service, store)
	action, err := service.ProposeAction(context.Background(), proposeRevise(candidate.ID))
	if err != nil {
		t.Fatal(err)
	}
	if action.State != assistantdomain.StatePendingConfirmation || action.Risk != assistantdomain.RiskPaid {
		t.Fatalf("action = %#v", action)
	}
	if kernel.createN != 0 || kernel.executeN != 0 {
		t.Fatalf("paid propose must not call kernel: create=%d execute=%d", kernel.createN, kernel.executeN)
	}
	if _, err := service.ProposeAction(context.Background(), ProposeRequest{
		ProjectID: "project-1", Operation: assistantdomain.OpConfirmArchive,
		CandidateID: candidate.ID, IdempotencyKey: "bad-archive",
	}); !errors.Is(err, assistantdomain.ErrInvalidOperation) {
		t.Fatalf("archive via assistant error = %v", err)
	}
}

func TestConfirmRejectsArgumentHashChange(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	candidate := seedProposalCandidate(t, service, store)
	action, err := service.ProposeAction(context.Background(), proposeRevise(candidate.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmAction(context.Background(), ActionConfirmRequest{
		ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: "changed", IdempotencyKey: action.IdempotencyKey,
	}); !errors.Is(err, assistantdomain.ErrArgumentMismatch) {
		t.Fatalf("hash change error = %v", err)
	}
	if kernel.executeN != 0 {
		t.Fatalf("hash mismatch must not execute: %d", kernel.executeN)
	}
}

func TestConfirmReviseFieldsUsesAuthorDirective(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	candidate := seedProposalCandidate(t, service, store)
	action, err := service.ProposeAction(context.Background(), proposeRevise(candidate.ID))
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ConfirmAction(context.Background(), ActionConfirmRequest{
		ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action.State != assistantdomain.StateSucceeded || result.Candidate.Version != 2 {
		t.Fatalf("result = %#v", result)
	}
	if kernel.lastCreate.RevisionDirective == nil || kernel.lastCreate.RevisionDirective.Mode != agentdomain.RevisionAuthorDirected {
		t.Fatalf("directive = %#v", kernel.lastCreate.RevisionDirective)
	}
	if kernel.lastCreate.WorkflowID != agentassets.StoryAuthorReviseWorkflowID {
		t.Fatalf("workflow = %q", kernel.lastCreate.WorkflowID)
	}
}

func TestOutcomeUnknownDoesNotReplay(t *testing.T) {
	kernel := newFakeKernel()
	kernel.failExecute = true
	service, store, _ := newTestService(t, kernel, nil)
	candidate := seedProposalCandidate(t, service, store)
	action, err := service.ProposeAction(context.Background(), proposeRevise(candidate.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmAction(context.Background(), ActionConfirmRequest{
		ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
	}); err == nil {
		t.Fatal("expected execute failure")
	}
	loaded, err := store.Action(context.Background(), action.ID)
	if err != nil || loaded.State != assistantdomain.StateOutcomeUnknown {
		t.Fatalf("loaded = %#v, err=%v", loaded, err)
	}
	if _, err := service.ConfirmAction(context.Background(), ActionConfirmRequest{
		ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey,
	}); !errors.Is(err, assistantdomain.ErrReplayForbidden) {
		t.Fatalf("replay error = %v", err)
	}
	if kernel.createN != 1 || kernel.executeN != 1 {
		t.Fatalf("replayed kernel calls create=%d execute=%d", kernel.createN, kernel.executeN)
	}
}

func seedProposalCandidate(t *testing.T, service *Service, store *memoryStore) creationdomain.Candidate {
	t.Helper()
	session := seedProposalSession(t, service, store)
	candidate, err := creationdomain.NewCandidate(
		"proposal-1", session.ID, creationdomain.StageProposal, creationdomain.KindProposal,
		1, "", "run-prop", "art-prop", proposalPayload, "story_blueprint.schema", 2, testClock{}.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	return candidate
}

func proposeRevise(candidateID string) ProposeRequest {
	request := generateRequest(creationdomain.StageProposal)
	return ProposeRequest{
		ProjectID: request.ProjectID, BranchID: request.BranchID, ChapterID: request.ChapterID,
		CanonicalHead: request.CanonicalHead, Profile: request.Profile,
		Operation: assistantdomain.OpReviseFields, CandidateID: candidateID,
		Paths: []string{"/title"}, Instruction: "把标题改得更冷", IdempotencyKey: "revise-1",
	}
}
