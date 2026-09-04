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
