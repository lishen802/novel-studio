package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
)

func TestJobEventStoreAppendsAndQueriesBySequence(t *testing.T) {
	ctx := context.Background()
	store, project, _ := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(200, 0).UTC()

	events := []agentdomain.JobEvent{
		mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, 1, agentdomain.EventJobStarted, now),
		mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, 2, agentdomain.EventStepStarted, now.Add(time.Second)),
		mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, 3, agentdomain.EventStepCompleted, now.Add(2*time.Second)),
	}
	for _, event := range events {
		if err := store.AppendJobEvent(ctx, event); err != nil {
			t.Fatalf("AppendJobEvent seq %d: %v", event.SequenceNumber, err)
		}
	}

	got, err := store.JobEvents(ctx, "run-1", 0, 10)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(got))
	}
	for i, event := range got {
		if event.SequenceNumber != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d", i, event.SequenceNumber, i+1)
		}
		if event.JobKind != agentdomain.JobKindAgent || event.ProjectID != project.ID {
			t.Fatalf("event %d = %#v", i, event)
		}
	}
}

func TestJobEventStoreRejectsDuplicateSequence(t *testing.T) {
	ctx := context.Background()
	store, project, _ := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(200, 0).UTC()

	first := mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, 1, agentdomain.EventJobStarted, now)
	if err := store.AppendJobEvent(ctx, first); err != nil {
		t.Fatalf("AppendJobEvent first: %v", err)
	}
	dup := mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, 1, agentdomain.EventStepStarted, now.Add(time.Second))
	if err := store.AppendJobEvent(ctx, dup); !errors.Is(err, ports.ErrJobEventConflict) {
		t.Fatalf("AppendJobEvent duplicate = %v, want ErrJobEventConflict", err)
	}

	got, err := store.JobEvents(ctx, "run-1", 0, 10)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	if len(got) != 1 || got[0].EventType != agentdomain.EventJobStarted {
		t.Fatalf("events = %#v, want only the first job_started", got)
	}
}

func TestJobEventStoreFiltersByAfterSequence(t *testing.T) {
	ctx := context.Background()
	store, project, _ := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(200, 0).UTC()

	for seq := uint64(1); seq <= 3; seq++ {
		event := mustJobEvent(t, "run-1", agentdomain.JobKindAgent, project.ID, seq, agentdomain.EventStepDelta, now.Add(time.Duration(seq)*time.Second))
		if err := store.AppendJobEvent(ctx, event); err != nil {
			t.Fatalf("AppendJobEvent seq %d: %v", seq, err)
		}
	}

	got, err := store.JobEvents(ctx, "run-1", 1, 10)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(events) after seq 1 = %d, want 2", len(got))
	}
	if got[0].SequenceNumber != 2 || got[1].SequenceNumber != 3 {
		t.Fatalf("events = [%d, %d], want [2, 3]", got[0].SequenceNumber, got[1].SequenceNumber)
	}

	empty, err := store.JobEvents(ctx, "run-1", 3, 10)
	if err != nil {
		t.Fatalf("JobEvents after max: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("events after max = %d, want 0", len(empty))
	}
}

func TestJobEventStoreUpsertSnapshotProjectsAndUpdates(t *testing.T) {
	ctx := context.Background()
	store, project, _ := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(200, 0).UTC()

	snapshot, err := agentdomain.NewJobSnapshot(
		"run-1", agentdomain.JobKindAgent, project.ID, "running",
		3, 1, 1200, 800, "review_change_set", "", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertJobSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("UpsertJobSnapshot: %v", err)
	}

	got, err := store.JobSnapshot(ctx, "run-1")
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if got.MaxSequence != 3 || got.EarliestReplayableSequence != 1 ||
		got.Status != "running" || got.InputTokens != 1200 || got.OutputTokens != 800 {
		t.Fatalf("snapshot = %#v", got)
	}

	updated, err := agentdomain.NewJobSnapshot(
		"run-1", agentdomain.JobKindAgent, project.ID, "completed",
		5, 1, 1500, 1100, "", `{"change_set_id":"cs-1"}`, now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertJobSnapshot(ctx, updated); err != nil {
		t.Fatalf("UpsertJobSnapshot update: %v", err)
	}
	got, err = store.JobSnapshot(ctx, "run-1")
	if err != nil {
		t.Fatalf("JobSnapshot after update: %v", err)
	}
	if got.MaxSequence != 5 || got.Status != "completed" || got.TerminalInfo != `{"change_set_id":"cs-1"}` {
		t.Fatalf("updated snapshot = %#v", got)
	}
}

func TestJobSnapshotMissingReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	store, _, _ := createStoreWithChapter(t)
	defer store.Close()

	if _, err := store.JobSnapshot(ctx, "missing"); !errors.Is(err, ports.ErrJobSnapshotMissing) {
		t.Fatalf("JobSnapshot missing = %v, want ErrJobSnapshotMissing", err)
	}
}

func TestListRecoverableJobsUnionsAgentAndExtraction(t *testing.T) {
	ctx := context.Background()
	store, project, importJobID, chapterID := createExtractionTestStore(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()

	// Agent run: queued -> running.
	connection, err := agentdomain.NewProviderConnection(
		"provider-je", project.ID, "OpenAI", agentdomain.ProviderOpenAI,
		"https://api.openai.com/v1", "provider:provider-je", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertProviderConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	profile, err := agentdomain.NewAgentProfile(
		"profile-je", project.ID, agentdomain.RolePlanner, connection.ID,
		"gpt-test", 1024, 0.3, true, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertAgentProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	agentRun, err := agentdomain.NewChapterDraftRun(
		"run-recover", project.ID, chapterID, 1, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "step-1", Role: agentdomain.RolePlanner, ProfileID: profile.ID},
			{ID: "step-2", Role: agentdomain.RoleWriter, ProfileID: profile.ID},
			{ID: "step-3", Role: agentdomain.RoleContinuityReviewer, ProfileID: profile.ID},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgentRun(ctx, agentRun); err != nil {
		t.Fatal(err)
	}
	runningRun, err := agentRun.Transition(agentdomain.RunRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateAgentRun(ctx, runningRun); err != nil {
		t.Fatal(err)
	}

	// Extraction task: queued -> running.
	task, err := extractiondomain.NewTask(
		"task-recover", project.ID, importJobID,
		"connection-je", "gpt-4", "提取",
		[]extractiondomain.StepSpec{
			{ID: "step-1", ImportChapterID: "ic-1", CreatedChapterID: chapterID, ChapterTitle: "第一章", SourceStartLine: 1, SourceEndLine: 80, ChapterBody: "正文"},
			{ID: "step-2", ImportChapterID: "ic-2", CreatedChapterID: "chapter-2", ChapterTitle: "第二章", SourceStartLine: 81, SourceEndLine: 160, ChapterBody: "正文二"},
		}, 60000, 120000, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	runningTask, err := task.Transition(extractiondomain.StatusRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTask(ctx, runningTask); err != nil {
		t.Fatal(err)
	}

	jobs, err := store.ListRecoverableJobs(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRecoverableJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("recoverable jobs = %#v, want 2", jobs)
	}
	byID := make(map[string]agentdomain.JobSummary, len(jobs))
	for _, job := range jobs {
		byID[job.JobID] = job
	}
	if _, ok := byID["run-recover"]; !ok {
		t.Fatalf("missing agent run in recoverable jobs: %#v", byID)
	}
	if _, ok := byID["task-recover"]; !ok {
		t.Fatalf("missing extraction task in recoverable jobs: %#v", byID)
	}

	// Transition the agent run to completed; it should drop out of the listing.
	completedRun, err := runningRun.Transition(agentdomain.RunCompleted, "", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateAgentRun(ctx, completedRun); err != nil {
		t.Fatal(err)
	}
	jobs, err = store.ListRecoverableJobs(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListRecoverableJobs after completion: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "task-recover" {
		t.Fatalf("recoverable jobs after completion = %#v, want only task-recover", jobs)
	}
}

func mustJobEvent(
	t *testing.T,
	jobID string,
	kind agentdomain.JobKind,
	projectID string,
	sequence uint64,
	eventType agentdomain.JobEventType,
	now time.Time,
) agentdomain.JobEvent {
	t.Helper()
	event, err := agentdomain.NewJobEvent(jobID, kind, projectID, sequence, eventType, "step-1", `{"note":"ok"}`, now)
	if err != nil {
		t.Fatalf("NewJobEvent: %v", err)
	}
	return event
}
