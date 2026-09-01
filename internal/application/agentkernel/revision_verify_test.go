package agentkernel

import (
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const (
	verifyOriginalBody = "潮钟三响。阿绫推门。"
	verifyRevisedBody  = "潮钟哑响。阿绫推门。"
	verifyReviewJSON   = `{"quality_status":"needs_review","summary":"两处问题","findings":[` +
		`{"id":"f1","category":"continuity","severity":"blocker","status":"open","source":"model_review","message":"钟声与前章矛盾","constraint_ref":"chapter:document"},` +
		`{"id":"f2","category":"motivation","severity":"major","status":"open","source":"model_review","message":"动机不足","constraint_ref":"chapter:document"}]}`
	verifyRevisionJSON = `{"title":"潮钟","body":"潮钟哑响。阿绫推门。",` +
		`"resolutions":[{"finding_id":"f1","status":"resolved","note":"已改写钟声"},{"finding_id":"f2","status":"retained","note":"保留"}],` +
		`"changes":[{"finding_id":"f1","original_text":"潮钟三响","revised_text":"潮钟哑响"}]}`
)

func verifyNodeFixture() (FrozenNode, map[string]agentdomain.RunStep, []agentdomain.Artifact) {
	node := FrozenNode{
		Capability:     agentdomain.CapabilityRevisionVerify,
		InputSelectors: []string{"node:draft", "node:review", "node:revision"},
	}
	latest := map[string]agentdomain.RunStep{
		"draft": {ID: "draft-step", NodeID: "draft", Capability: agentdomain.CapabilityProseWriter,
			Status: agentdomain.StepCompleted, PrimaryArtifactID: "draft-artifact"},
		"review": {ID: "review-step", NodeID: "review", Capability: agentdomain.CapabilityContinuityReview,
			Status: agentdomain.StepCompleted, PrimaryArtifactID: "review-artifact"},
		"revision": {ID: "revision-step", NodeID: "revision", Capability: agentdomain.CapabilityBoundedRevision,
			Status: agentdomain.StepCompleted, PrimaryArtifactID: "revision-artifact"},
	}
	artifacts := []agentdomain.Artifact{
		{ID: "draft-artifact", StepID: "draft-step", Kind: agentdomain.ArtifactDraft, Content: verifyOriginalBody},
		{ID: "review-artifact", StepID: "review-step", Kind: agentdomain.ArtifactReview, Content: verifyReviewJSON},
		{ID: "revision-artifact", StepID: "revision-step", Kind: agentdomain.ArtifactRevision, Content: verifyRevisedBody},
		{ID: "revision-json", StepID: "revision-step", Kind: agentdomain.ArtifactStructuredOutput, Content: verifyRevisionJSON},
	}
	return node, latest, artifacts
}

// 裁定“已解决”却给不出可在修订稿中定位的证据时，不予采信并退回未解决。
func TestDecodeRevisionVerdictRejectsResolvedWithoutLocatableEvidence(t *testing.T) {
	node, latest, artifacts := verifyNodeFixture()
	content := `{"summary":"复核完成","verdicts":[` +
		`{"finding_id":"f1","verified_status":"resolved","rationale":"看起来已经改好"},` +
		`{"finding_id":"f2","verified_status":"retained","rationale":"修订者保留"}],"regressions":[]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskVerifyRevision, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	verdict := decoded.(*agentdomain.RevisionVerdictOutput)
	if verdict.Verdicts[0].VerifiedStatus != agentdomain.FindingOpen {
		t.Fatalf("unproven resolution was accepted: %#v", verdict.Verdicts[0])
	}
	if verdict.Verdicts[1].VerifiedStatus != agentdomain.FindingRetained {
		t.Fatalf("verdicts = %#v", verdict.Verdicts)
	}
}

// 引文能在修订稿中定位时保留已解决裁定，并由应用层重算 rune 范围。
func TestDecodeRevisionVerdictKeepsProvenResolutionAndRecomputesScope(t *testing.T) {
	node, latest, artifacts := verifyNodeFixture()
	content := `{"summary":"复核完成","verdicts":[` +
		`{"finding_id":"f1","verified_status":"resolved","rationale":"钟声已与前章一致","quote":"潮钟哑响","scope_start":999,"scope_end":1000},` +
		`{"finding_id":"f2","verified_status":"cannot_resolve","rationale":"需作者补充设定"}],"regressions":[]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskVerifyRevision, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	verdict := decoded.(*agentdomain.RevisionVerdictOutput)
	first := verdict.Verdicts[0]
	if first.VerifiedStatus != agentdomain.FindingResolved || first.ScopeStart == nil || *first.ScopeStart != 0 || *first.ScopeEnd != 4 {
		t.Fatalf("verdict = %#v", first)
	}
	if err := validateRevisionVerify(verdict, node, latest, artifacts); err != nil {
		t.Fatalf("validateRevisionVerify: %v", err)
	}
}

// 裁定缺失或 ID 错乱时补位为未解决，绝不因模型少答一条而让问题静默通过。
func TestDecodeRevisionVerdictFillsMissingVerdictsAsUnresolved(t *testing.T) {
	node, latest, artifacts := verifyNodeFixture()
	content := `{"summary":"复核完成","verdicts":[` +
		`{"finding_id":"unknown","verified_status":"resolved","rationale":"已解决","quote":"潮钟哑响"}],"regressions":[]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskVerifyRevision, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	verdict := decoded.(*agentdomain.RevisionVerdictOutput)
	if len(verdict.Verdicts) != 2 {
		t.Fatalf("verdicts = %#v", verdict.Verdicts)
	}
	if verdict.Verdicts[0].FindingID != "f1" || verdict.Verdicts[0].VerifiedStatus != agentdomain.FindingResolved {
		t.Fatalf("reassigned verdict = %#v", verdict.Verdicts[0])
	}
	if verdict.Verdicts[1].FindingID != "f2" || verdict.Verdicts[1].VerifiedStatus != agentdomain.FindingOpen {
		t.Fatalf("missing verdict was not filled conservatively: %#v", verdict.Verdicts[1])
	}
}

// 回归问题使用独立 ID 空间，避免与原问题重复导致对账失败。
func TestDecodeRevisionVerdictIsolatesRegressionIdentifiers(t *testing.T) {
	node, latest, artifacts := verifyNodeFixture()
	content := `{"summary":"复核完成","verdicts":[` +
		`{"finding_id":"f1","verified_status":"resolved","rationale":"已解决","quote":"潮钟哑响"},` +
		`{"finding_id":"f2","verified_status":"retained","rationale":"保留"}],` +
		`"regressions":[{"id":"f1","category":"pacing","severity":"major","status":"open","source":"model_review","message":"修订后节奏被打断","quote":"阿绫推门"}]}`

	decoded, err := decodeNodeOutput(agentdomain.TaskVerifyRevision, node, latest, artifacts, content)
	if err != nil {
		t.Fatalf("decodeNodeOutput: %v", err)
	}
	verdict := decoded.(*agentdomain.RevisionVerdictOutput)
	if len(verdict.Regressions) != 1 || verdict.Regressions[0].ID != "revision-regression-1" {
		t.Fatalf("regressions = %#v", verdict.Regressions)
	}
	if verdict.Regressions[0].ScopeStart == nil || *verdict.Regressions[0].ScopeStart != 5 {
		t.Fatalf("regression evidence = %#v", verdict.Regressions[0])
	}
}

// 验证者只能看到改动明细，不能看到修订模型的处置状态与说辞。
func TestChangeEvidenceStripsRevisionClaims(t *testing.T) {
	node, latest, artifacts := verifyNodeFixture()
	_, resolutions, changes, _, err := verificationInputs(node, latest, artifacts)
	if err != nil {
		t.Fatalf("verificationInputs: %v", err)
	}
	if len(resolutions) != 2 || len(changes) != 1 {
		t.Fatalf("resolutions = %#v changes = %#v", resolutions, changes)
	}
	evidence, err := changeEvidence(changes)
	if err != nil {
		t.Fatalf("changeEvidence: %v", err)
	}
	for _, leaked := range []string{"resolved", "retained", "已改写钟声", "status", "note"} {
		if contains(evidence, leaked) {
			t.Fatalf("change evidence leaked revision claim %q: %s", leaked, evidence)
		}
	}
	if !contains(evidence, "潮钟哑响") {
		t.Fatalf("change evidence lost the actual edit: %s", evidence)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if haystack[index:index+len(needle)] == needle {
			return index
		}
	}
	return -1
}
