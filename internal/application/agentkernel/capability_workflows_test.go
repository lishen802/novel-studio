package agentkernel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

func TestQualityChapterWorkflowRunsRecordedProviderWithReviewIsolation(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入雾港","opening_hook":"空船靠岸","ending_hook":"船票署名","scenes":[{"id":"s1","goal":"登船","conflict":"守卫阻拦","outcome":"成功潜入"}]}`, inputTokens: 10, outputTokens: 20},
		{text: `{"title":"空船","body":"雾港钟楼三响。潮声里没有脚步。","completion_note":"完成潜入目标"}`, inputTokens: 20, outputTokens: 30},
		{text: `{"quality_status":"needs_review","summary":"一处表达需收紧","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"语气不够悬疑","suggestion":"增加不确定感","constraint_ref":"style:suspense"}]}`, inputTokens: 15, outputTokens: 20},
		{text: `{"title":"空船","body":"雾港钟楼三响。潮声里仍没有脚步。","resolutions":[{"finding_id":"f1","status":"resolved","note":"增加不确定感"}],"changes":[{"finding_id":"f1","original_text":"潮声里没有脚步。","revised_text":"潮声里仍没有脚步。"}]}`, inputTokens: 20, outputTokens: 30},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 4 {
		t.Fatalf("completed=%#v requests=%d", completed, len(provider.requests))
	}
	if completed.QualityStatus != agentdomain.QualityPassed {
		t.Fatalf("final quality=%s, want passed", completed.QualityStatus)
	}
	wantModels := []string{"planner-model", "writer-model", "reviewer-model", "writer-model"}
	for index, model := range wantModels {
		if provider.requests[index].Model != model {
			t.Fatalf("request %d model=%s, want %s", index, provider.requests[index].Model, model)
		}
	}
	if strings.Contains(provider.requests[2].Input, `"chapter_goal"`) {
		t.Fatal("review input leaked planning artifact")
	}
	if strings.Contains(provider.requests[1].Input, `"findings"`) {
		t.Fatal("writer input leaked review result")
	}
	if !strings.Contains(provider.requests[3].Input, `"id":"f1"`) {
		t.Fatal("revision did not receive normalized review")
	}
	if len(store.findings) != 1 || store.findings[0].Source != agentdomain.FindingModelReview {
		t.Fatalf("findings=%#v", store.findings)
	}
	if countArtifacts(store.artifacts, agentdomain.ArtifactLintReport) != 2 || countArtifacts(store.artifacts, agentdomain.ArtifactQualityReport) != 3 {
		t.Fatalf("quality artifacts=%#v", store.artifacts)
	}
	for _, step := range completed.Steps {
		if step.PromptBinding.Version != 1 || step.SchemaBinding.Version != 1 || step.InputTokens == 0 || step.OutputTokens == 0 {
			t.Fatalf("step trace incomplete: %#v", step)
		}
	}
}

func TestQualityGateRepairsForbiddenItemEvenWhenModelReviewPasses(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。黑刃出鞘。","completion_note":"完成"}`},
		{text: `{"quality_status":"passed","summary":"模型审阅未发现问题","findings":[]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。短刀出鞘。","resolutions":[{"finding_id":"id-2:constraint.forbidden_item:001","status":"resolved","note":"移除禁止项"}],"changes":[{"finding_id":"id-2:constraint.forbidden_item:001","original_text":"黑刃","revised_text":"短刀"}]}`},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	request.QualitySpec.ForbiddenItems = []quality.Constraint{{ID: "forbidden-1", Text: "黑刃"}}
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 4 {
		t.Fatalf("completed=%#v requests=%d", completed, len(provider.requests))
	}
	if !strings.Contains(provider.requests[3].Input, `"source":"deterministic_lint"`) {
		t.Fatal("revision did not receive deterministic lint finding")
	}
	if len(store.findings) != 1 || store.findings[0].Source != agentdomain.FindingDeterministicLint {
		t.Fatalf("findings=%#v", store.findings)
	}
}

func TestFinalQualityGateBlocksViolationIntroducedByRevision(t *testing.T) {
	service, _, _ := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。短刀出鞘。","completion_note":"完成"}`},
		{text: `{"quality_status":"needs_review","summary":"兵器缺少辨识度","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"兵器描写平淡","constraint_ref":"style:weapon"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。黑刃出鞘。","resolutions":[{"finding_id":"f1","status":"resolved","note":"增强辨识度"}],"changes":[{"finding_id":"f1","original_text":"短刀","revised_text":"黑刃"}]}`},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	request.QualitySpec.ForbiddenItems = []quality.Constraint{{ID: "forbidden-1", Text: "黑刃"}}
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityBlocked {
		t.Fatalf("completed=%#v", completed)
	}
}

func TestFinalQualityGateKeepsRetainedMajorAsNeedsReview(t *testing.T) {
	service, _, _ := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。短刀出鞘。","completion_note":"完成"}`},
		{text: `{"quality_status":"needs_review","summary":"兵器描写仍有风险","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"兵器描写平淡","constraint_ref":"style:weapon"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。短刀出鞘。","resolutions":[{"finding_id":"f1","status":"retained","note":"作者规格要求保持克制"}],"changes":[]}`},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityNeedsReview {
		t.Fatalf("completed=%#v", completed)
	}
}

func TestBaselineLintBlockerCannotBeOverriddenBySuccessfulGeneration(t *testing.T) {
	service, store, _ := newRunnerHarness(t, []providerReply{{
		text: "{\"title\":\"空船\",\"body\":\"```markdown\\n雾港落雨。\\n```\",\"completion_note\":\"完成\"}",
	}})
	request := baselineRequest()
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.QualityStatus != agentdomain.QualityBlocked || countArtifacts(store.artifacts, agentdomain.ArtifactLintReport) != 1 {
		t.Fatalf("completed=%#v artifacts=%#v", completed, store.artifacts)
	}
}

func TestQualityBlueprintWorkflowReturnsStructuredRevision(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{
		{text: `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`, inputTokens: 10, outputTokens: 20},
		{text: `{"quality_status":"needs_review","summary":"核心冲突不足","findings":[{"id":"f1","category":"causality","severity":"major","status":"open","source":"model_review","message":"冲突缺少亲情代价","constraint_ref":"blueprint:core_conflict"}]}`, inputTokens: 10, outputTokens: 15},
		{text: `{"blueprint":{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪与守护亲情冲突","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]},"resolutions":[{"finding_id":"f1","status":"resolved","note":"补入亲情代价"}],"changes":[{"finding_id":"f1","original_text":"追查失踪","revised_text":"追查失踪与守护亲情冲突"}]}`, inputTokens: 15, outputTokens: 25},
	})
	request := qualityRequest(agentassets.QualityBlueprintWorkflowID, agentassets.QualityBlueprintWorkflowVersion, 11500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 3 || provider.requests[2].Model != "planner-model" {
		t.Fatalf("completed=%#v requests=%#v", completed, provider.requests)
	}
	last := completed.Steps[len(completed.Steps)-1]
	artifact, ok := artifactByID(store.artifacts, last.PrimaryArtifactID)
	if !ok || !strings.Contains(artifact.Content, `"core_conflict":"追查失踪与守护亲情冲突"`) || strings.Contains(artifact.Content, `"body"`) {
		t.Fatalf("blueprint revision artifact=%#v", artifact)
	}
}

func TestBoundedRevisionRejectsLostRequiredAnchor(t *testing.T) {
	service, _, _ := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。潮声里没有脚步。","completion_note":"完成"}`},
		{text: `{"quality_status":"needs_review","summary":"需修改","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"修改钟楼描写","constraint_ref":"style:suspense"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼寂静。潮声里没有脚步。","resolutions":[{"finding_id":"f1","status":"resolved","note":"修改钟楼描写"}],"changes":[{"finding_id":"f1","original_text":"钟楼三响","revised_text":"钟楼寂静"}]}`},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if !errors.Is(err, ErrInvalidOutput) || !strings.Contains(err.Error(), "lost required anchor") || failed.Status != agentdomain.RunFailed {
		t.Fatalf("failed=%#v err=%v", failed, err)
	}
}

func TestBoundedRevisionRejectsUndeclaredRewrite(t *testing.T) {
	service, _, _ := newRunnerHarness(t, []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"空船","ending_hook":"门开","scenes":[{"id":"s1","goal":"登船","conflict":"守卫","outcome":"成功"}]}`},
		{text: `{"title":"空船","body":"雾港钟楼三响。潮声里没有脚步。","completion_note":"完成"}`},
		{text: `{"quality_status":"needs_review","summary":"需修改","findings":[{"id":"f1","category":"style","severity":"major","status":"open","source":"model_review","message":"修改尾句","constraint_ref":"style:suspense"}]}`},
		{text: `{"title":"空船","body":"钟楼三响。潮声里仍没有脚步。","resolutions":[{"finding_id":"f1","status":"resolved","note":"修改尾句"}],"changes":[{"finding_id":"f1","original_text":"潮声里没有脚步。","revised_text":"潮声里仍没有脚步。"}]}`},
	})
	request := qualityRequest(agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion, 17500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if !errors.Is(err, ErrInvalidOutput) || !strings.Contains(err.Error(), "outside declared evidence") || failed.Status != agentdomain.RunFailed {
		t.Fatalf("failed=%#v err=%v", failed, err)
	}
}

func qualityRequest(workflowID string, workflowVersion, maxRunTokens int) CreateRequest {
	request := baselineRequest()
	request.WorkflowID = workflowID
	request.WorkflowVersion = workflowVersion
	request.Profile.MaxRunTokens = maxRunTokens
	request.Profile.Roles = []agentdomain.ResolvedRole{
		resolvedRole(agentdomain.RolePlanner, "planner-model"),
		resolvedRole(agentdomain.RoleWriter, "writer-model"),
		resolvedRole(agentdomain.RoleContinuityReviewer, "reviewer-model"),
	}
	request.Sources = []Source{{
		Kind: agentdomain.ManifestAnchor, ID: "anchor-1", Version: "1", Content: "钟楼三响",
		Priority: 1000, SelectionReason: "固定情节锚点", Required: true,
	}}
	request.QualitySpec = quality.Spec{TargetRunes: 0}
	return request
}
