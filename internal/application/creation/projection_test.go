package creation

import (
	"context"
	"testing"

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

func TestPeopleWorldProjectionUsesStableCharacterIDs(t *testing.T) {
	const payload = `{"characters":[{"id":"char-a","name":"无名","role":"protagonist","aliases":[],"summary":"甲","motivation":"守城","fear":"","secret":"","arc":"承担"},{"id":"char-b","name":"无名","role":"antagonist","aliases":[],"summary":"乙","motivation":"破城","fear":"","secret":"","arc":"醒悟"}],"relationships":[{"id":"rel-1","source_character_ref":"char-a","target_character_ref":"char-b","kind":"rival","label":"宿敌","strength":-80}],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路","scope_type":"project"}],"forbidden":[],"omission_reasons":["无需其他分类"]}`
	view := projectPeopleWorld(creationdomain.Candidate{ID: "people-1", PayloadJSON: payload})
	if len(view.Entities) < 2 || view.Entities[0].ID != "character:char-a" || view.Entities[1].ID != "character:char-b" {
		t.Fatalf("同名人物投影未使用稳定 ID: %+v", view.Entities)
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
