package workspace

import (
	"context"
	"testing"
	"time"
)

// waitForAgentRunStatus polls AgentRun until the run reaches the expected
// status or the deadline expires. ExecuteAgentRun returns immediately with a
// running view (the provider pipeline runs in a background goroutine), so tests
// that assert the final state must wait for the goroutine to persist the
// terminal status before querying events or snapshot projections.
func waitForAgentRunStatus(
	t *testing.T,
	service *Service,
	runID string,
	wantStatus string,
	timeout time.Duration,
) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, err := service.AgentRun(ctx, runID)
		if err != nil {
			t.Fatalf("AgentRun: %v", err)
		}
		if run.Status == wantStatus {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	run, _ := service.AgentRun(ctx, runID)
	t.Fatalf("run %s did not reach status %q within %s, got %q",
		runID, wantStatus, timeout, run.Status)
}

// waitForExtractionTaskStatus polls ExtractionTask until the task reaches the
// expected status or the deadline expires. Mirrors waitForAgentRunStatus for
// the extraction pipeline.
func waitForExtractionTaskStatus(
	t *testing.T,
	service *Service,
	taskID string,
	wantStatus string,
	timeout time.Duration,
) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		view, err := service.ExtractionTask(ctx, taskID)
		if err != nil {
			t.Fatalf("ExtractionTask: %v", err)
		}
		if view.Task.Status == wantStatus {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	view, _ := service.ExtractionTask(ctx, taskID)
	t.Fatalf("task %s did not reach status %q within %s, got %q",
		taskID, wantStatus, timeout, view.Task.Status)
}
