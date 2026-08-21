package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

// completedAgentRunWithEvents runs an agent run to completion and returns the
// run ID. It is a shared helper for cursor-query tests that need a fully
// populated event feed.
func completedAgentRunWithEvents(
	t *testing.T,
	service *Service,
	chapter ChapterView,
	prompt string,
) string {
	t.Helper()
	ctx := context.Background()
	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, prompt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "completed", 5*time.Second)
	return run.ID
}

// TestExecuteAgentRunAppendsContiguousJobEvents asserts that ExecuteAgentRun
// emits an append-only JobEvent feed with a per-job monotonic sequence covering
// the full lifecycle: job_started, then step_started/step_completed for each of
// the three authoring steps, then job_completed. The authoritative snapshot
// must reflect the completed status and the highest sequence.
func TestExecuteAgentRunAppendsContiguousJobEvents(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "completed", 5*time.Second)

	agentStore, ok := service.active.store.(ports.AgentStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.AgentStore", service.active.store)
	}
	events, err := agentStore.JobEvents(ctx, run.ID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}

	wantTypes := []agentdomain.JobEventType{
		agentdomain.EventJobQueued,
		agentdomain.EventJobStarted,
		// Planner step.
		agentdomain.EventStepStarted,
		agentdomain.EventStepCompleted,
		// Writer step.
		agentdomain.EventStepStarted,
		agentdomain.EventStepCompleted,
		// Continuity reviewer step.
		agentdomain.EventStepStarted,
		agentdomain.EventStepCompleted,
		// Terminal.
		agentdomain.EventJobCompleted,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("len(events) = %d, want %d (%#v)", len(events), len(wantTypes), events)
	}
	for i, want := range wantTypes {
		if events[i].SequenceNumber != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d", i, events[i].SequenceNumber, i+1)
		}
		if events[i].EventType != want {
			t.Fatalf("event %d type = %q, want %q", i, events[i].EventType, want)
		}
		if events[i].JobKind != agentdomain.JobKindAgent || events[i].ProjectID == "" {
			t.Fatalf("event %d missing kind/project: %#v", i, events[i])
		}
	}

	snapshot, err := agentStore.JobSnapshot(ctx, run.ID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != string(agentdomain.RunCompleted) {
		t.Fatalf("snapshot status = %q, want completed", snapshot.Status)
	}
	if snapshot.MaxSequence != uint64(len(wantTypes)) {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, len(wantTypes))
	}
	if snapshot.EarliestReplayableSequence != 1 {
		t.Fatalf("snapshot earliest_replayable_sequence = %d, want 1", snapshot.EarliestReplayableSequence)
	}
	if snapshot.InputTokens != 30 || snapshot.OutputTokens != 15 {
		t.Fatalf("snapshot tokens = in %d out %d, want 30/15", snapshot.InputTokens, snapshot.OutputTokens)
	}
}

// TestCreatedQueuedAgentRunHasReplayableSnapshot protects project-open
// recovery: ListRecoverableJobs includes queued runs, so creation must already
// have persisted a sequence-1 snapshot before execution starts.
func TestCreatedQueuedAgentRunHasReplayableSnapshot(t *testing.T) {
	ctx := context.Background()
	service, chapter := configuredAgentService(t, &fakeModelProviderFactory{})
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "等待作者执行。")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.GetJobSnapshot(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetJobSnapshot for queued run: %v", err)
	}
	if snapshot.Status != "queued" || snapshot.MaxSequence != 1 {
		t.Fatalf("queued snapshot = %#v, want status queued and sequence 1", snapshot)
	}
	page, err := service.GetJobEvents(ctx, run.ID, 0, 10)
	if err != nil {
		t.Fatalf("GetJobEvents for queued run: %v", err)
	}
	if len(page.Events) != 1 || page.Events[0].EventType != string(agentdomain.EventJobQueued) {
		t.Fatalf("queued events = %#v, want one job_queued event", page.Events)
	}
}

// TestFailExtractionStartCompensatesRunningTask ensures the post-running
// startup failure path cannot strand a task without a background pipeline.
func TestFailExtractionStartCompensatesRunningTask(t *testing.T) {
	ctx := context.Background()
	service, taskID, _ := configuredExtractionService(t, &fakeModelProviderFactory{})
	defer service.Close()

	store, ok := service.active.store.(ports.ExtractionStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.ExtractionStore", service.active.store)
	}
	task, err := store.Task(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	task, err = task.Transition("running", "", service.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	startedDraft, err := newExtractionJobDraft(task, agentdomain.EventJobStarted, "", "", service.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTask(ctx, task, startedDraft); err != nil {
		t.Fatal(err)
	}

	_, err = service.failExtractionStart(ctx, store, task, errors.New("import lifecycle write failed"), service.active.generation)
	if err == nil {
		t.Fatal("failExtractionStart returned nil error")
	}
	persisted, err := store.Task(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != "failed" || persisted.ErrorCode != "extraction_start_state_failed" {
		t.Fatalf("compensated task = %#v", persisted)
	}
	events, err := store.JobEvents(ctx, taskID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 3 || events[len(events)-1].EventType != agentdomain.EventJobFailed {
		t.Fatalf("compensation events = %#v", events)
	}
}

// TestExecuteAgentRunFailureAppendsJobFailedEvent asserts that a provider
// failure during ExecuteAgentRun emits a job_failed event rather than leaving
// the feed at job_started with no terminal marker.
func TestExecuteAgentRunFailureAppendsJobFailedEvent(t *testing.T) {
	ctx := context.Background()
	service, chapter := configuredAgentService(t, failingModelProviderFactory{})
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}
	// ExecuteAgentRun returns immediately with a running view; the provider
	// failure is persisted asynchronously by the background goroutine.
	if _, err := service.ExecuteAgentRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "failed", 5*time.Second)

	agentStore, ok := service.active.store.(ports.AgentStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.AgentStore", service.active.store)
	}
	events, err := agentStore.JobEvents(ctx, run.ID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least job_started and job_failed events")
	}
	last := events[len(events)-1]
	if last.EventType != agentdomain.EventJobFailed {
		t.Fatalf("last event type = %q, want job_failed", last.EventType)
	}
	if events[0].EventType != agentdomain.EventJobQueued {
		t.Fatalf("first event type = %q, want job_queued", events[0].EventType)
	}

	snapshot, err := agentStore.JobSnapshot(ctx, run.ID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != string(agentdomain.RunFailed) {
		t.Fatalf("snapshot status = %q, want failed", snapshot.Status)
	}
	if snapshot.MaxSequence != uint64(len(events)) {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, len(events))
	}
}

// configuredExtractionService wires a service with a provider connection and an
// applied import job containing one chapter, returning the task created by
// StartExtractionRun. It mirrors configuredAgentService for the extraction
// flow so event-feed tests can focus on the JobEvent sequence.
func configuredExtractionService(
	t *testing.T,
	providerFactory ports.ModelProviderFactory,
) (*Service, string, string) {
	t.Helper()
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(300, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "墨老夺舍")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	importJob, importChapters := seedAppliedImportJob(t, store, project.ID, "import-job-evt", "import-chapter-evt", "第一章", "宗门暗门后藏有名册。")
	applied, err := service.ApplyImportJob(ctx, importJob.ID, []string{importChapters[0].ID})
	if err != nil {
		t.Fatalf("apply import job: %v", err)
	}
	if applied.Job.Status != "applied" {
		t.Fatalf("apply result = %#v", applied)
	}
	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  importJob.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
		Instruction:  "抽取故事档案候选。",
	})
	if err != nil {
		t.Fatalf("start extraction run: %v", err)
	}
	return service, run.Task.ID, project.ID
}

const extractionEventModelOutput = `{
	"outline": [],
	"characters": [
		{"name":"林默","aliases":[],"role":"protagonist","summary":"沉默的修仙者","motivation":"夺回肉身","status":"active"}
	],
	"relationships": [],
	"worldRules": [],
	"storyEvents": [],
	"facts": [],
	"foreshadowingHooks": []
}`

// TestExecuteExtractionRunAppendsContiguousJobEvents asserts that
// ExecuteExtractionRun emits an append-only JobEvent feed with a per-job
// monotonic sequence covering the full lifecycle: job_started, step_started,
// step_completed, then job_completed. The authoritative snapshot must reflect
// the completed status and the highest sequence.
func TestExecuteExtractionRunAppendsContiguousJobEvents(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{"extraction-model": extractionEventModelOutput},
	}
	service, taskID, _ := configuredExtractionService(t, providerFactory)
	defer service.Close()

	if _, err := service.ExecuteExtractionRun(ctx, taskID); err != nil {
		t.Fatalf("ExecuteExtractionRun: %v", err)
	}
	// ExecuteExtractionRun returns a running view immediately; the contiguous
	// event feed is appended by the background goroutine, so wait for the
	// terminal status before asserting the full event sequence.
	waitForExtractionTaskStatus(t, service, taskID, "completed", 5*time.Second)

	store, ok := service.active.store.(ports.ExtractionStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.ExtractionStore", service.active.store)
	}
	events, err := store.JobEvents(ctx, taskID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}

	wantTypes := []agentdomain.JobEventType{
		agentdomain.EventJobQueued,
		agentdomain.EventJobStarted,
		agentdomain.EventStepStarted,
		agentdomain.EventStepCompleted,
		agentdomain.EventJobCompleted,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("len(events) = %d, want %d (%#v)", len(events), len(wantTypes), events)
	}
	for i, want := range wantTypes {
		if events[i].SequenceNumber != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d", i, events[i].SequenceNumber, i+1)
		}
		if events[i].EventType != want {
			t.Fatalf("event %d type = %q, want %q", i, events[i].EventType, want)
		}
		if events[i].JobKind != agentdomain.JobKindExtraction || events[i].ProjectID == "" {
			t.Fatalf("event %d missing kind/project: %#v", i, events[i])
		}
	}

	snapshot, err := store.JobSnapshot(ctx, taskID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != "completed" {
		t.Fatalf("snapshot status = %q, want completed", snapshot.Status)
	}
	if snapshot.MaxSequence != uint64(len(wantTypes)) {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, len(wantTypes))
	}
	if snapshot.JobKind != agentdomain.JobKindExtraction {
		t.Fatalf("snapshot job_kind = %q, want extraction", snapshot.JobKind)
	}
	if snapshot.InputTokens != 10 || snapshot.OutputTokens != 5 {
		t.Fatalf("snapshot tokens = in %d out %d, want 10/5", snapshot.InputTokens, snapshot.OutputTokens)
	}
}

// TestExecuteExtractionRunFailureAppendsJobFailedEvent asserts that a provider
// failure during ExecuteExtractionRun emits a step_failed then job_failed event
// rather than leaving the feed at job_started with no terminal marker.
func TestExecuteExtractionRunFailureAppendsJobFailedEvent(t *testing.T) {
	ctx := context.Background()
	service, taskID, _ := configuredExtractionService(t, failingModelProviderFactory{})
	defer service.Close()

	if _, err := service.ExecuteExtractionRun(ctx, taskID); err != nil {
		t.Fatalf("ExecuteExtractionRun: %v", err)
	}
	// The provider failure is persisted by the background goroutine, so wait
	// for the failed terminal state before asserting the job_failed event.
	waitForExtractionTaskStatus(t, service, taskID, "failed", 5*time.Second)

	store, ok := service.active.store.(ports.ExtractionStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.ExtractionStore", service.active.store)
	}
	events, err := store.JobEvents(ctx, taskID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	if len(events) < 2 {
		t.Fatal("expected at least job_started and job_failed events")
	}
	if events[0].EventType != agentdomain.EventJobQueued {
		t.Fatalf("first event type = %q, want job_queued", events[0].EventType)
	}
	last := events[len(events)-1]
	if last.EventType != agentdomain.EventJobFailed {
		t.Fatalf("last event type = %q, want job_failed", last.EventType)
	}

	snapshot, err := store.JobSnapshot(ctx, taskID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != "failed" {
		t.Fatalf("snapshot status = %q, want failed", snapshot.Status)
	}
	if snapshot.MaxSequence != uint64(len(events)) {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, len(events))
	}
}

// TestGetJobSnapshotReturnsAuthoritativeProjection asserts that GetJobSnapshot
// returns the status, max_sequence, earliest_replayable_sequence, and cumulative
// token usage from the authoritative projection, not from a full event replay.
func TestGetJobSnapshotReturnsAuthoritativeProjection(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()

	runID := completedAgentRunWithEvents(t, service, chapter, "续写这一章。")

	view, err := service.GetJobSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("GetJobSnapshot: %v", err)
	}
	if view.JobID != runID {
		t.Fatalf("snapshot jobID = %q, want %q", view.JobID, runID)
	}
	if view.JobKind != "agent" {
		t.Fatalf("snapshot job_kind = %q, want agent", view.JobKind)
	}
	if view.Status != "completed" {
		t.Fatalf("snapshot status = %q, want completed", view.Status)
	}
	// 9 events: job_queued + job_started + 3×(step_started+step_completed) + job_completed.
	if view.MaxSequence != 9 {
		t.Fatalf("snapshot max_sequence = %d, want 9", view.MaxSequence)
	}
	if view.EarliestReplayableSequence != 1 {
		t.Fatalf("snapshot earliest_replayable_sequence = %d, want 1", view.EarliestReplayableSequence)
	}
	if view.InputTokens != 30 || view.OutputTokens != 15 {
		t.Fatalf("snapshot tokens = in %d out %d, want 30/15", view.InputTokens, view.OutputTokens)
	}
	if view.PublicStageID != string(agentdomain.PublicStageCandidateReady) || view.CompletedSteps != 3 || view.TotalSteps != 3 {
		t.Fatalf("public projection = stage %q progress %d/%d", view.PublicStageID, view.CompletedSteps, view.TotalSteps)
	}
}

// TestGetJobEventsPaginatesByCursor asserts that GetJobEvents returns events in
// ascending sequence order with correct paging metadata: the first limited page
// has hasMore=true and nextCursor at the last event's sequence, and the
// subsequent page returns the remaining events with hasMore=false.
func TestGetJobEventsPaginatesByCursor(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()

	runID := completedAgentRunWithEvents(t, service, chapter, "续写这一章。")

	// Page 1: first 2 events from the beginning.
	page, err := service.GetJobEvents(ctx, runID, 0, 2)
	if err != nil {
		t.Fatalf("GetJobEvents page 1: %v", err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("page 1 len(events) = %d, want 2", len(page.Events))
	}
	if page.Events[0].SequenceNumber != 1 || page.Events[1].SequenceNumber != 2 {
		t.Fatalf("page 1 sequences = %d, %d, want 1, 2",
			page.Events[0].SequenceNumber, page.Events[1].SequenceNumber)
	}
	if !page.HasMore {
		t.Fatal("page 1 hasMore = false, want true")
	}
	if page.NextCursor != 2 {
		t.Fatalf("page 1 nextCursor = %d, want 2", page.NextCursor)
	}
	if page.SnapshotRequired {
		t.Fatal("page 1 snapshotRequired = true, want false")
	}

	// Page 2: remaining 7 events after sequence 2.
	page, err = service.GetJobEvents(ctx, runID, 2, 100)
	if err != nil {
		t.Fatalf("GetJobEvents page 2: %v", err)
	}
	if len(page.Events) != 7 {
		t.Fatalf("page 2 len(events) = %d, want 7", len(page.Events))
	}
	if page.Events[0].SequenceNumber != 3 {
		t.Fatalf("page 2 first sequence = %d, want 3", page.Events[0].SequenceNumber)
	}
	if page.Events[6].SequenceNumber != 9 {
		t.Fatalf("page 2 last sequence = %d, want 9", page.Events[6].SequenceNumber)
	}
	if page.HasMore {
		t.Fatal("page 2 hasMore = true, want false")
	}
	if page.SnapshotRequired {
		t.Fatal("page 2 snapshotRequired = true, want false")
	}
	if page.NextCursor != 9 {
		t.Fatalf("page 2 nextCursor = %d, want 9", page.NextCursor)
	}
}

// TestGetJobEventsSignalsSnapshotRequired asserts that when the cursor is
// earlier than the snapshot's earliest_replayable_sequence (simulating event
// cleanup), GetJobEvents returns an empty events list with snapshotRequired=true
// so the frontend rebuilds from GetJobSnapshot.
func TestGetJobEventsSignalsSnapshotRequired(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上。",
			"reviewer": `{"approved":true,"summary":"ok","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()

	runID := completedAgentRunWithEvents(t, service, chapter, "续写这一章。")

	// Advance earliest_replayable_sequence to 5, simulating event cleanup.
	store, ok := service.active.store.(ports.JobEventStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.JobEventStore", service.active.store)
	}
	snapshot, err := store.JobSnapshot(ctx, runID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	snapshot.EarliestReplayableSequence = 5
	if err := store.UpsertJobSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("UpsertJobSnapshot: %v", err)
	}

	// Cursor 3 < earliest 5 → snapshotRequired=true, empty events.
	page, err := service.GetJobEvents(ctx, runID, 3, 10)
	if err != nil {
		t.Fatalf("GetJobEvents: %v", err)
	}
	if !page.SnapshotRequired {
		t.Fatal("snapshotRequired = false, want true")
	}
	if len(page.Events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(page.Events))
	}
	if page.HasMore {
		t.Fatal("hasMore = true, want false")
	}

	// Cursor at the lower bound returns events normally.
	page, err = service.GetJobEvents(ctx, runID, 5, 10)
	if err != nil {
		t.Fatalf("GetJobEvents at lower bound: %v", err)
	}
	if page.SnapshotRequired {
		t.Fatal("snapshotRequired = true at lower bound, want false")
	}
	if len(page.Events) == 0 {
		t.Fatal("expected events at lower bound")
	}
}

// TestRecoverInterruptedAgentRunsAppendsJobRecoveredEvent asserts that when a
// project is reopened after an in-flight agent run was left in the Running
// state (simulating an app restart or Bridge remount), RecoverInterruptedAgentRuns
// appends a job_recovered event with the next monotonic sequence and refreshes
// the snapshot to Recoverable in the same transaction. The event feed must end
// at job_recovered so the UI can replay the recovery on remount, and
// ListRecoverableJobs must surface the job for cursor rebuild. This also
// proves the stale-drain safety net: the background goroutine skips
// persistence on the closed store, and recovery writes job_recovered to the
// newly reopened store.
func TestRecoverInterruptedAgentRunsAppendsJobRecoveredEvent(t *testing.T) {
	ctx := context.Background()
	factory := newStickyModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	defer service.Close()
	// A zero drain timeout causes replaceActive to mark remaining leases stale
	// immediately, simulating a drain that cannot wait for the stuck provider.
	service.drainTimeout = 0
	projectPath := service.active.packageInfo.Root

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	// Wait until the sticky provider has started, proving Phase 1 committed
	// job_started and the goroutine is blocked inside Model IO.
	waitForStickyProviderStarted(t, factory)

	// Switch to a new project. replaceActive drains (timeout=0 → stale), closes
	// the old store, and installs a new generation. The goroutine must NOT write
	// its step result to the now-closed store.
	if _, err := service.CreateProject(ctx, t.TempDir(), "新项目", "奇幻", "简介"); err != nil {
		t.Fatal(err)
	}
	// Release the stuck provider. The job detects the stale lease/generation and
	// skips persistence (failStale writes nothing).
	close(factory.release)

	// Reopen the original project. RecoverInterruptedAgentRuns transitions the
	// stuck Running run to Recoverable and appends job_recovered to the event
	// feed in the same transaction.
	if _, err := service.OpenProject(ctx, projectPath); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}

	agentStore, ok := service.active.store.(ports.AgentStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.AgentStore", service.active.store)
	}
	events, err := agentStore.JobEvents(ctx, run.ID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	// The goroutine persisted job_queued (seq 1), job_started and step_started
	// for the planner before blocking in the provider. Recovery appends
	// job_recovered as the next sequence. The feed must be contiguous and
	// end at job_recovered.
	if len(events) < 2 {
		t.Fatalf("len(events) = %d, want at least 2: %#v", len(events), events)
	}
	if events[0].EventType != agentdomain.EventJobQueued || events[0].SequenceNumber != 1 {
		t.Fatalf("event 0 = %#v, want job_queued/seq 1", events[0])
	}
	for i, evt := range events {
		if evt.SequenceNumber != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d (non-contiguous feed)", i, evt.SequenceNumber, i+1)
		}
		if evt.JobKind != agentdomain.JobKindAgent || evt.ProjectID == "" {
			t.Fatalf("event %d missing kind/project: %#v", i, evt)
		}
	}
	last := events[len(events)-1]
	if last.EventType != agentdomain.EventJobRecovered {
		t.Fatalf("last event type = %q, want job_recovered", last.EventType)
	}

	snapshot, err := agentStore.JobSnapshot(ctx, run.ID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != "recoverable" {
		t.Fatalf("snapshot status = %q, want recoverable", snapshot.Status)
	}
	if snapshot.MaxSequence != last.SequenceNumber {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, last.SequenceNumber)
	}
	if snapshot.EarliestReplayableSequence != 1 {
		t.Fatalf("snapshot earliest_replayable_sequence = %d, want 1",
			snapshot.EarliestReplayableSequence)
	}

	// ListRecoverableJobs must surface the recovered run for cursor rebuild.
	jobs, err := service.ListRecoverableJobs(ctx)
	if err != nil {
		t.Fatalf("ListRecoverableJobs: %v", err)
	}
	found := false
	for _, job := range jobs {
		if job.JobID == run.ID {
			if job.JobKind != "agent" {
				t.Fatalf("recovered job kind = %q, want agent", job.JobKind)
			}
			if job.Status != "recoverable" {
				t.Fatalf("recovered job status = %q, want recoverable", job.Status)
			}
			if job.MaxSequence != last.SequenceNumber {
				t.Fatalf("recovered job max_sequence = %d, want %d", job.MaxSequence, last.SequenceNumber)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("recovered run not found in ListRecoverableJobs")
	}

	// The cursor rebuild at 0 must see snapshotRequired=false (earliest is still
	// 1) and return the full feed so the UI can replay the recovery.
	page, err := service.GetJobEvents(ctx, run.ID, 0, 100)
	if err != nil {
		t.Fatalf("GetJobEvents: %v", err)
	}
	if page.SnapshotRequired {
		t.Fatal("snapshotRequired = true, want false (earliest is still 1)")
	}
	if len(page.Events) != len(events) {
		t.Fatalf("GetJobEvents returned %d events, want %d", len(page.Events), len(events))
	}
}

// TestRecoverInterruptedTasksAppendsJobRecoveredEvent mirrors
// TestRecoverInterruptedAgentRunsAppendsJobRecoveredEvent for the extraction
// pipeline: a task left Running across a project reopen must get a
// job_recovered event and a Recoverable snapshot so the extraction workbench
// can rebuild its cursor on remount.
func TestRecoverInterruptedTasksAppendsJobRecoveredEvent(t *testing.T) {
	ctx := context.Background()
	factory := newStickyModelProviderFactory(map[string]string{
		"extraction-model": extractionEventModelOutput,
	})
	service, taskID, _ := configuredExtractionService(t, factory)
	defer service.Close()
	service.drainTimeout = 0
	projectPath := service.active.packageInfo.Root

	if _, err := service.ExecuteExtractionRun(ctx, taskID); err != nil {
		t.Fatalf("ExecuteExtractionRun: %v", err)
	}
	waitForStickyProviderStarted(t, factory)

	// Switch to a new project — the old store is closed and the lease is stale.
	if _, err := service.CreateProject(ctx, t.TempDir(), "新项目", "奇幻", "简介"); err != nil {
		t.Fatal(err)
	}
	close(factory.release)

	// Reopen the original project. RecoverInterruptedTasks transitions the stuck
	// Running task to Recoverable and appends job_recovered.
	if _, err := service.OpenProject(ctx, projectPath); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}

	store, ok := service.active.store.(ports.ExtractionStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.ExtractionStore", service.active.store)
	}
	events, err := store.JobEvents(ctx, taskID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	// The goroutine persisted job_queued (seq 1), job_started and step_started
	// before blocking in the provider; recovery appends job_recovered next.
	if len(events) < 2 {
		t.Fatalf("len(events) = %d, want at least 2: %#v", len(events), events)
	}
	if events[0].EventType != agentdomain.EventJobQueued || events[0].SequenceNumber != 1 {
		t.Fatalf("event 0 = %#v, want job_queued/seq 1", events[0])
	}
	for i, evt := range events {
		if evt.SequenceNumber != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d (non-contiguous feed)", i, evt.SequenceNumber, i+1)
		}
		if evt.JobKind != agentdomain.JobKindExtraction || evt.ProjectID == "" {
			t.Fatalf("event %d missing kind/project: %#v", i, evt)
		}
	}
	last := events[len(events)-1]
	if last.EventType != agentdomain.EventJobRecovered {
		t.Fatalf("last event type = %q, want job_recovered", last.EventType)
	}

	snapshot, err := store.JobSnapshot(ctx, taskID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != "recoverable" {
		t.Fatalf("snapshot status = %q, want recoverable", snapshot.Status)
	}
	if snapshot.MaxSequence != last.SequenceNumber {
		t.Fatalf("snapshot max_sequence = %d, want %d", snapshot.MaxSequence, last.SequenceNumber)
	}

	jobs, err := service.ListRecoverableJobs(ctx)
	if err != nil {
		t.Fatalf("ListRecoverableJobs: %v", err)
	}
	found := false
	for _, job := range jobs {
		if job.JobID == taskID {
			if job.JobKind != "extraction" {
				t.Fatalf("recovered job kind = %q, want extraction", job.JobKind)
			}
			if job.Status != "recoverable" {
				t.Fatalf("recovered job status = %q, want recoverable", job.Status)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("recovered task not found in ListRecoverableJobs")
	}
}

// TestRecoverInterruptedAgentRunsLeavesTerminalJobsUntouched asserts that
// recovery does not append job_recovered events to already-terminal runs
// (completed/failed/cancelled). Only Running runs are recovered; terminal runs
// keep their existing event feed so the UI does not see a spurious recovery.
func TestRecoverInterruptedAgentRunsLeavesTerminalJobsUntouched(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上。",
			"reviewer": `{"approved":true,"summary":"ok","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	projectPath := service.active.packageInfo.Root

	// A completed run must not gain a job_recovered event on reopen.
	completedRunID := completedAgentRunWithEvents(t, service, chapter, "续写这一章。")

	// Reopen the project. RecoverInterruptedAgentRuns finds no Running runs.
	if _, err := service.OpenProject(ctx, projectPath); err != nil {
		t.Fatalf("OpenProject: %v", err)
	}

	agentStore, ok := service.active.store.(ports.AgentStore)
	if !ok {
		t.Fatalf("active store %T does not implement ports.AgentStore", service.active.store)
	}
	events, err := agentStore.JobEvents(ctx, completedRunID, 0, 100)
	if err != nil {
		t.Fatalf("JobEvents: %v", err)
	}
	for _, evt := range events {
		if evt.EventType == agentdomain.EventJobRecovered {
			t.Fatalf("terminal run gained a job_recovered event: %#v", events)
		}
	}
	snapshot, err := agentStore.JobSnapshot(ctx, completedRunID)
	if err != nil {
		t.Fatalf("JobSnapshot: %v", err)
	}
	if snapshot.Status != "completed" {
		t.Fatalf("snapshot status = %q, want completed", snapshot.Status)
	}
}

// TestListRecoverableJobsReturnsNonTerminalJobs asserts that ListRecoverableJobs
// returns queued agent runs and extraction tasks but excludes terminal
// (completed/failed/cancelled) jobs.
func TestListRecoverableJobsReturnsNonTerminalJobs(t *testing.T) {
	ctx := context.Background()
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer":   "夜色压在山门之上。",
			"reviewer": `{"approved":true,"summary":"ok","issues":[]}`,
		},
	}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()

	// One queued agent run (not yet executed → recoverable).
	queuedRun, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	// One completed agent run (terminal → excluded).
	completedRunID := completedAgentRunWithEvents(t, service, chapter, "续写另一章。")

	// One queued extraction task.
	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "Extraction", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key",
	})
	if err != nil {
		t.Fatal(err)
	}
	importStore, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	project, err := service.active.store.Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	importJob, importChapters := seedAppliedImportJob(
		t, importStore, project.ID, "import-job-rcv", "import-chapter-rcv", "第一章", "正文。",
	)
	if _, err := service.ApplyImportJob(ctx, importJob.ID, []string{importChapters[0].ID}); err != nil {
		t.Fatalf("apply import job: %v", err)
	}
	extractionRun, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  importJob.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
		Instruction:  "抽取故事档案。",
	})
	if err != nil {
		t.Fatalf("start extraction run: %v", err)
	}

	jobs, err := service.ListRecoverableJobs(ctx)
	if err != nil {
		t.Fatalf("ListRecoverableJobs: %v", err)
	}

	var queuedAgent, queuedExtraction int
	for _, job := range jobs {
		switch job.JobID {
		case queuedRun.ID:
			if job.JobKind != "agent" {
				t.Fatalf("queued run job_kind = %q, want agent", job.JobKind)
			}
			if job.Status != "queued" {
				t.Fatalf("queued run status = %q, want queued", job.Status)
			}
			queuedAgent++
		case extractionRun.Task.ID:
			if job.JobKind != "extraction" {
				t.Fatalf("extraction task job_kind = %q, want extraction", job.JobKind)
			}
			if job.Status != "queued" {
				t.Fatalf("extraction task status = %q, want queued", job.Status)
			}
			queuedExtraction++
		case completedRunID:
			t.Fatal("completed run should not appear in recoverable jobs")
		}
	}
	if queuedAgent != 1 {
		t.Fatalf("queued agent count = %d, want 1", queuedAgent)
	}
	if queuedExtraction != 1 {
		t.Fatalf("queued extraction count = %d, want 1", queuedExtraction)
	}
}
