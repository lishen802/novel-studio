package creation

import (
	"errors"
	"testing"
)

func TestCreationStagesFollowProductRail(t *testing.T) {
	want := []Stage{StageIntent, StageDirections, StageProposal, StagePeopleWorld, StageRoadmap, StageArchiveConfirm}
	got := Rail()
	if len(got) != len(want) {
		t.Fatalf("rail length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rail[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorkbenchProjectionMapsFineStages(t *testing.T) {
	cases := map[Stage]string{
		StageIntent:         "idea",
		StageDirections:     "idea",
		StageProposal:       "proposal",
		StagePeopleWorld:    "proposal",
		StageRoadmap:        "roadmap",
		StageArchiveConfirm: "roadmap",
	}
	for stage, want := range cases {
		if got := stage.WorkbenchStage(); got != want {
			t.Fatalf("%s: workbench = %q, want %q", stage, got, want)
		}
	}
}

func TestAdvanceAllowsOnlyImmediateNextStage(t *testing.T) {
	if err := CanAdvance(StageIntent, StageDirections); err != nil {
		t.Fatal(err)
	}
	if err := CanAdvance(StageDirections, StageProposal); err != nil {
		t.Fatal(err)
	}
	if err := CanAdvance(StageIntent, StageProposal); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("skip ahead error = %v", err)
	}
	if err := CanAdvance(StageArchiveConfirm, StageIntent); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("rewind by advance error = %v", err)
	}
}

func TestLookBackAllowsReachedStagesOnly(t *testing.T) {
	if err := CanLookBack(StageProposal, StageIntent); err != nil {
		t.Fatal(err)
	}
	if err := CanLookBack(StageIntent, StageProposal); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("look ahead error = %v", err)
	}
}

func TestLegacyIdeaStageStartsAtIntent(t *testing.T) {
	if got := NormalizeLegacyWorkbenchStage("idea"); got != StageIntent {
		t.Fatalf("legacy idea = %q, want %q", got, StageIntent)
	}
	if got := NormalizeLegacyWorkbenchStage("proposal"); got != StageProposal {
		t.Fatalf("legacy proposal = %q, want %q", got, StageProposal)
	}
}
