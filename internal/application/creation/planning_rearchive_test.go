package creation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// 再次入档只追加未来版本：旧章、类型原行与作者正文保持原样，重复确认无二次写入。
func TestPlanningRearchiveKeepsHistoryAndReplays(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, c11RoadmapPayload, 3)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: first.ConfirmationDigest, IdempotencyKey: "first"}); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"已写旧章", "后续空章"} {
		chapter, e := projectdomain.NewChapter([]string{"old", "future"}[i], project.ID, title, i+1, testClock{}.Now())
		if e != nil {
			t.Fatal(e)
		}
		if e = raw.CreateChapter(ctx, chapter); e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			next, e := chapter.Commit("原有正文不能修改。", chapter.Revision, testClock{}.Now())
			if e != nil {
				t.Fatal(e)
			}
			if e = raw.CommitChapter(ctx, next, chapter); e != nil {
				t.Fatal(e)
			}
		}
	}
	proposal, e := bridge.Candidate(ctx, "proposal-1")
	if e != nil {
		t.Fatal(e)
	}
	changedProposal, e := service.SaveCandidateDraft(ctx, project.ID, proposal.ID, strings.Replace(proposalPayload, "奇幻抉择", "奇幻群像抉择", 1))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.AcceptAllReview(ctx, project.ID, changedProposal.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = service.AcceptCandidate(ctx, project.ID, changedProposal.ID); e != nil {
		t.Fatal(e)
	}
	road, e := bridge.Candidate(ctx, "roadmap-1")
	if e != nil {
		t.Fatal(e)
	}
	changed, e := service.SaveCandidateDraft(ctx, project.ID, road.ID, strings.Replace(c11RoadmapPayload, "守关人必须在亲情与职责间选择", "守关人必须在亲情、职责与同伴间选择", 1))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.AcceptAllReview(ctx, project.ID, changed.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = service.AcceptCandidate(ctx, project.ID, changed.ID); e != nil {
		t.Fatal(e)
	}
	request := asset.PlanningRearchiveRequest{ProjectID: project.ID, BranchID: "branch-1", ChapterID: "future", Reason: "让后续选择更明确"}
	preview, e := service.PreviewPlanningRearchive(ctx, request)
	if e != nil || len(preview.Changes) != 2 {
		t.Fatalf("preview: %+v %v", preview, e)
	}
	assetsBefore, e := raw.PlanningArchiveAssets(ctx, project.ID)
	if e != nil {
		t.Fatal(e)
	}
	changedProposal, e = bridge.Candidate(ctx, changedProposal.ID)
	if e != nil {
		t.Fatal(e)
	}
	changed, e = bridge.Candidate(ctx, changed.ID)
	if e != nil {
		t.Fatal(e)
	}
	proposalGuard, e := checkedRearchiveCandidateReview(ctx, service, changedProposal)
	if e != nil {
		t.Fatal(e)
	}
	people, e := bridge.Candidate(ctx, "people-1")
	if e != nil {
		t.Fatal(e)
	}
	peopleReview, e := bridge.Review(ctx, people.ID)
	if e != nil {
		t.Fatal(e)
	}
	peopleGuard := asset.PlanningRearchiveGuard{CandidateID: people.ID, ContentSHA256: people.ContentSHA256, ReviewVersion: peopleReview.ReviewVersion}
	roadGuard, e := checkedRearchiveCandidateReview(ctx, service, changed)
	if e != nil {
		t.Fatal(e)
	}
	invalid := preview
	invalid.Changes = append([]asset.PlanningRearchiveChange(nil), preview.Changes...)
	invalid.Changes[1].Digest = "invalid"
	if _, e = raw.ApplyPlanningRearchive(ctx, asset.PlanningRearchiveApply{Preview: invalid, Assets: assetsBefore, Guards: []asset.PlanningRearchiveGuard{proposalGuard, peopleGuard, roadGuard}, Key: "invalid-preview"}, testClock{}.Now()); !errors.Is(e, asset.ErrChangeConflict) {
		t.Fatalf("partial preview must fail: %v", e)
	}
	assetsAfter, e := raw.PlanningArchiveAssets(ctx, project.ID)
	if e != nil || !reflect.DeepEqual(assetsBefore, assetsAfter) {
		t.Fatalf("failed group changed formal revisions: %v", e)
	}
	command := asset.PlanningRearchiveCommand{Request: request, Digest: preview.Digest, IdempotencyKey: "planning-second"}
	for range 2 {
		if _, e = service.ConfirmPlanningRearchive(ctx, command); e != nil {
			t.Fatal(e)
		}
	}
	old, e := raw.EffectiveConstraints(ctx, 1)
	if e != nil {
		t.Fatal(e)
	}
	future, e := raw.EffectiveConstraints(ctx, 2)
	if e != nil {
		t.Fatal(e)
	}
	find := func(items []asset.EffectiveConstraint, kind string) string {
		for _, item := range items {
			for _, change := range preview.Changes {
				if item.AssetID == change.AssetID && change.Kind == kind {
					return item.Text
				}
			}
		}
		return ""
	}
	if strings.Contains(find(old, "outline_node"), "同伴间选择") || !strings.Contains(find(future, "outline_node"), "同伴间选择") || strings.Contains(find(old, "proposal"), "奇幻群像抉择") || !strings.Contains(find(future, "proposal"), "奇幻群像抉择") {
		t.Fatalf("wrong chapter versions: old=%v future=%v", old, future)
	}
	chapter, e := raw.Chapter(ctx, "old")
	if e != nil || chapter.Body != "原有正文不能修改。" {
		t.Fatalf("changed manuscript: %+v %v", chapter, e)
	}
	if _, e = service.ConfirmPlanningRearchive(ctx, asset.PlanningRearchiveCommand{Request: request, Digest: preview.Digest, IdempotencyKey: "different"}); !errors.Is(e, asset.ErrChangeUnsupported) && !errors.Is(e, asset.ErrChangeConflict) {
		t.Fatalf("duplicate should not append: %v", e)
	}
}
