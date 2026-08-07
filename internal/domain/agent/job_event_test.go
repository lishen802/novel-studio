package agent

import (
	"errors"
	"testing"
	"time"
)

func TestNewJobEventValidatesRequiredFields(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	event, err := NewJobEvent(
		"run-1",
		JobKindAgent,
		"project-1",
		1,
		EventJobStarted,
		"step-1",
		`{"note":"ok"}`,
		now,
	)
	if err != nil {
		t.Fatalf("NewJobEvent() error = %v", err)
	}
	if event.JobID != "run-1" ||
		event.JobKind != JobKindAgent ||
		event.ProjectID != "project-1" ||
		event.SequenceNumber != 1 ||
		event.EventType != EventJobStarted ||
		event.StepID != "step-1" ||
		event.Payload != `{"note":"ok"}` ||
		!event.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestNewJobEventAllowsBlankStepAndPayload(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	event, err := NewJobEvent("run-1", JobKindExtraction, "project-1", 3, EventStepCompleted, "  ", "  ", now)
	if err != nil {
		t.Fatalf("NewJobEvent() error = %v", err)
	}
	if event.StepID != "" || event.Payload != "" {
		t.Fatalf("expected trimmed blank fields, got StepID=%q Payload=%q", event.StepID, event.Payload)
	}
}

func TestNewJobEventRejectsInvalidInputs(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		jobID     string
		kind      JobKind
		projectID string
		sequence  uint64
		eventType JobEventType
		wantErr   error
	}{
		{"blank job id", "  ", JobKindAgent, "project-1", 1, EventJobStarted, ErrBlankValue},
		{"blank project id", "run-1", JobKindAgent, "  ", 1, EventJobStarted, ErrBlankValue},
		{"invalid kind", "run-1", JobKind("foo"), "project-1", 1, EventJobStarted, ErrInvalidJobEvent},
		{"zero sequence", "run-1", JobKindAgent, "project-1", 0, EventJobStarted, ErrInvalidJobEvent},
		{"blank event type", "run-1", JobKindAgent, "project-1", 1, "", ErrInvalidJobEvent},
		{"unknown event type", "run-1", JobKindAgent, "project-1", 1, JobEventType("job_wat"), ErrInvalidJobEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewJobEvent(tc.jobID, tc.kind, tc.projectID, tc.sequence, tc.eventType, "", "", now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNewJobSnapshotValidatesBounds(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	snapshot, err := NewJobSnapshot(
		"run-1",
		JobKindAgent,
		"project-1",
		"running",
		3,
		1,
		1200,
		800,
		"review_change_set",
		"",
		now,
	)
	if err != nil {
		t.Fatalf("NewJobSnapshot() error = %v", err)
	}
	if snapshot.MaxSequence != 3 ||
		snapshot.EarliestReplayableSequence != 1 ||
		snapshot.InputTokens != 1200 ||
		snapshot.OutputTokens != 800 ||
		!snapshot.UpdatedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	// earliest == max == 1 is allowed (terminal single-event job).
	if _, err := NewJobSnapshot("run-1", JobKindAgent, "project-1", "completed", 1, 1, 0, 0, "", "", now); err != nil {
		t.Fatalf("expected snapshot with earliest==max==1 to be valid, got %v", err)
	}
}

func TestNewJobSnapshotRejectsInvalidInputs(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name                     string
		jobID                    string
		kind                     JobKind
		projectID                string
		status                   string
		maxSequence              uint64
		earliestReplayable       uint64
		inputTokens              int
		outputTokens             int
		wantErr                  error
	}{
		{"blank job id", "  ", JobKindAgent, "project-1", "running", 3, 1, 0, 0, ErrBlankValue},
		{"blank project id", "run-1", JobKindAgent, "  ", "running", 3, 1, 0, 0, ErrBlankValue},
		{"blank status", "run-1", JobKindAgent, "project-1", "  ", 3, 1, 0, 0, ErrBlankValue},
		{"invalid kind", "run-1", JobKind("foo"), "project-1", "running", 3, 1, 0, 0, ErrInvalidJobEvent},
		{"earliest zero", "run-1", JobKindAgent, "project-1", "running", 3, 0, 0, 0, ErrInvalidJobEvent},
		{"max below earliest", "run-1", JobKindAgent, "project-1", "running", 2, 5, 0, 0, ErrInvalidJobEvent},
		{"negative input tokens", "run-1", JobKindAgent, "project-1", "running", 3, 1, -1, 0, ErrInvalidJobEvent},
		{"negative output tokens", "run-1", JobKindAgent, "project-1", "running", 3, 1, 0, -1, ErrInvalidJobEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewJobSnapshot(
				tc.jobID, tc.kind, tc.projectID, tc.status,
				tc.maxSequence, tc.earliestReplayable,
				tc.inputTokens, tc.outputTokens,
				"", "", now,
			)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNewJobSummaryValidatesRequiredFields(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	summary, err := NewJobSummary("run-1", JobKindAgent, "project-1", "running", 3, now)
	if err != nil {
		t.Fatalf("NewJobSummary() error = %v", err)
	}
	if summary.JobID != "run-1" ||
		summary.JobKind != JobKindAgent ||
		summary.ProjectID != "project-1" ||
		summary.Status != "running" ||
		summary.MaxSequence != 3 ||
		!summary.UpdatedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	if _, err := NewJobSummary("  ", JobKindAgent, "project-1", "running", 1, now); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("expected ErrBlankValue for blank job id, got %v", err)
	}
	if _, err := NewJobSummary("run-1", JobKind("foo"), "project-1", "running", 1, now); !errors.Is(err, ErrInvalidJobEvent) {
		t.Fatalf("expected ErrInvalidJobEvent for invalid kind, got %v", err)
	}
}
