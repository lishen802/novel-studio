package quality

import "testing"

func TestHarnessWorkflowVariantsKeepExperimentalComplexityDisabled(t *testing.T) {
	variants := BuiltinHarnessWorkflowVariants()
	if len(variants) != 3 || !variants[0].DefaultEnabled {
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
