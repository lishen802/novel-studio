package creation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestWorkbenchSelectsOnlyRequestedDirectionCard(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	seedDirectionsSelection(t, service, store, "d2")
	view, err := service.Workbench(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range view.DirectionSet.Cards {
		if card.Selected != (card.ID == "d2") {
			t.Fatalf("wrong selection: %+v", view.DirectionSet)
		}
	}
}

func TestWorkbenchReadDoesNotCreateSession(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	if _, err := service.Workbench(context.Background(), "project-1", "branch-1"); err != nil {
		t.Fatal(err)
	}
	if len(store.sessions) != 0 {
		t.Fatal("read created a session")
	}
}

func TestWorkbenchReadDoesNotRecoverInterruptedRun(t *testing.T) {
	service, store, kernel := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	session, _ = session.AttachRun("operation-1", "run-1", testClock{}.Now())
	if err := store.SaveRunSlot(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	kernel.runs["run-1"] = agentdomain.Run{ID: "run-1", Status: agentdomain.RunFailed}
	view, err := service.Workbench(context.Background(), session.ProjectID, session.BranchID)
	if err != nil {
		t.Fatal(err)
	}
	if view.CurrentRun.Status != string(agentdomain.RunFailed) {
		t.Fatalf("run = %+v", view.CurrentRun)
	}
	loaded, _ := store.Session(context.Background(), session.ProjectID)
	if loaded.ActiveRunID != session.ActiveRunID || loaded.BlockingReason != creationdomain.BlockNone {
		t.Fatalf("read changed session: %+v", loaded)
	}
}

func TestWorkbenchShowsLatestFailedRunAfterEarlierCandidate(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	seedDirectionsSelection(t, service, store, "d1")
	seedStageCandidate(t, service, store, creationdomain.StageProposal, proposalPayload)
	kernel.runs["run-proposal"] = agentdomain.Run{ID: "run-proposal", Status: agentdomain.RunCompleted}
	kernel.runs["run-failed"] = agentdomain.Run{ID: "run-failed", Status: agentdomain.RunFailed}
	store.operations["failed"] = creationdomain.Operation{
		ID: "failed", ProjectID: "project-1", BranchID: "branch-1", RunID: "run-failed",
		Status: creationdomain.OperationFailed, UpdatedAt: testClock{}.Now().Add(time.Minute),
	}
	view, err := service.Workbench(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if view.CurrentRun.RunID != "run-failed" || view.CurrentRun.Status != string(agentdomain.RunFailed) {
		t.Fatalf("不能把最近失败显示成旧候选的成功: %+v", view.CurrentRun)
	}
}

func TestRoadmapProjectionKeepsArcPlanningGoals(t *testing.T) {
	view := projectRoadmap(creationdomain.Candidate{ID: "roadmap-1", PayloadJSON: roadmapPayloadV3}, creationdomain.CandidateReview{})
	if view.Book.Title != "潮汐关" || len(view.Arcs) != 1 || view.Arcs[0].ID != "arc:arc-1" {
		t.Fatalf("路线图基础投影错误: %+v", view)
	}
	arc := view.Arcs[0]
	if arc.ReaderExpectation == "" || arc.StagePayoff == "" || arc.IrreversibleChange == "" ||
		arc.EndingContribution == "" || len(arc.Commitments) != 1 || len(arc.RetainedQuestions) != 1 {
		t.Fatalf("故事弧规划目标投影丢失: %+v", arc)
	}
	if arc.Commitments[0].ID != "cmt-1" || arc.Commitments[0].Detail == "" ||
		arc.RetainedQuestions[0].ID != "q-1" || arc.RetainedQuestions[0].Detail == "" {
		t.Fatalf("承诺或悬念引用丢失: %+v", arc)
	}
	legacy := projectRoadmap(creationdomain.Candidate{ID: "roadmap-legacy", PayloadJSON: roadmapPayload}, creationdomain.CandidateReview{})
	if len(legacy.Arcs) != 1 || legacy.Arcs[0].ReaderExpectation != "" || len(legacy.Arcs[0].Commitments) != 0 ||
		len(legacy.Arcs[0].RetainedQuestions) != 0 {
		t.Fatalf("旧 v2 不得伪造规划目标: %+v", legacy.Arcs)
	}
}

func TestPeopleWorldProjectionUsesStableCharacterIDs(t *testing.T) {
	const payload = `{"characters":[{"id":"char-a","name":"无名","role":"protagonist","aliases":[],"summary":"甲","motivation":"守城","fear":"","secret":"","arc":"承担"},{"id":"char-b","name":"无名","role":"antagonist","aliases":[],"summary":"乙","motivation":"破城","fear":"","secret":"","arc":"醒悟"}],"relationships":[{"id":"rel-1","source_character_ref":"char-a","target_character_ref":"char-b","kind":"rival","label":"宿敌","strength":-80}],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路","scope_type":"project"}],"forbidden":[],"omission_reasons":["无需其他分类"]}`
	view := projectPeopleWorld(creationdomain.Candidate{ID: "people-1", PayloadJSON: payload}, creationdomain.CandidateReview{})
	if len(view.Entities) < 2 || view.Entities[0].ID != "character:char-a" || view.Entities[1].ID != "character:char-b" {
		t.Fatalf("同名人物投影未使用稳定 ID: %+v", view.Entities)
	}
}

func TestProposalProjectionKeepsUnresolvedAnchorsReviewable(t *testing.T) {
	payload := strings.Replace(proposalPayload, `"scope_hint":"proposal"`, `"scope_hint":"结局"`, 1)
	candidate := creationdomain.Candidate{ID: "proposal-1", Kind: creationdomain.KindProposal, PayloadJSON: payload}
	view := projectProposal(candidate, creationdomain.CandidateReview{})
	if view.Title != "潮汐关" || len(view.Sections) != 7 {
		t.Fatalf("未解析锚点不应隐藏方案审阅: %+v", view)
	}
	if len(reviewRefs(candidate)[creationdomain.ReviewScopeSection]) != 7 {
		t.Fatal("未解析锚点不应让方案审阅单位消失")
	}
	if _, err := decodeBlueprint(payload); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("未解析锚点仍须阻断入档: %v", err)
	}
}

func TestRunProgressKeepsOnlyLatestAttemptAndAllVisibleSteps(t *testing.T) {
	run := agentdomain.Run{ID: "run", Status: agentdomain.RunRunning, Steps: []agentdomain.RunStep{
		{NodeID: "plan", PublicStageID: agentdomain.PublicStageStoryPlanning, Status: agentdomain.StepCompleted},
		{NodeID: "review", PublicStageID: agentdomain.PublicStageIndependentReview, Status: agentdomain.StepFailed},
		{NodeID: "review", PublicStageID: agentdomain.PublicStageIndependentReview, Status: agentdomain.StepRunning},
	}}
	view := projectCreationRun(run, true)
	if len(view.Steps) != 2 || view.Steps[0].Status != "completed" || view.Steps[1].Status != "running" {
		t.Fatalf("步骤或重试投影错误: %+v", view.Steps)
	}
	if view.PublicStage != "independent_review" {
		t.Fatalf("当前阶段错误: %s", view.PublicStage)
	}
}
