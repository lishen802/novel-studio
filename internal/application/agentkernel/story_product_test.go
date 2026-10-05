package agentkernel

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestStoryProductWorkflowsRunWithFakeProvider(t *testing.T) {
	passedReview := `{"quality_status":"passed","summary":"满足结构约束","findings":[]}`
	cases := []struct {
		id       string
		version  int
		budget   int
		generate string
		want     string
	}{
		{
			id: agentassets.StoryDirectionsWorkflowID, version: agentassets.StoryProductWorkflowVersion, budget: 7000,
			generate: `{"directions":[{"id":"d1","hook":"守关人放走仇敌","genre":"奇幻悬疑","protagonist":"守关人阿绫","core_conflict":"守城与救人","growth_cost":"失去军职","pacing":"中速","ending":"开城救人"},{"id":"d2","hook":"潮汐神谕错位","genre":"历史传奇","protagonist":"潮汐祭司","core_conflict":"神谕与民生","growth_cost":"信仰崩解","pacing":"慢热","ending":"改写神谕"},{"id":"d3","hook":"假身份潜入盐帮","genre":"江湖复仇","protagonist":"失踪少主","core_conflict":"复仇与认亲","growth_cost":"放弃姓氏","pacing":"快节奏","ending":"揭穿盐帮"}]}`,
			want:     `"id":"d1"`,
		},
		{
			id: agentassets.StoryProposalWorkflowID, version: agentassets.StoryProposalWorkflowVersion, budget: 11500,
			generate: `{"title":"潮汐关","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","开城救人"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"开城救人","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[{"strength":"hard","statement":"钟楼必须三响","scope_hint":"chapter"}]}`,
			want:     `"sell_point":"守关人必须放走仇敌"`,
		},
		{
			id: agentassets.StoryPeopleWorldWorkflowID, version: agentassets.StoryPeopleWorldWorkflowVersion, budget: 7500,
			generate: `{"characters":[{"id":"char-1","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["本方向不需要势力和地点"]}`,
			want:     `"role":"protagonist"`,
		},
		{
			id: agentassets.StoryRoadmapWorkflowID, version: agentassets.StoryRoadmapWorkflowVersion, budget: 10000,
			generate: `{"book":{"id":"book-1","title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},"volumes":[{"id":"vol-1","title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1}],"arcs":[{"id":"arc-1","volume_ref":"vol-1","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","reader_expectation":"读者应感到守关人将被亲情撕裂","stage_payoff":"开城救人并失去军职","irreversible_change":"阿绫不再以军令为最高准则","ending_contribution":"为终局选择救人立下先例","commitments":[{"id":"cmt-1","statement":"旧令牌兑现身份","fulfillment_range":"本弧末章","related_refs":["arc-1"]}],"retained_questions":[{"id":"q-1","statement":"潮汐神谕是否被篡改","reason":"留给后续卷揭示"}],"foreshadowing":["旧令牌"],"position":1}],"near_term_chapters":[{"id":"ch-1","arc_ref":"arc-1","title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":[{"strength":"hard","statement":"钟楼三响"}],"position":1},{"id":"ch-2","arc_ref":"arc-1","title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":2},{"id":"ch-3","arc_ref":"arc-1","title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":[{"strength":"soft","statement":"潮汐律"}],"position":3}]}`,
			want:     `"reader_expectation"`,
		},
	}
	for _, test := range cases {
		t.Run(fmt.Sprintf("%s@%d", test.id, test.version), func(t *testing.T) {
			service, store, provider := newRunnerHarness(t, []providerReply{
				{text: test.generate, inputTokens: 10, outputTokens: 20},
				{text: passedReview, inputTokens: 8, outputTokens: 12},
			})
			request := qualityRequest(test.id, test.version, test.budget)
			run, err := service.CreateRun(context.Background(), request)
			if err != nil {
				t.Fatalf("CreateRun: %v", err)
			}
			completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
			if err != nil {
				t.Fatalf("ExecuteRun: %v", err)
			}
			if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 2 {
				t.Fatalf("completed=%#v requests=%d", completed, len(provider.requests))
			}
			architect := completed.Steps[0]
			if architect.Capability != agentdomain.CapabilityStoryArchitect {
				t.Fatalf("first step capability = %s", architect.Capability)
			}
			artifact, ok := artifactByID(store.artifacts, architect.PrimaryArtifactID)
			if !ok || !strings.Contains(artifact.Content, test.want) {
				t.Fatalf("architect artifact=%#v", artifact)
			}
			if provider.requests[0].Model != "planner-model" || provider.requests[1].Model != "reviewer-model" {
				t.Fatalf("models = %#v", provider.requests)
			}
		})
	}
}

func TestStoryProposalRejectsV1PayloadOnProductSchema(t *testing.T) {
	service, _, _ := newRunnerHarness(t, []providerReply{{
		text: `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`,
	}})
	request := qualityRequest(agentassets.StoryProposalWorkflowID, agentassets.StoryProposalWorkflowVersion, 11500)
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err == nil || failed.Status != agentdomain.RunFailed {
		t.Fatalf("expected product schema rejection, failed=%#v err=%v", failed, err)
	}
}

func TestAuthorMaterialWorkflowKeepsNullableStagesAndGapEvidence(t *testing.T) {
	for _, version := range []int{2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			payload := `{"entry_id":"source-1","source_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","proposal":null,"people_world":null,"roadmap":null,"gaps":[{"id":"p","stage":"proposal","field_path":"/ending","kind":"required","reason":"结局未定","quote":"","suggested_action":"补充结局"},{"id":"c","stage":"people_world","field_path":"/characters","kind":"required","reason":"人物未定","quote":"","suggested_action":"补充人物"},{"id":"r","stage":"roadmap","field_path":"/book","kind":"required","reason":"结构未定","quote":"","suggested_action":"补充结构"}],"field_sources":[]}`
			service, store, provider := newRunnerHarness(t, []providerReply{{text: payload, inputTokens: 10, outputTokens: 20}, {text: `{"quality_status":"passed","summary":"缺口保留","findings":[]}`, inputTokens: 5, outputTokens: 10}})
			request := qualityRequest("story.author_material", version, 30000)
			run, err := service.CreateRun(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			done, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
			if err != nil {
				t.Fatal(err)
			}
			if done.Status != agentdomain.RunCompleted || len(provider.requests) != 2 {
				t.Fatalf("运行未完成: %v", done.Status)
			}
			artifact, ok := artifactByID(store.artifacts, done.Steps[0].PrimaryArtifactID)
			if !ok || !strings.Contains(artifact.Content, `"proposal":null`) || !strings.Contains(artifact.Content, "结局未定") {
				t.Fatal("解析缺口丢失或被补造")
			}
			if provider.requests[0].Model == provider.requests[1].Model {
				t.Fatal("解析与审阅应允许不同模型")
			}
		})
	}

}
