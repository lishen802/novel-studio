package agentkernel

import (
	"errors"
	"testing"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestAdaptLegacyRunMapsRolesToCapabilities(t *testing.T) {
	run, err := agentdomain.NewChapterDraftRun(
		"run", "project", "chapter", 1, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "plan", Role: agentdomain.RolePlanner, ProfileID: "p1"},
			{ID: "write", Role: agentdomain.RoleWriter, ProfileID: "p2"},
			{ID: "review", Role: agentdomain.RoleContinuityReviewer, ProfileID: "p3"},
		}, time.Unix(1, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := AdaptLegacyRun(run)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 || nodes[1].Capability != agentdomain.CapabilityProseWriter || nodes[2].StepIndex != 2 {
		t.Fatalf("nodes = %#v", nodes)
	}
}

func TestAdaptLegacyRunRejectsKernelRun(t *testing.T) {
	_, err := AdaptLegacyRun(agentdomain.Run{WorkflowBinding: agentdomain.VersionBinding{ID: "workflow"}})
	if !errors.Is(err, ErrInvalidLegacyRun) {
		t.Fatalf("error = %v", err)
	}
}
