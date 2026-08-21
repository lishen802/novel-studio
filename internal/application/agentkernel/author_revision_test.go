package agentkernel

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestBuildAuthorRevisionCandidateEnforcesTargetsAndStaleBase(t *testing.T) {
	now := time.Unix(10_000, 0).UTC()
	base := agentdomain.Artifact{ID: "base", RunID: "old-run", StepID: "old-step", Kind: agentdomain.ArtifactDraft, Content: "第一段保持。第二段需要修改。", CreatedAt: now}
	base.SHA256 = agentdomain.SHA256Hex([]byte(base.Content))
	directive := agentdomain.RevisionDirective{Version: 1, Mode: agentdomain.RevisionAuthorDirected, BaseArtifactID: base.ID, BaseDigest: base.SHA256, TargetRefs: []string{"paragraph:2"}, PreserveRefs: []string{"paragraph:1"}, AuthorInstructionRef: "instruction-1"}
	output := &agentdomain.ReviseDraftOutput{Title: "标题", Body: "第一段保持。第二段已经改好。", Changes: []agentdomain.RevisionChange{{TargetRef: "paragraph:2", OriginalText: "第二段需要修改", RevisedText: "第二段已经改好"}}}
	candidate, err := BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: directive, Base: base, Output: output, CreatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Relation.RelatedArtifactID != base.ID || candidate.Artifact.Content != output.Body {
		t.Fatalf("candidate = %+v", candidate)
	}

	stale := directive
	stale.BaseDigest = agentdomain.SHA256Hex([]byte("changed"))
	_, err = BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: stale, Base: base, Output: output, CreatedAt: now})
	if !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("stale base error = %v", err)
	}

	outOfScope := *output
	outOfScope.Body = "第一段被改。第二段已经改好。"
	_, err = BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: directive, Base: base, Output: &outOfScope, CreatedAt: now})
	if err == nil {
		t.Fatal("out-of-scope author revision was accepted")
	}
}

func TestBuildAuthorBlueprintRevisionCandidateKeepsIndependentLineage(t *testing.T) {
	now := time.Unix(20_000, 0).UTC()
	baseContent := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	base := agentdomain.Artifact{ID: "base-blueprint", RunID: "old-run", StepID: "old-step", Kind: agentdomain.ArtifactPlan, Content: baseContent, CreatedAt: now}
	base.SHA256 = agentdomain.SHA256Hex([]byte(base.Content))
	directive := agentdomain.RevisionDirective{Version: 1, Mode: agentdomain.RevisionAuthorDirected, BaseArtifactID: base.ID, BaseDigest: base.SHA256, TargetRefs: []string{"core_conflict"}, PreserveRefs: []string{"world_rules"}, AuthorInstructionRef: "instruction-2"}
	output := &agentdomain.ReviseBlueprintOutput{
		Blueprint: agentdomain.StoryBlueprintOutput{Title: "雾港", Positioning: "悬疑成长", CoreConflict: "追查失踪与守护亲情冲突", ProtagonistArc: "从逃避到承担", WorldRules: []string{"潮汐决定航路"}, MajorMilestones: []string{"失踪", "追查", "抉择"}},
		Changes:   []agentdomain.RevisionChange{{TargetRef: "core_conflict", OriginalText: "追查失踪", RevisedText: "追查失踪与守护亲情冲突"}},
	}
	candidate, err := BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate-blueprint", Directive: directive, Base: base, Output: output, CreatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded agentdomain.StoryBlueprintOutput
	if json.Unmarshal([]byte(candidate.Artifact.Content), &decoded) != nil || decoded.CoreConflict != output.Blueprint.CoreConflict {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.Relation.Relation != agentdomain.ArtifactRevises || candidate.Relation.RelatedArtifactID != base.ID {
		t.Fatalf("relation = %+v", candidate.Relation)
	}
}
