package revision

import (
	"errors"
	"testing"
	"time"
)

func TestNewMainBranchEstablishesProjectBaseline(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)

	branch, err := NewMainBranch("main-id", "project-id", "baseline", now)
	if err != nil {
		t.Fatalf("NewMainBranch() error = %v", err)
	}
	if branch.Name != "主分支" || branch.ParentID != "" || branch.HeadDigest != "baseline" {
		t.Fatalf("NewMainBranch() = %#v", branch)
	}
	if branch.Generation != 1 || branch.CreatedAt != now || branch.UpdatedAt != now {
		t.Fatalf("NewMainBranch() metadata = %#v", branch)
	}
}

func TestForkBranchTracksUpstreamWithoutChangingProjectIdentity(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	parent, err := NewMainBranch("main-id", "project-id", "baseline", now)
	if err != nil {
		t.Fatal(err)
	}

	child, err := ForkBranch("branch-id", "黑化线", "主角接受魔契", parent, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ForkBranch() error = %v", err)
	}
	if child.ProjectID != parent.ProjectID || child.ParentID != parent.ID {
		t.Fatalf("ForkBranch() lineage = %#v", child)
	}
	if child.HeadDigest != parent.HeadDigest || child.UpstreamDigest != parent.HeadDigest {
		t.Fatalf("ForkBranch() digests = %#v", child)
	}
	if child.Generation != 1 {
		t.Fatalf("ForkBranch() generation = %d", child.Generation)
	}
}

func TestCheckpointOnlyAdvancesGenerationForChangedContent(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	branch, _ := NewMainBranch("main-id", "project-id", "baseline", now)

	unchanged := branch.Checkpoint("baseline", now.Add(time.Hour))
	if unchanged.Generation != branch.Generation || unchanged.UpdatedAt != branch.UpdatedAt {
		t.Fatalf("unchanged checkpoint = %#v", unchanged)
	}

	changed := branch.Checkpoint("next", now.Add(2*time.Hour))
	if changed.Generation != 2 || changed.HeadDigest != "next" {
		t.Fatalf("changed checkpoint = %#v", changed)
	}
}

func TestPlanFastForwardMergeRequiresUnchangedParent(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	parent, _ := NewMainBranch("main-id", "project-id", "baseline", now)
	child, _ := ForkBranch("branch-id", "黑化线", "", parent, now)
	child = child.Checkpoint("child-change", now.Add(time.Hour))

	plan, err := PlanFastForwardMerge(child, parent)
	if err != nil {
		t.Fatalf("PlanFastForwardMerge() error = %v", err)
	}
	if plan.SourceID != child.ID || plan.TargetID != parent.ID ||
		plan.SourceDigest != child.HeadDigest || plan.TargetDigest != parent.HeadDigest {
		t.Fatalf("PlanFastForwardMerge() = %#v", plan)
	}

	divergedParent := parent.Checkpoint("parent-change", now.Add(2*time.Hour))
	if _, err := PlanFastForwardMerge(child, divergedParent); !errors.Is(err, ErrBranchDiverged) {
		t.Fatalf("PlanFastForwardMerge() error = %v, want ErrBranchDiverged", err)
	}
}

func TestCompleteFastForwardMergeSupportsLaterFastForwards(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	parent, _ := NewMainBranch("main-id", "project-id", "baseline", now)
	child, _ := ForkBranch("branch-id", "黑化线", "", parent, now)
	child = child.Checkpoint("child-change", now.Add(time.Hour))
	plan, _ := PlanFastForwardMerge(child, parent)

	nextChild, nextParent, err := CompleteFastForwardMerge(
		child,
		parent,
		plan,
		now.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("CompleteFastForwardMerge() error = %v", err)
	}
	if nextParent.HeadDigest != "child-change" || nextParent.Generation != 2 {
		t.Fatalf("merged parent = %#v", nextParent)
	}
	if nextChild.UpstreamDigest != nextParent.HeadDigest {
		t.Fatalf("merged child = %#v", nextChild)
	}

	nextChild = nextChild.Checkpoint("second-child-change", now.Add(3*time.Hour))
	if _, err := PlanFastForwardMerge(nextChild, nextParent); err != nil {
		t.Fatalf("later PlanFastForwardMerge() error = %v", err)
	}
}

func TestBranchValidationAndMergeGuards(t *testing.T) {
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	parent, _ := NewMainBranch("main-id", "project-id", "baseline", now)

	if _, err := ForkBranch("branch-id", "  ", "", parent, now); !errors.Is(err, ErrBlankBranchName) {
		t.Fatalf("ForkBranch() error = %v, want ErrBlankBranchName", err)
	}
	child, _ := ForkBranch("branch-id", "黑化线", "", parent, now)
	if _, err := PlanFastForwardMerge(child, parent); !errors.Is(err, ErrBranchUnchanged) {
		t.Fatalf("PlanFastForwardMerge() error = %v, want ErrBranchUnchanged", err)
	}
	unrelated, _ := NewMainBranch("other-id", "project-id", "other", now)
	child = child.Checkpoint("child-change", now.Add(time.Hour))
	if _, err := PlanFastForwardMerge(child, unrelated); !errors.Is(err, ErrInvalidBranchTarget) {
		t.Fatalf("PlanFastForwardMerge() error = %v, want ErrInvalidBranchTarget", err)
	}
}
