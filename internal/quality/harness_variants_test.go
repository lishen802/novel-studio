package quality

import "testing"

func TestHarnessWorkflowVariantsKeepExperimentalComplexityDisabled(t *testing.T) {
	variants := BuiltinHarnessWorkflowVariants()
	if len(variants) != 5 || !variants[0].DefaultEnabled {
		t.Fatalf("variants = %+v", variants)
	}
	for _, variant := range variants[1:] {
		if variant.DefaultEnabled {
			t.Fatalf("experimental variant %s became default without evidence", variant.ID)
		}
	}
	if !variants[2].SplitIndependentReview || len(variants[2].ProfessionalAnalyses) != 0 {
		t.Fatalf("split review variant = %+v", variants[2])
	}
}

func TestSlotParallelVariantUsesEnhancedDirectorAsSingleVariableControl(t *testing.T) {
	variants := BuiltinHarnessWorkflowVariants()
	var director, slot HarnessWorkflowVariant
	for _, variant := range variants {
		if variant.ID == "enhanced-director" {
			director = variant
		}
		if variant.ID == "slot-parallel" {
			slot = variant
		}
	}
	if slot.ID == "" || !slot.SlotParallel || slot.DefaultEnabled || slot.EnhancedDirector != director.EnhancedDirector || slot.SplitIndependentReview != director.SplitIndependentReview || slot.VerifiedRevision != director.VerifiedRevision {
		t.Fatalf("slot variant is not isolated from enhanced director: %+v vs %+v", slot, director)
	}
}

// 修订复核变体必须与增强导演变体只差复核节点，否则无法把质量差异归因到该节点。
func TestVerifiedRevisionVariantIsolatesVerificationNode(t *testing.T) {
	variants := BuiltinHarnessWorkflowVariants()
	var director, verified HarnessWorkflowVariant
	for _, variant := range variants {
		switch variant.ID {
		case "enhanced-director":
			director = variant
		case "verified-revision":
			verified = variant
		}
	}
	if verified.ID == "" || !verified.VerifiedRevision || verified.DefaultEnabled {
		t.Fatalf("verified variant = %+v", verified)
	}
	if verified.WorkflowID != director.WorkflowID || verified.WorkflowVersion == director.WorkflowVersion {
		t.Fatalf("verified variant is not a single-variable comparison: %+v vs %+v", verified, director)
	}
	if verified.EnhancedDirector != director.EnhancedDirector || verified.SplitIndependentReview != director.SplitIndependentReview {
		t.Fatalf("verified variant changed more than the verification node: %+v", verified)
	}
}
