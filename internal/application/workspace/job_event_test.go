package workspace

import (
	"context"
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
	if events[0].EventType != agentdomain.EventJobStarted {
		t.Fatalf("first event type = %q, want job_started", events[0].EventType)
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
	if events[0].EventType != agentdomain.EventJobStarted {
		t.Fatalf("first event type = %q, want job_started", events[0].EventType)
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
	// 8 events: job_started + 3×(step_started+step_completed) + job_completed.
	if view.MaxSequence != 8 {
		t.Fatalf("snapshot max_sequence = %d, want 8", view.MaxSequence)
	}
	if view.EarliestReplayableSequence != 1 {
		t.Fatalf("snapshot earliest_replayable_sequence = %d, want 1", view.EarliestReplayableSequence)
	}
	if view.InputTokens != 30 || view.OutputTokens != 15 {
		t.Fatalf("snapshot tokens = in %d out %d, want 30/15", view.InputTokens, view.OutputTokens)
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

	// Page 2: remaining 6 events after sequence 2.
	page, err = service.GetJobEvents(ctx, runID, 2, 100)
	if err != nil {
		t.Fatalf("GetJobEvents page 2: %v", err)
	}
	if len(page.Events) != 6 {
		t.Fatalf("page 2 len(events) = %d, want 6", len(page.Events))
	}
	if page.Events[0].SequenceNumber != 3 {
		t.Fatalf("page 2 first sequence = %d, want 3", page.Events[0].SequenceNumber)
	}
	if page.Events[5].SequenceNumber != 8 {
		t.Fatalf("page 2 last sequence = %d, want 8", page.Events[5].SequenceNumber)
	}
	if page.HasMore {
		t.Fatal("page 2 hasMore = true, want false")
	}
	if page.SnapshotRequired {
		t.Fatal("page 2 snapshotRequired = true, want false")
	}
	if page.NextCursor != 8 {
		t.Fatalf("page 2 nextCursor = %d, want 8", page.NextCursor)
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
