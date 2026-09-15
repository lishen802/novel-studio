package creation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestSaveAuthorEntryKeepsMaterialAndRequiresFreshSession(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	request := SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, ExpectedRunSlotVersion: session.RunSlotVersion, Kind: creationdomain.EntryAuthorPlan, Material: "  原始方案😀\n结局已确定。  "}
	saved, err := service.SaveEntry(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Material != request.Material {
		t.Fatal("材料被裁剪")
	}
	if _, err := service.SaveEntry(context.Background(), request); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("过期页面: %v", err)
	}
	view, err := service.Workbench(context.Background(), session.ProjectID, session.BranchID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Entry.Kind != creationdomain.EntryAuthorPlan || view.Entry.Material != request.Material {
		t.Fatalf("来源未恢复: %+v", view.Entry)
	}
	for _, stop := range view.StageRail {
		if stop.ID == creationdomain.StageDirections && stop.Status != "not_applicable" {
			t.Fatal("跳过方向不应标为接受")
		}
	}
	candidates, _ := store.ListCandidates(context.Background(), session.ID)
	if len(candidates) != 0 {
		t.Fatal("保存原文不能伪造方向或标准候选")
	}
}

func TestAuthorMaterialReusesAllStagesAndArchivesWithoutDirection(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	store := NewSQLiteStore(raw)
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.EnsureSession(ctx, project.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: project.ID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, ExpectedRunSlotVersion: session.RunSlotVersion, Kind: creationdomain.EntryAuthorPlan, Material: "作者完整方案，包括人物世界与三章路线图。"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.loadSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	output := materialFixture(t, entry)
	encoded, _ := json.Marshal(output)
	analysis, err := creationdomain.NewCandidate("analysis", session.ID, creationdomain.StageIntent, creationdomain.KindProposal, 1, "", "analysis-run", "analysis-artifact", string(encoded), "author_material.schema", 1, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	analysis = analysis.WithUpstream(liveUpstreamDigest(session, nil, creationdomain.StageIntent)).WithQuality(creationdomain.QualityPassed)
	if err := store.SaveCandidate(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AdvanceStage(ctx, project.ID); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []creationdomain.Stage{creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		session, err = service.loadSession(ctx, project.ID)
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := service.UseMaterialCandidate(ctx, UseMaterialRequest{ProjectID: project.ID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, AnalysisID: analysis.ID, Stage: stage})
		if err != nil {
			t.Fatalf("reuse %s: %v", stage, err)
		}
		if candidate.Status == creationdomain.StatusConfirmed || candidate.ParentCandidateID != analysis.ID {
			t.Fatal("复用不能隐式接受，须保留来源")
		}
		if _, err := service.AdvanceStage(ctx, project.ID); !errors.Is(err, creationdomain.ErrCandidateAcceptanceRequired) {
			t.Fatalf("未接受推进: %v", err)
		}
		if _, err := service.AcceptCandidate(ctx, project.ID, candidate.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := service.AdvanceStage(ctx, project.ID); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "author-archive"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ChangeSetID == "" || result.Session.SelectedDirectionCandidateID != "" {
		t.Fatal("不得伪造方向")
	}
	candidates, _ := store.ListCandidates(ctx, session.ID)
	for _, candidate := range candidates {
		if candidate.Stage == creationdomain.StageDirections {
			t.Fatal("已有方案不应生成方向")
		}
	}
}

func materialFixture(t *testing.T, entry creationdomain.EntryRevision) agentdomain.MaterialAnalysisOutput {
	t.Helper()
	var proposal agentdomain.StoryBlueprintOutput
	var cast agentdomain.StoryCastOutput
	var roadmap agentdomain.StoryRoadmapOutput
	if err := json.Unmarshal([]byte(proposalPayload), &proposal); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(peopleWorldPayload), &cast); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(roadmapPayload), &roadmap); err != nil {
		t.Fatal(err)
	}
	output := agentdomain.MaterialAnalysisOutput{EntryID: entry.ID, SourceHash: entry.MaterialSHA256, Proposal: &proposal, PeopleWorld: &cast, Roadmap: &roadmap, Gaps: []agentdomain.MaterialGap{}, Sources: []agentdomain.MaterialFieldSource{}}
	for _, stage := range []creationdomain.Stage{creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		payload, _ := materialPayload(output, stage)
		var fields map[string]any
		_ = json.Unmarshal([]byte(payload), &fields)
		for key := range fields {
			output.Sources = append(output.Sources, agentdomain.MaterialFieldSource{Stage: string(stage), FieldPath: "/" + key, Quote: entry.Material})
		}
	}
	return output
}

func TestMaterialAnalysisRejectsWrongSourceAndStaleReuse(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	ctx := context.Background()
	session := seedIntentSession(t, service)
	entry, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, Kind: creationdomain.EntryAuthorPlan, Material: "作者原始材料😀"})
	if err != nil {
		t.Fatal(err)
	}
	output := materialFixture(t, entry)
	output.Sources[0].Quote = "原文不存在"
	payload, _ := json.Marshal(output)
	if _, err := decodeMaterialForEntry(&entry, string(payload)); !errors.Is(err, creationdomain.ErrInvalidSourceSpan) {
		t.Fatalf("伪造引用: %v", err)
	}
	session, _ = service.loadSession(ctx, session.ProjectID)
	good := materialFixture(t, entry)
	payload, _ = json.Marshal(good)
	analysis, _ := creationdomain.NewCandidate("old-analysis", session.ID, creationdomain.StageIntent, creationdomain.KindProposal, 1, "", "run", "artifact", string(payload), "author_material.schema", 1, testClock{}.Now())
	analysis = analysis.WithUpstream(liveUpstreamDigest(session, nil, creationdomain.StageIntent))
	if err := store.SaveCandidate(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, ExpectedRunSlotVersion: session.RunSlotVersion, Kind: creationdomain.EntryAuthorPlan, Material: "修改后的方案"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AdvanceStage(ctx, session.ProjectID); !errors.Is(err, creationdomain.ErrImpactUnresolved) {
		t.Fatalf("旧分析不得解锁: %v", err)
	}
}

func TestAuthorMaterialPaidRunFreezesSourceAndRecoversCandidate(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	entry, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, Kind: creationdomain.EntryAuthorPlan, Material: "这一版作者来源必须完整传入模型。"})
	if err != nil {
		t.Fatal(err)
	}
	fixture := materialFixture(t, entry)
	var road agentdomain.StoryRoadmapOutput
	if err := json.Unmarshal([]byte(roadmapPayloadV3), &road); err != nil {
		t.Fatal(err)
	}
	fixture.Roadmap = &road
	raw, _ := json.Marshal(fixture)
	kernel.payloads["story.author_material"] = string(raw)
	request := generateRequest(creationdomain.StageIntent)
	request.Profile.MaxRunTokens = 30000
	preview, err := service.PreviewGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if preview.WorkflowID != "story.author_material" {
		t.Fatal("错误工作流")
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "material-paid"
	run, err := service.ConfirmGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		operation, err := store.OperationByIdempotency(ctx, session.ProjectID, session.BranchID, "generate:intent", request.IdempotencyKey)
		return err == nil && operation.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("解析未完成")
	}
	kernel.mu.Lock()
	created := kernel.lastCreate
	calls := kernel.executeN
	kernel.mu.Unlock()
	found := false
	for _, source := range created.Sources {
		if source.ID == "author_material" && strings.Contains(source.Content, entry.Material) && strings.Contains(source.Content, entry.MaterialSHA256) {
			found = true
		}
		if strings.HasPrefix(source.ID, "direction:") {
			t.Fatal("不应伪造方向来源")
		}
	}
	if !found {
		t.Fatal("模型缺少冻结作者原文")
	}
	restarted, err := New(Config{Store: store, Kernel: kernel, Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Recover(ctx, session.ProjectID); err != nil {
		t.Fatal(err)
	}
	replay, err := restarted.ConfirmGenerate(ctx, request)
	if err != nil || replay.RunID != run.RunID {
		t.Fatalf("重复请求: %v", err)
	}
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	if kernel.executeN != calls {
		t.Fatal("恢复重复计费")
	}
}

func TestAuthorMaterialConflictCannotBeResolvedByAcceptingCandidate(t *testing.T) {
	service, _, _ := newTestService(t, newFakeKernel(), nil)
	ctx := context.Background()
	session := seedIntentSession(t, service)
	entry, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, Kind: creationdomain.EntryAuthorPlan, Material: "作者关于结局的冲突描述。"})
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.loadSession(ctx, session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"required", "conflict"} {
		t.Run(kind, func(t *testing.T) {
			output := materialFixture(t, entry)
			output.Gaps = []agentdomain.MaterialGap{{ID: "ending", Stage: "proposal", FieldPath: "/ending", Kind: kind, Reason: "结局需要作者确认", Quote: entry.Material, SuggestedAction: "核对结局"}}
			raw, _ := json.Marshal(output)
			analysis, err := creationdomain.NewCandidate("analysis", session.ID, creationdomain.StageIntent, creationdomain.KindProposal, 1, "", "run", "artifact", string(raw), "author_material.schema", 1, testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			analysis = analysis.WithUpstream(liveUpstreamDigest(session, nil, creationdomain.StageIntent)).WithQuality(creationdomain.QualityPassed)
			candidate, err := creationdomain.NewCandidate("proposal", session.ID, creationdomain.StageProposal, creationdomain.KindProposal, 1, "analysis", "", "artifact", proposalPayload, "story_blueprint.schema", 2, testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			candidate = candidate.WithUpstream(liveUpstreamDigest(session, []creationdomain.Candidate{analysis}, creationdomain.StageProposal)).WithQuality(creationdomain.QualityPassed)
			candidate.Status = creationdomain.StatusConfirmed
			err = validateMaterialGapsForArchive(session, []creationdomain.Candidate{analysis, candidate})
			if kind == "conflict" && !errors.Is(err, creationdomain.ErrInvalidGapReport) {
				t.Fatalf("接受候选不能消除来源冲突: %v", err)
			}
			if kind == "required" && err != nil {
				t.Fatalf("有效已接受候选应解决必填缺口: %v", err)
			}
		})
	}
}

func TestMaterialV2RequiresPlanningAndPreservesEmptyLists(t *testing.T) {
	ctx := context.Background()
	service, _, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	entry, err := service.SaveEntry(ctx, SaveEntryRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, Kind: creationdomain.EntryAuthorPlan, Material: "作者完整方案原文"})
	if err != nil {
		t.Fatal(err)
	}
	fixture := materialFixture(t, entry)
	raw, _ := json.Marshal(fixture)
	if _, err := decodeMaterialForEntry(&entry, string(raw), 2); err == nil {
		t.Fatal("新解析不应把缺少目标的旧路线图当完整方案")
	}
	if _, err := decodeMaterialForEntry(&entry, string(raw), 1); err != nil {
		t.Fatalf("旧解析应仍可读取: %v", err)
	}
	var road agentdomain.StoryRoadmapOutput
	if err := json.Unmarshal([]byte(roadmapPayloadV3), &road); err != nil {
		t.Fatal(err)
	}
	road.Arcs[0].Commitments = nil
	road.Arcs[0].CommitmentsOmission = "本弧只回收既有承诺"
	road.Arcs[0].RetainedQuestions = nil
	road.Arcs[0].RetainedQuestionsOmission = "本弧独立收束"
	fixture.Roadmap = &road
	raw, _ = json.Marshal(fixture)
	output, err := decodeMaterialForEntry(&entry, string(raw), 2)
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := materialPayload(output, creationdomain.StageRoadmap)
	if !ok || !strings.Contains(payload, `"commitments":[]`) || !strings.Contains(payload, `"retained_questions":[]`) {
		t.Fatal("复用丢失合法空数组")
	}
}
