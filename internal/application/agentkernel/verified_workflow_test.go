package agentkernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

const verifiedChapterMaxTokens = 20_000

var verifiedWorkflowPrefix = []providerReply{
	{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`, inputTokens: 10, outputTokens: 20},
	{text: `{"title":"空船","body":"雾港钟楼三响。潮声里没有脚步。","completion_note":"完成"}`, inputTokens: 20, outputTokens: 30},
	{text: `{"quality_status":"needs_review","summary":"连续性存疑","findings":[{"id":"f1","category":"continuity","severity":"blocker","status":"open","source":"model_review","message":"钟声与前章矛盾","constraint_ref":"chapter:document"}]}`, inputTokens: 15, outputTokens: 20},
	{text: `{"title":"空船","body":"雾港钟楼三响。潮声里仍没有脚步。","resolutions":[{"finding_id":"f1","status":"resolved","note":"已处理"}],"changes":[{"finding_id":"f1","original_text":"潮声里没有脚步。","revised_text":"潮声里仍没有脚步。"}]}`, inputTokens: 20, outputTokens: 30},
}

func verifiedChapterReplies(verdict string) []providerReply {
	replies := append([]providerReply(nil), verifiedWorkflowPrefix...)
	return append(replies, providerReply{text: verdict, inputTokens: 15, outputTokens: 20})
}

// 修订模型声称已解决但复核判定问题仍在时，最终质量以复核结论为准。
func TestVerifiedChapterWorkflowBlocksUnprovenRevisionClaim(t *testing.T) {
	service, store, provider := newRunnerHarness(t, verifiedChapterReplies(
		`{"summary":"复核完成","verdicts":[{"finding_id":"f1","verified_status":"open","rationale":"改动只调整了措辞，未处理钟声与前章的矛盾"}],"regressions":[]}`,
	))
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.VerifiedChapterWorkflowVersion, verifiedChapterMaxTokens)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 5 {
		t.Fatalf("completed=%#v requests=%d", completed, len(provider.requests))
	}
	if completed.QualityStatus != agentdomain.QualityBlocked {
		t.Fatalf("quality = %s, want blocked because the claimed resolution failed verification", completed.QualityStatus)
	}
	report := verificationGateReport(t, store, completed)
	if report.Verification == nil || report.Verification.ClaimedResolved != 1 || report.Verification.Overturned != 1 {
		t.Fatalf("verification report = %+v", report.Verification)
	}
}

// 复核确认解决时保持通过，避免复核节点变成无条件降级。
func TestVerifiedChapterWorkflowPassesConfirmedRevision(t *testing.T) {
	service, store, provider := newRunnerHarness(t, verifiedChapterReplies(
		`{"summary":"复核完成","verdicts":[{"finding_id":"f1","verified_status":"resolved","rationale":"修订稿已与前章钟声一致","quote":"潮声里仍没有脚步。"}],"regressions":[]}`,
	))
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.VerifiedChapterWorkflowVersion, verifiedChapterMaxTokens)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 5 {
		t.Fatalf("quality=%s requests=%d", completed.QualityStatus, len(provider.requests))
	}
	report := verificationGateReport(t, store, completed)
	if report.Verification == nil || report.Verification.ConfirmedResolved != 1 || report.Verification.Overturned != 0 {
		t.Fatalf("verification report = %+v", report.Verification)
	}
}

// 修订引入的新问题由复核登记，并直接进入未解决计数。
func TestVerifiedChapterWorkflowRecordsRevisionRegression(t *testing.T) {
	service, store, _ := newRunnerHarness(t, verifiedChapterReplies(
		`{"summary":"复核完成","verdicts":[{"finding_id":"f1","verified_status":"resolved","rationale":"钟声已一致","quote":"潮声里仍没有脚步。"}],`+
			`"regressions":[{"id":"r1","category":"pacing","severity":"major","status":"open","source":"model_review","message":"新增副词破坏了原有节奏","quote":"潮声里仍没有脚步。"}]}`,
	))
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.VerifiedChapterWorkflowVersion, verifiedChapterMaxTokens)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityNeedsReview {
		t.Fatalf("quality = %s, want needs_review because the revision introduced a major regression", completed.QualityStatus)
	}
	report := verificationGateReport(t, store, completed)
	if report.Verification == nil || report.Verification.Regressions != 1 || report.Unresolved.Major != 1 {
		t.Fatalf("gate report = %+v", report)
	}
}

// 审阅通过时修订与复核都不触发，复核节点不应增加无问题章节的成本。
func TestVerifiedChapterWorkflowSkipsVerificationWhenReviewPasses(t *testing.T) {
	service, _, provider := newRunnerHarness(t, []providerReply{
		verifiedWorkflowPrefix[0],
		verifiedWorkflowPrefix[1],
		{text: `{"quality_status":"passed","summary":"模型审阅未发现问题","findings":[]}`, inputTokens: 15, outputTokens: 20},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.VerifiedChapterWorkflowVersion, verifiedChapterMaxTokens)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 3 {
		t.Fatalf("quality=%s requests=%d", completed.QualityStatus, len(provider.requests))
	}
	for _, step := range completed.Steps {
		if step.Capability != agentdomain.CapabilityRevisionVerify {
			continue
		}
		if step.Status != agentdomain.StepCompleted || step.OutputTokens != 0 {
			t.Fatalf("skipped verification step = %#v", step)
		}
	}
}

// v1 工作流保持原有语义，修订自述仍是最终门禁输入。
func TestUnverifiedChapterWorkflowKeepsLegacyBehaviour(t *testing.T) {
	service, _, provider := newRunnerHarness(t, verifiedWorkflowPrefix)
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17_500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 4 {
		t.Fatalf("quality=%s requests=%d", completed.QualityStatus, len(provider.requests))
	}
}

func verificationGateReport(t *testing.T, store *memoryStore, run agentdomain.Run) quality.GateReport {
	t.Helper()
	var stepID string
	for _, step := range run.Steps {
		if step.Capability == agentdomain.CapabilityRevisionVerify {
			stepID = step.ID
		}
	}
	if stepID == "" {
		t.Fatal("run has no verification step")
	}
	for _, artifact := range store.artifacts {
		if artifact.StepID != stepID || artifact.Kind != agentdomain.ArtifactQualityReport {
			continue
		}
		var report quality.GateReport
		if err := json.Unmarshal([]byte(artifact.Content), &report); err != nil {
			t.Fatalf("decode gate report: %v", err)
		}
		return report
	}
	t.Fatal("verification step has no quality report")
	return quality.GateReport{}
}
