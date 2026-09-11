package creation

import (
	"context"
	"errors"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestSaveCandidateDraftRejectsHistoricalBaseline(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	originalID := currentCandidateID(t, service, store, creationdomain.StageDirections)
	first, err := service.SaveCandidateDraft(context.Background(), session.ProjectID, originalID, altDirectionPayload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveCandidateDraft(
		context.Background(), session.ProjectID, originalID, directionPayload,
	); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
		t.Fatalf("historical draft error = %v", err)
	}
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(candidates, creationdomain.StageDirections)
	if !ok || current.ID != first.ID {
		t.Fatalf("historical edit changed current candidate: %+v", current)
	}
}

func TestChangingDirectionMarksProposalStale(t *testing.T) {
	service, store, kernel := newTestService(t, newFakeKernel(), nil)
	session, direction, proposal := seedTrackedProposal(t, service, store)

	if _, err := service.SaveCandidateDraft(context.Background(), session.ProjectID, direction.ID, altDirectionPayload); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(listed, creationdomain.StageProposal)
	if !ok || current.ID != proposal.ID || current.Status != creationdomain.StatusStale {
		t.Fatalf("proposal after direction edit = %#v ok=%v", current, ok)
	}
	if current.ImpactSeverity != creationdomain.ImpactMaybe {
		t.Fatalf("severity = %q", current.ImpactSeverity)
	}
	if _, err := service.AdvanceStage(context.Background(), session.ProjectID); !errors.Is(err, creationdomain.ErrImpactUnresolved) {
		t.Fatalf("advance stale error = %v", err)
	}
	if kernel.executeN != 0 {
		t.Fatalf("stale must not start a paid run: executeN=%d", kernel.executeN)
	}
}

func TestKeepExceptionAllowsAdvanceAfterStale(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session, direction, _ := seedTrackedProposal(t, service, store)
	if _, err := service.SaveCandidateDraft(context.Background(), session.ProjectID, direction.ID, altDirectionPayload); err != nil {
		t.Fatal(err)
	}

	result, err := service.ResolveImpact(context.Background(), ResolveImpactRequest{
		ProjectID: session.ProjectID, Choice: creationdomain.ChoiceKeepException,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Projection.CanAdvance || len(result.Projection.Items) == 0 || !result.Projection.Items[0].Excepted {
		t.Fatalf("projection = %#v", result.Projection)
	}
	currentID := currentCandidateID(t, service, store, creationdomain.StageProposal)
	if _, err := service.AcceptCandidate(context.Background(), session.ProjectID, currentID); err != nil {
		t.Fatal(err)
	}
	advanced, err := service.AdvanceStage(context.Background(), session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Stage != creationdomain.StagePeopleWorld {
		t.Fatalf("stage = %s", advanced.Stage)
	}
}

func TestAbandonUpstreamRestoresDirectionHash(t *testing.T) {
	service, store, kernel := newTestService(t, newFakeKernel(), nil)
	session, direction, proposal := seedTrackedProposal(t, service, store)
	originalHash := direction.ContentSHA256
	if _, err := service.SaveCandidateDraft(context.Background(), session.ProjectID, direction.ID, altDirectionPayload); err != nil {
		t.Fatal(err)
	}

	result, err := service.ResolveImpact(context.Background(), ResolveImpactRequest{
		ProjectID: session.ProjectID, Choice: creationdomain.ChoiceAbandonUpstream,
	})
	if err != nil {
		t.Fatal(err)
	}
	if kernel.executeN != 0 {
		t.Fatalf("abandon must not start a paid run: executeN=%d", kernel.executeN)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := currentCandidate(listed, creationdomain.StageDirections)
	if !ok || restored.ContentSHA256 != originalHash || restored.Status == creationdomain.StatusDiscarded {
		t.Fatalf("restored direction = %#v ok=%v", restored, ok)
	}
	currentProposal, ok := currentCandidate(listed, creationdomain.StageProposal)
	if !ok || currentProposal.ID != proposal.ID || currentProposal.Status == creationdomain.StatusStale {
		t.Fatalf("proposal after abandon = %#v ok=%v", currentProposal, ok)
	}
	if result.Projection.CanAdvance != true {
		t.Fatalf("projection = %#v", result.Projection)
	}
}

func seedTrackedProposal(t *testing.T, service *Service, store *memoryStore) (creationdomain.Session, creationdomain.Candidate, creationdomain.Candidate) {
	t.Helper()
	session := seedDirectionsSelection(t, service, store, "d1")
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	direction, ok := currentCandidate(listed, creationdomain.StageDirections)
	if !ok {
		t.Fatal("missing direction candidate")
	}
	proposal, err := creationdomain.NewCandidate(
		"prop-1", session.ID, creationdomain.StageProposal, creationdomain.KindProposal,
		1, "", "run-p", "art-p", proposalPayload, "story_blueprint.schema", 2, testClock{}.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	// 上游摘要必须用实际输入公式（意图+选中卡），而不是方向候选整包哈希。
	proposal = proposal.WithUpstream(liveUpstreamDigest(session, listed, creationdomain.StageProposal))
	if err := store.SaveCandidate(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	return session, direction, proposal
}

const altDirectionPayload = `{"directions":[{"id":"d1","hook":"守关人私放旧友","genre":"奇幻悬疑","protagonist":"守关人阿绫","core_conflict":"守城与救人","growth_cost":"失去军职","pacing":"中速","ending":"开城救人"},{"id":"d2","hook":"潮汐神谕错位","genre":"历史传奇","protagonist":"潮汐祭司","core_conflict":"神谕与民生","growth_cost":"信仰崩解","pacing":"慢热","ending":"改写神谕"},{"id":"d3","hook":"假身份潜入盐帮","genre":"江湖复仇","protagonist":"失踪少主","core_conflict":"复仇与认亲","growth_cost":"放弃姓氏","pacing":"快节奏","ending":"揭穿盐帮"}]}`
