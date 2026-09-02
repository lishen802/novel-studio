package agentkernel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
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

func TestSkeletonRejectsSourceOutsideFrozenManifest(t *testing.T) {
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
	if err := validateSkeletonOutput(skeleton, manifest); err == nil {
		t.Fatal("source outside frozen manifest accepted")
	}
}

func TestChapterDirectorAcceptsEveryDisplayedFrozenSourceReference(t *testing.T) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterPlan, []byte(slotParallelReplies()[0].text))
	if err != nil {
		t.Fatal(err)
	}
	plan := decoded.(*agentdomain.ChapterPlanOutput)
	for _, reference := range []string{"author-instruction@1", "anchor-1@1"} {
		plan.CanonicalSourceRefs = []string{reference}
		manifest := testSourceManifest("author-instruction", "1", "anchor-1", "1")
		if err := validateChapterDirector(plan, manifest); err != nil {
			t.Fatalf("displayed reference %q rejected: %v", reference, err)
		}
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
