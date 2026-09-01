package agentkernel

import (
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestDecodeDraftRevisionNormalizesInvalidTraceAndRestoresUntrackedChanges(t *testing.T) {
	original := "潮钟三响。阿绫推门。"
	review := `{"quality_status":"needs_review","summary":"两处问题","findings":[{"id":"f1","category":"hook","severity":"major","status":"open","source":"model_review","message":"开头缺少异样感","constraint_ref":"chapter:document"},{"id":"f2","category":"motivation","severity":"minor","status":"open","source":"model_review","message":"动作动机不足","constraint_ref":"chapter:document"}]}`
	node := FrozenNode{Capability: agentdomain.CapabilityBoundedRevision, InputSelectors: []string{"node:draft", "node:review"}}
	latest := map[string]agentdomain.RunStep{
		"draft":  {NodeID: "draft", Capability: agentdomain.CapabilityProseWriter, Status: agentdomain.StepCompleted, PrimaryArtifactID: "draft-artifact"},
		"review": {NodeID: "review", Capability: agentdomain.CapabilityContinuityReview, Status: agentdomain.StepCompleted, PrimaryArtifactID: "review-artifact"},
	}
	artifacts := []agentdomain.Artifact{{ID: "draft-artifact", Content: original}, {ID: "review-artifact", Content: review}}
	content := `{"title":"潮钟","body":"潮钟哑响。阿绫猛地推门，又回头望了一眼。","resolutions":[{"finding_id":"f1","status":"resolved","note":"增加异样感"},{"finding_id":"f1","status":"resolved","note":"补足动机"}],"changes":[{"finding_id":"f1","original_text":"潮钟三响","revised_text":"潮钟哑响"}]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskReviseDraft, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	revision := decoded.(*agentdomain.ReviseDraftOutput)
	if revision.Body != "潮钟哑响。阿绫推门。" || len(revision.Changes) != 1 {
		t.Fatalf("normalized revision = %#v", revision)
	}
	if revision.Resolutions[0].FindingID != "f1" || revision.Resolutions[0].Status != agentdomain.FindingResolved ||
		revision.Resolutions[1].FindingID != "f2" || revision.Resolutions[1].Status != agentdomain.FindingCannotResolve {
		t.Fatalf("normalized resolutions = %#v", revision.Resolutions)
	}
}

func TestNormalizeDraftRevisionCreatesMissingResolutionWithoutChangingBody(t *testing.T) {
	review := &agentdomain.ReviewBundleOutput{Findings: []agentdomain.ReviewFinding{{ID: "f1"}}}
	revision := &agentdomain.ReviseDraftOutput{Title: "潮钟", Body: "模型改写"}
	if err := normalizeDraftRevision(review, "原正文", revision); err != nil {
		t.Fatalf("normalizeDraftRevision: %v", err)
	}
	if revision.Body != "原正文" || len(revision.Resolutions) != 1 || revision.Resolutions[0].Status != agentdomain.FindingCannotResolve || len(revision.Changes) != 0 {
		t.Fatalf("normalized revision = %#v", revision)
	}
}

func TestDecodeDraftRevisionRestoresMissingTitleFromFrozenDraft(t *testing.T) {
	node := FrozenNode{Capability: agentdomain.CapabilityBoundedRevision, InputSelectors: []string{"node:draft", "node:review"}}
	latest := map[string]agentdomain.RunStep{
		"draft":  {ID: "draft-step", NodeID: "draft", Capability: agentdomain.CapabilityProseWriter, PrimaryArtifactID: "draft-body"},
		"review": {ID: "review-step", NodeID: "review", Capability: agentdomain.CapabilityContinuityReview, PrimaryArtifactID: "review-artifact"},
	}
	artifacts := []agentdomain.Artifact{
		{ID: "draft-body", StepID: "draft-step", Kind: agentdomain.ArtifactDraft, Content: "潮钟三响。"},
		{ID: "draft-json", StepID: "draft-step", Kind: agentdomain.ArtifactStructuredOutput, Content: `{"title":"潮钟","body":"潮钟三响。","completion_note":"完成"}`},
		{ID: "review-artifact", StepID: "review-step", Kind: agentdomain.ArtifactReview, Content: `{"quality_status":"needs_review","summary":"需修改","findings":[{"id":"f1","category":"hook","severity":"major","status":"open","source":"model_review","message":"开头偏弱","constraint_ref":"chapter:document"}]}`},
	}
	content := `{"body":"潮钟哑响。","resolutions":[{"finding_id":"f1","status":"resolved","note":"增强异样感"}],"changes":[{"finding_id":"f1","original_text":"潮钟三响","revised_text":"潮钟哑响"}]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskReviseDraft, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	revision := decoded.(*agentdomain.ReviseDraftOutput)
	if revision.Title != "潮钟" || revision.Body != "潮钟哑响。" {
		t.Fatalf("revision = %#v", revision)
	}
}
