package agentkernel

import (
	"context"
	"encoding/json"
	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"strings"
	"testing"
)

func TestStoryCandidateRevisionWorkflowFourKinds(t *testing.T) {
	for _, tc := range candidateRevisionFixtures {
		t.Run(tc.kind, func(t *testing.T) {
			original := agentdomain.StoryCandidateRevisionOutput{CandidateKind: tc.kind, PayloadSchemaRef: tc.schema, Payload: json.RawMessage(tc.payload)}
			raw, _ := json.Marshal(original)
			revised, err := (creationdomain.Candidate{PayloadJSON: tc.payload}).EditTextFields(map[string]string{tc.path: "作者修订的新文本"})
			if err != nil {
				t.Fatal(err)
			}
			output := original
			output.Payload = json.RawMessage(revised)
			reply, _ := json.Marshal(output)
			base, directive := authorDirectedBase()
			base.Content = string(raw)
			base.Digest = agentdomain.SHA256Hex(raw)
			directive.BaseDigest = base.Digest
			directive.TargetRefs = []string{tc.path}
			directive.PreserveRefs = nil
			req := authorDirectedRequest(base, directive)
			req.ChapterID = ""
			req.ChapterBaseRevision = 0
			req.WorkflowID = agentassets.StoryCandidateRevisionWorkflowID
			req.Profile.MaxRunTokens = 100000
			req.Profile.MaxContextChars = 100000
			req.Profile.Roles = append(req.Profile.Roles, resolvedRole(agentdomain.RoleContinuityReviewer, "independent-review-model"))
			registry, err := agentassets.Builtin()
			if err != nil {
				t.Fatal(err)
			}
			provider := &sequenceProvider{replies: []providerReply{{text: string(reply), inputTokens: 10, outputTokens: 30}, {text: `{"quality_status":"passed","summary":"修订符合要求","findings":[]}`, inputTokens: 10, outputTokens: 20}}}
			store := newMemoryStore()
			service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry}, fixedRunnerClock{}, &sequenceIDs{}, nil)
			run, err := service.CreateRun(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if frozen, ok := artifactByID(store.artifacts, base.ArtifactID); !ok || frozen.Content != base.Content {
				t.Fatal("缺少持久化基线工件")
			}
			completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
			if err != nil {
				t.Fatal(err)
			}
			if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityPassed || len(provider.requests) != 2 {
				t.Fatalf("run=%+v calls=%d", completed, len(provider.requests))
			}
			if !strings.Contains(provider.requests[1].Input, "作者修订的新文本") || !strings.Contains(provider.requests[1].Input, string(raw)) || !strings.Contains(provider.requests[1].Input, tc.path) {
				t.Fatal("独立审阅缺少基线、修改或授权范围")
			}
			if completed.Steps[0].Model == completed.Steps[1].Model {
				t.Fatal("未使用独立审阅模型")
			}
		})
	}
}

var candidateRevisionFixtures = []struct{ kind, schema, path, payload string }{
	{"direction_set", "story_directions.schema@1", "/directions/@d1/hook", `{"directions":[{"id":"d1","hook":"守关人放走仇敌","genre":"奇幻悬疑","protagonist":"守关人阿绫","core_conflict":"守城与救人","growth_cost":"失去军职","pacing":"中速","ending":"开城救人"},{"id":"d2","hook":"潮汐神谕错位","genre":"历史传奇","protagonist":"潮汐祭司","core_conflict":"神谕与民生","growth_cost":"信仰崩解","pacing":"慢热","ending":"改写神谕"},{"id":"d3","hook":"假身份潜入盐帮","genre":"江湖复仇","protagonist":"失踪少主","core_conflict":"复仇与认亲","growth_cost":"放弃姓氏","pacing":"快节奏","ending":"揭穿盐帮"}]}`},
	{"proposal", "story_blueprint.schema@2", "/title", `{"title":"潮汐关","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","开城救人"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"开城救人","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[{"strength":"hard","statement":"钟楼必须三响","scope_hint":"proposal"}]}`},
	{"people_world", "story_cast.schema@2", "/characters/@char-1/secret", `{"characters":[{"id":"char-1","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["本方向不需要势力和地点"]}`},
	{"roadmap", "story_roadmap.schema@2", "/book/title", `{"book":{"id":"book-1","title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},"volumes":[{"id":"vol-1","title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1}],"arcs":[{"id":"arc-1","volume_ref":"vol-1","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","foreshadowing":["旧令牌"],"position":1}],"near_term_chapters":[{"id":"ch-1","arc_ref":"arc-1","title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":[{"strength":"hard","statement":"钟楼三响"}],"position":1},{"id":"ch-2","arc_ref":"arc-1","title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":2},{"id":"ch-3","arc_ref":"arc-1","title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":[{"strength":"soft","statement":"潮汐律"}],"position":3}]}`},
}

func TestStoryCandidateRevisionKeepsRoadmapV3PlanningGoals(t *testing.T) {
	payload := `{"book":{"id":"book-1","title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},"volumes":[{"id":"vol-1","title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1}],"arcs":[{"id":"arc-1","volume_ref":"vol-1","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","reader_expectation":"读者应感到守关人将被亲情撕裂","stage_payoff":"开城救人并失去军职","irreversible_change":"阿绫不再以军令为最高准则","ending_contribution":"为终局选择救人立下先例","commitments":[{"id":"cmt-1","statement":"旧令牌兑现身份","fulfillment_range":"本弧末章","related_refs":["arc-1"]}],"retained_questions":[{"id":"q-1","statement":"潮汐神谕是否被篡改","reason":"留给后续卷揭示"}],"foreshadowing":["旧令牌"],"position":1}],"near_term_chapters":[{"id":"ch-1","arc_ref":"arc-1","title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":[{"strength":"hard","statement":"钟楼三响"}],"position":1},{"id":"ch-2","arc_ref":"arc-1","title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":2},{"id":"ch-3","arc_ref":"arc-1","title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":[{"strength":"soft","statement":"潮汐律"}],"position":3}]}`
	path := "/arcs/@arc-1/reader_expectation"
	original := agentdomain.StoryCandidateRevisionOutput{CandidateKind: "roadmap", PayloadSchemaRef: "story_roadmap.schema@3", Payload: json.RawMessage(payload)}
	raw, _ := json.Marshal(original)
	revised, err := (creationdomain.Candidate{PayloadJSON: payload}).EditTextFields(map[string]string{path: "作者改写后的读者期待"})
	if err != nil {
		t.Fatal(err)
	}
	output := original
	output.Payload = json.RawMessage(revised)
	reply, _ := json.Marshal(output)
	base, directive := authorDirectedBase()
	base.Content = string(raw)
	base.Digest = agentdomain.SHA256Hex(raw)
	directive.BaseDigest = base.Digest
	directive.TargetRefs = []string{path}
	directive.PreserveRefs = nil
	req := authorDirectedRequest(base, directive)
	req.ChapterID = ""
	req.ChapterBaseRevision = 0
	req.WorkflowID = agentassets.StoryCandidateRevisionWorkflowID
	req.WorkflowVersion = agentassets.StoryCandidateRevisionWorkflowVersionV2
	req.Profile.MaxRunTokens = 100000
	req.Profile.MaxContextChars = 100000
	req.Profile.Roles = append(req.Profile.Roles, resolvedRole(agentdomain.RoleContinuityReviewer, "independent-review-model"))
	registry, err := agentassets.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	provider := &sequenceProvider{replies: []providerReply{{text: string(reply), inputTokens: 10, outputTokens: 30}, {text: `{"quality_status":"passed","summary":"修订符合要求","findings":[]}`, inputTokens: 10, outputTokens: 20}}}
	store := newMemoryStore()
	service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry}, fixedRunnerClock{}, &sequenceIDs{}, nil)
	run, err := service.CreateRun(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityPassed {
		t.Fatalf("v3 路线图修订未完成: %+v", completed)
	}
	if !strings.Contains(revised, "作者改写后的读者期待") || !strings.Contains(revised, "开城救人并失去军职") || !strings.Contains(revised, "cmt-1") {
		t.Fatal("规划目标在修订中丢失")
	}
}

func TestStoryCandidateRevisionStopsUnsafeOutput(t *testing.T) {
	for _, scenario := range []string{"越界", "审阅失败", "基线无效", "严重问题"} {
		t.Run(scenario, func(t *testing.T) {
			tc := candidateRevisionFixtures[1]
			envelope := agentdomain.StoryCandidateRevisionOutput{CandidateKind: tc.kind, PayloadSchemaRef: tc.schema, Payload: json.RawMessage(tc.payload)}
			raw, _ := json.Marshal(envelope)
			base, directive := authorDirectedBase()
			base.Content = string(raw)
			base.Digest = agentdomain.SHA256Hex(raw)
			directive.BaseDigest = base.Digest
			directive.TargetRefs = []string{tc.path}
			directive.PreserveRefs = []string{"/ending"}
			changes := map[string]string{tc.path: "新标题"}
			if scenario == "越界" {
				changes["/ending"] = "锁值被改变"
			}
			payload, _ := (creationdomain.Candidate{PayloadJSON: tc.payload}).EditTextFields(changes)
			envelope.Payload = json.RawMessage(payload)
			reply, _ := json.Marshal(envelope)
			req := authorDirectedRequest(base, directive)
			req.ChapterID = ""
			req.ChapterBaseRevision = 0
			req.WorkflowID = agentassets.StoryCandidateRevisionWorkflowID
			req.Profile.MaxRunTokens = 100000
			req.Profile.MaxContextChars = 100000
			req.Profile.Roles = append(req.Profile.Roles, resolvedRole(agentdomain.RoleContinuityReviewer, "independent-review-model"))
			if scenario == "基线无效" {
				req.BaseCandidate.Content = `{}`
				req.BaseCandidate.Digest = agentdomain.SHA256Hex([]byte(`{}`))
				req.RevisionDirective.BaseDigest = req.BaseCandidate.Digest
			}
			registry, err := agentassets.Builtin()
			if err != nil {
				t.Fatal(err)
			}
			provider := &sequenceProvider{replies: []providerReply{{text: string(reply), inputTokens: 10, outputTokens: 30}, {err: context.DeadlineExceeded}}}
			if scenario == "严重问题" {
				provider.replies[1] = providerReply{text: `{"quality_status":"blocked","summary":"存在严重问题","findings":[{"id":"f1","category":"continuity","severity":"blocker","status":"open","source":"model_review","message":"修订违背作者约束","quote":"新标题"}]}`, inputTokens: 10, outputTokens: 30}
			}
			store := newMemoryStore()
			service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry}, fixedRunnerClock{}, &sequenceIDs{}, nil)
			run, err := service.CreateRun(context.Background(), req)
			if scenario == "基线无效" {
				if err == nil || len(provider.requests) != 0 {
					t.Fatal("无效基线启动了运行")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			completed, executeErr := service.ExecuteRun(context.Background(), run.ID, "head-1")
			err = executeErr
			if scenario == "严重问题" {
				if err != nil || completed.QualityStatus != agentdomain.QualityBlocked {
					t.Fatalf("未阻断: %v %v", completed.QualityStatus, err)
				}
				return
			}
			if err == nil {
				t.Fatal("无效输出或失败审阅被接受")
			}
			want := 2
			if scenario == "越界" {
				want = 1
			}
			if len(provider.requests) != want {
				t.Fatalf("calls=%d want=%d", len(provider.requests), want)
			}
			found := false
			for _, artifact := range store.artifacts {
				if strings.Contains(artifact.Content, "新标题") {
					found = true
				}
			}
			if !found {
				t.Fatal("失败时未保留修订工件")
			}
		})
	}
}
