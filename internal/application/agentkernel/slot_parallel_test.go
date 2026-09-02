package agentkernel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestSlotParallelWorkflowKeepsWritersIsolatedAndPersistsMechanicalJoin(t *testing.T) {
	replies := slotParallelReplies()
	service, store, provider := newRunnerHarness(t, replies)
	request := qualityRequest(agentassets.SlotParallelWorkflowID, agentassets.SlotParallelWorkflowVersion, 40000)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 9 {
		t.Fatalf("completed=%+v requests=%d", completed, len(provider.requests))
	}
	if !strings.Contains(provider.requests[0].Input, "【冻结来源 anchor-1@1】") {
		t.Fatalf("director received anonymous context: %q", provider.requests[0].Input)
	}
	for index := 2; index <= 6; index++ {
		if strings.Contains(provider.requests[index].Input, "她推门奔出") || strings.Contains(provider.requests[index].Input, "雨水沿屋檐") {
			t.Fatalf("slot writer %d saw another writer output", index)
		}
	}
	if countArtifacts(store.artifacts, agentdomain.ArtifactSlotJoin) != 1 || !strings.Contains(provider.requests[7].Input, "她推门奔出") {
		t.Fatalf("mechanical join missing: artifacts=%+v", store.artifacts)
	}
	for _, step := range completed.Steps {
		if step.Capability == agentdomain.CapabilitySkeletonWriter || step.Capability == agentdomain.CapabilitySlotWriter || step.Capability == agentdomain.CapabilityIntegrationWriter {
			if step.PublicStageID != agentdomain.PublicStageProseWriting || step.Role != agentdomain.RoleWriter {
				t.Fatalf("slot writing public contract leaked: %+v", step)
			}
		}
	}
}

func TestSlotParallelRevisionReceivesIntegratedDraft(t *testing.T) {
	replies := slotParallelReplies()
	replies[8] = providerReply{text: `{"quality_status":"needs_review","summary":"一处动作需收紧","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"奔出缺少阻力","suggestion":"补一句门轴阻力","constraint_ref":"style:action"}]}`}
	replies = append(replies, providerReply{text: `{"title":"雨夜钟楼","body":"“现在就走。”她低声催促。\n\n她推门奔出，鞋底擦过积水。\n\n恐惧顶住喉咙，她没有回头。\n\n钟楼三响，冷雨敲打破窗。\n\n转过狭窄走廊，楼梯口已经在前。","resolutions":[{"finding_id":"f1","status":"retained","note":"门轴阻力会破坏节奏"}],"changes":[]}`})
	service, _, provider := newRunnerHarness(t, replies)
	request := qualityRequest(agentassets.SlotParallelWorkflowID, agentassets.SlotParallelWorkflowVersion, 40000)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatalf("revision failed: %v status=%s message=%s", err, completed.Status, completed.ErrorMessage)
	}
	if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 10 {
		t.Fatalf("completed=%+v requests=%d", completed, len(provider.requests))
	}
	revisionInput := provider.requests[9].Input
	if !strings.Contains(revisionInput, "她推门奔出") || !strings.Contains(revisionInput, `"id":"f1"`) {
		t.Fatalf("revision did not receive integrate draft and review: %q", revisionInput)
	}
}

func TestRenderPromptBindsDraftFromIntegrateWhenReviewAlsoPresent(t *testing.T) {
	node := FrozenNode{
		ID: "revision", Capability: agentdomain.CapabilityBoundedRevision,
		InputSelectors: []string{"$task", "$context", "node:integrate", "node:review"},
	}
	prompt := ports.PromptAsset{
		Binding:   agentdomain.VersionBinding{ID: "revise_draft.prompt", Version: 1},
		Template:  "正文：{{.Draft}}\n审阅：{{.Review}}",
		Variables: []string{"Draft", "Review"},
	}
	latest := map[string]agentdomain.RunStep{
		"integrate": {NodeID: "integrate", Status: agentdomain.StepCompleted, PrimaryArtifactID: "draft-1", Capability: agentdomain.CapabilityIntegrationWriter},
		"review":    {NodeID: "review", Status: agentdomain.StepCompleted, PrimaryArtifactID: "review-1", Capability: agentdomain.CapabilityContinuityReview},
	}
	artifacts := []agentdomain.Artifact{
		{ID: "draft-1", Kind: agentdomain.ArtifactDraft, Content: "她推门奔出。"},
		{ID: "review-1", Kind: agentdomain.ArtifactReview, Content: `{"summary":"需修订"}`},
	}
	rendered, err := renderPrompt(prompt, persistedInput{AuthorInstruction: "写一章"}, node, latest, artifacts, "冻结上下文")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "她推门奔出。") || !strings.Contains(rendered, "需修订") {
		t.Fatalf("rendered=%q", rendered)
	}
}

func TestSlotParallelIntegrationFailureRetainsMechanicalJoin(t *testing.T) {
	replies := slotParallelReplies()[:7]
	replies = append(replies, providerReply{err: errors.New("integration unavailable")})
	service, store, _ := newRunnerHarness(t, replies)
	request := qualityRequest(agentassets.SlotParallelWorkflowID, agentassets.SlotParallelWorkflowVersion, 40000)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err == nil || failed.Status != agentdomain.RunFailed || countArtifacts(store.artifacts, agentdomain.ArtifactSlotJoin) != 1 {
		t.Fatalf("failed=%+v err=%v artifacts=%+v", failed, err, store.artifacts)
	}
}

func TestSkeletonRewritesSourceOutsideFrozenManifest(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterSkeleton, []byte(slotParallelReplies()[1].text))
	if err != nil {
		t.Fatal(err)
	}
	manifest := testSourceManifest("anchor-1", "1")
	if err := validateSkeletonOutput(decoded, manifest); err != nil {
		t.Fatalf("frozen source rejected: %v", err)
	}
	skeleton := decoded.(*agentdomain.ChapterSkeletonOutput)
	skeleton.Beats[0].Slots[0].SourceRefs = []string{"outside@1"}
	if err := validateSkeletonOutput(skeleton, manifest); err != nil {
		t.Fatalf("unknown skeleton refs should be rewritten, got %v", err)
	}
	if len(skeleton.Beats[0].Slots[0].SourceRefs) != 1 || skeleton.Beats[0].Slots[0].SourceRefs[0] != "anchor-1@1" {
		t.Fatalf("rewritten refs = %v", skeleton.Beats[0].Slots[0].SourceRefs)
	}
}

func TestChapterDirectorAcceptsEveryDisplayedFrozenSourceReference(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterPlan, []byte(slotParallelReplies()[0].text))
	if err != nil {
		t.Fatal(err)
	}
	plan := decoded.(*agentdomain.ChapterPlanOutput)
	for _, reference := range []string{"author-instruction@1", "anchor-1@1", "【冻结来源 anchor-1@1】"} {
		plan.CanonicalSourceRefs = []string{reference}
		manifest := testSourceManifest("author-instruction", "1", "anchor-1", "1")
		if err := validateChapterDirector(plan, manifest); err != nil {
			t.Fatalf("displayed reference %q rejected: %v", reference, err)
		}
		if plan.CanonicalSourceRefs[0] != "author-instruction@1" && plan.CanonicalSourceRefs[0] != "anchor-1@1" {
			t.Fatalf("canonical refs not stamped: %v", plan.CanonicalSourceRefs)
		}
	}
}

func TestChapterDirectorStampsManifestWhenModelRefsAreUnknown(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterPlan, []byte(slotParallelReplies()[0].text))
	if err != nil {
		t.Fatal(err)
	}
	plan := decoded.(*agentdomain.ChapterPlanOutput)
	plan.CanonicalSourceRefs = []string{"story-spec", "潮汐钟"}
	manifest := testSourceManifest("author-instruction", "1", "anchor-1", "1")
	if err := validateChapterDirector(plan, manifest); err != nil {
		t.Fatalf("unknown refs should be rewritten, got %v", err)
	}
	if len(plan.CanonicalSourceRefs) != 2 || plan.CanonicalSourceRefs[0] != "author-instruction@1" || plan.CanonicalSourceRefs[1] != "anchor-1@1" {
		t.Fatalf("stamped refs = %v", plan.CanonicalSourceRefs)
	}
}

func TestChapterDirectorRejectsExactForbiddenReleaseOnly(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterPlan, []byte(slotParallelReplies()[0].text))
	if err != nil {
		t.Fatal(err)
	}
	plan := decoded.(*agentdomain.ChapterPlanOutput)
	plan.ForbiddenReveals = []string{"闹鬼"}
	plan.Scenes[0].InformationRelease = []string{"排除闹鬼，只呈现钟慢七秒"}
	manifest := testSourceManifest("anchor-1", "1")
	if err := validateChapterDirector(plan, manifest); err != nil {
		t.Fatalf("mentioning a forbidden word in a longer release failed: %v", err)
	}
	plan.Scenes[0].InformationRelease = []string{"闹鬼"}
	if err := validateChapterDirector(plan, manifest); err == nil {
		t.Fatal("exact forbidden release accepted")
	}
}

func TestSkeletonAcceptsDisplayedUpstreamArtifactReference(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterSkeleton, []byte(slotParallelReplies()[1].text))
	if err != nil {
		t.Fatal(err)
	}
	skeleton := decoded.(*agentdomain.ChapterSkeletonOutput)
	skeleton.Beats[0].Slots[0].SourceRefs = []string{"director-artifact@attempt-1"}
	if err := validateSkeletonOutput(skeleton, testSourceManifest("director-artifact", "attempt-1", "anchor-1", "1")); err != nil {
		t.Fatalf("displayed upstream artifact rejected: %v", err)
	}
}

func testSourceManifest(values ...string) agentdomain.ContextManifest {
	manifest := agentdomain.ContextManifest{}
	for index := 0; index+1 < len(values); index += 2 {
		manifest.Items = append(manifest.Items, agentdomain.ContextManifestItem{
			SourceID: values[index], SourceVersion: values[index+1], Included: true,
		})
	}
	return manifest
}

func slotParallelReplies() []providerReply {
	return []providerReply{
		{text: `{"chapter_goal":"逃离钟楼","opening_hook":"钟楼三响","ending_hook":"门外脚步逼近","scenes":[{"id":"scene-1","goal":"逃离","conflict":"追兵逼近","outcome":"冲出房间","pov":"林岚","desire":"活下去","obstacle":"门被锁住","information_release":["追兵已经进楼"],"state_change":"从犹豫到行动"}],"required_anchors":["钟楼三响"],"forbidden_reveals":["幕后主使"],"improvisation_bounds":["不得离开钟楼区域"],"canonical_source_refs":["anchor-1@1"],"creative_suggestions":["用雨声增强压迫感"]}`},
		{text: `{"chapter_goal":"逃离钟楼","beats":[{"beat_id":"beat-1","summary":"林岚冲出房间","pov":"林岚","slots":[{"slot_id":"d1","slot_type":"dialogue","intent":"催促同伴离开","constraints":[],"target_runes":30,"source_refs":["anchor-1@1"]},{"slot_id":"a1","slot_type":"action","intent":"推门奔出","constraints":[],"target_runes":30,"source_refs":["anchor-1@1"]},{"slot_id":"i1","slot_type":"interiority","intent":"压住恐惧","constraints":[],"target_runes":30,"source_refs":["anchor-1@1"]},{"slot_id":"e1","slot_type":"description","intent":"交代钟楼雨夜","constraints":[],"target_runes":30,"source_refs":["anchor-1@1"]},{"slot_id":"t1","slot_type":"transition","intent":"抵达楼梯口","constraints":[],"target_runes":30,"source_refs":["anchor-1@1"]}]}]}`},
		{text: `{"filled_slots":[{"slot_id":"d1","content":"“现在就走。”她低声催促。","anchor_hits":[]}]}`},
		{text: `{"filled_slots":[{"slot_id":"a1","content":"她推门奔出，鞋底擦过积水。","anchor_hits":[]}]}`},
		{text: `{"filled_slots":[{"slot_id":"i1","content":"恐惧顶住喉咙，她没有回头。","anchor_hits":[]}]}`},
		{text: `{"filled_slots":[{"slot_id":"e1","content":"钟楼三响，冷雨敲打破窗。","anchor_hits":["钟楼三响"]}]}`},
		{text: `{"filled_slots":[{"slot_id":"t1","content":"转过狭窄走廊，楼梯口已经在前。","anchor_hits":[]}]}`},
		{text: `{"title":"雨夜钟楼","body":"“现在就走。”她低声催促。\n\n她推门奔出，鞋底擦过积水。\n\n恐惧顶住喉咙，她没有回头。\n\n钟楼三响，冷雨敲打破窗。\n\n转过狭窄走廊，楼梯口已经在前。","completion_note":"仅统一拼接处视角与过渡"}`},
		{text: `{"quality_status":"passed","summary":"未发现问题","findings":[]}`},
	}
}
