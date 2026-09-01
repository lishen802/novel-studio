package quality

import (
	"strings"
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestSkeletonLintDetectsOverreachAndLengthAllocation(t *testing.T) {
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	skeleton := qualitySkeleton()
	skeleton.Beats[0].Slots[0].Intent = strings.Repeat("长", 121)
	report, err := NewEngine(registry).EvaluateSkeleton(2, "skeleton", skeleton, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 2 || report.Findings[0].Severity != agentdomain.SeverityMajor || report.Findings[1].Severity != agentdomain.SeverityMajor {
		t.Fatalf("findings = %+v", report.Findings)
	}
}

func TestSlotIntegrationDetectsDriftAndCrossSlotRepetition(t *testing.T) {
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	skeleton := qualitySkeleton()
	groups := qualityFills()
	join, err := agentdomain.MechanicalJoin(skeleton, groups...)
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewEngine(registry).EvaluateSlotIntegration(2, "integrate", skeleton, groups, join.Body, strings.Repeat("完全改写", 50))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 2 || report.Findings[0].Severity != agentdomain.SeverityBlocker || report.Findings[1].ConstraintRef != "rule:slot.cross_repetition" {
		t.Fatalf("findings = %+v", report.Findings)
	}
}

func TestSlotIntegrationNormalSeamDoesNotTriggerDrift(t *testing.T) {
	registry, _ := BuiltinRegistry()
	skeleton := qualitySkeleton()
	groups := qualityFills()
	join, _ := agentdomain.MechanicalJoin(skeleton, groups...)
	report, err := NewEngine(registry).EvaluateSlotIntegration(2, "integrate", skeleton, groups, join.Body, join.Body+" 她终于松了口气。")
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.ConstraintRef == "rule:slot.integration_drift" {
			t.Fatalf("normal seam triggered drift: %+v", finding)
		}
	}
}

func TestSlotIntegrationReportsMissingCoverageAsBlocker(t *testing.T) {
	registry, _ := BuiltinRegistry()
	skeleton := qualitySkeleton()
	groups := qualityFills()[:4]
	report, err := NewEngine(registry).EvaluateSlotIntegration(2, "integrate", skeleton, groups, "incomplete", "incomplete")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].ConstraintRef != "rule:slot.coverage" || report.Findings[0].Severity != agentdomain.SeverityBlocker {
		t.Fatalf("coverage findings = %+v", report.Findings)
	}
}

func qualitySkeleton() agentdomain.ChapterSkeletonOutput {
	types := []agentdomain.SlotType{agentdomain.SlotDialogue, agentdomain.SlotAction, agentdomain.SlotInteriority, agentdomain.SlotDescription, agentdomain.SlotTransition}
	slots := make([]agentdomain.SkeletonSlot, 0, len(types))
	for index, kind := range types {
		slots = append(slots, agentdomain.SkeletonSlot{ID: string(rune('a' + index)), Type: kind, Intent: "推进", TargetRunes: 100, SourceRefs: []string{"anchor@1"}})
	}
	return agentdomain.ChapterSkeletonOutput{ChapterGoal: "逃离", Beats: []agentdomain.SkeletonBeat{{ID: "b1", Summary: "逃离", POV: "林岚", Slots: slots}}}
}

func qualityFills() []agentdomain.FillSlotOutput {
	return []agentdomain.FillSlotOutput{
		{FilledSlots: []agentdomain.FilledSlot{{ID: "a", Content: "雾港钟楼下潮声反复涌来，她说现在就走。"}}},
		{FilledSlots: []agentdomain.FilledSlot{{ID: "b", Content: "雾港钟楼下潮声反复涌来，她推门奔出。"}}},
		{FilledSlots: []agentdomain.FilledSlot{{ID: "c", Content: "她不敢回头。"}}},
		{FilledSlots: []agentdomain.FilledSlot{{ID: "d", Content: "雨水沿屋檐坠落。"}}},
		{FilledSlots: []agentdomain.FilledSlot{{ID: "e", Content: "片刻后街口已近。"}}},
	}
}
