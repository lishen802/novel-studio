package creation

import (
	"context"
	"errors"
	"github.com/lishen802/novel-studio/internal/agentassets"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"testing"
)

func TestRevisionScopeAcrossFourCandidateKinds(t *testing.T) {
	for _, tc := range []struct {
		stage         creationdomain.Stage
		payload, path string
	}{
		{creationdomain.StageDirections, directionPayload, "/directions/@d1/hook"},
		{creationdomain.StageProposal, proposalPayload, "/title"},
		{creationdomain.StagePeopleWorld, peopleWorldPayload, "/characters/@char-1/secret"},
		{creationdomain.StageRoadmap, roadmapPayload, "/book/title"},
	} {
		t.Run(string(tc.stage), func(t *testing.T) {
			base := creationdomain.Candidate{PayloadJSON: tc.payload}
			next, err := base.EditTextFields(map[string]string{tc.path: "作者指定的新文本"})
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := base.ReviewTextRevision(next, []string{tc.path})
			if err != nil || len(diffs) != 1 {
				t.Fatalf("diff=%v err=%v", diffs, err)
			}
		})
	}
}

func TestAuthorRevisionRejectsScopeBeforeCandidateMutation(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	kernel.payloads[agentassets.StoryAuthorReviseWorkflowID] = proposalRevisedPayload
	action, err := service.ProposeAction(ctx, proposeRevise(base.ID))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmAction(ctx, ActionConfirmRequest{ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey})
	if !errors.Is(err, creationdomain.ErrRevisionScope) {
		t.Fatalf("越界结果未拒绝: %v", err)
	}
	current, err := store.Candidate(ctx, base.ID)
	if err != nil || current.Status != base.Status || current.PayloadJSON != base.PayloadJSON {
		t.Fatalf("基线被改变: %v", err)
	}
	saved, err := store.Action(ctx, action.ID)
	if err != nil || saved.State != assistantdomain.StateFailed {
		t.Fatalf("应记录确定失败: %v %v", saved.State, err)
	}
	calls := kernel.executeN
	_, err = service.ConfirmAction(ctx, ActionConfirmRequest{ProjectID: "project-1", ActionID: action.ID, ArgumentsHash: action.ArgumentsHash, IdempotencyKey: action.IdempotencyKey})
	if err == nil || calls != kernel.executeN {
		t.Fatal("失败动作被重复付费执行")
	}
}
