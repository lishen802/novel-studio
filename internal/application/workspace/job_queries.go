package workspace

import (
	"context"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// JobEventView is the JSON-facing projection of a single append-only JobEvent.
// Payload carries only lightweight summaries; bulky content lives in artifacts
// and is referenced by step_id, not copied into the event feed.
type JobEventView struct {
	JobID          string `json:"jobId"`
	JobKind        string `json:"jobKind"`
	ProjectID      string `json:"projectId"`
	SequenceNumber uint64 `json:"sequenceNumber"`
	EventType      string `json:"eventType"`
	StepID         string `json:"stepId,omitempty"`
	Payload        string `json:"payload,omitempty"`
	CreatedAt      string `json:"createdAt"`
}

// JobSnapshotView is the authoritative projection of a job's current state. The
// frontend uses it on initial mount, after Bridge reattach, or whenever
// GetJobEvents signals snapshotRequired so it can rebuild without replaying the
// full event history.
type JobSnapshotView struct {
	JobID                      string `json:"jobId"`
	JobKind                    string `json:"jobKind"`
	ProjectID                  string `json:"projectId"`
	Status                     string `json:"status"`
	MaxSequence                uint64 `json:"maxSequence"`
	EarliestReplayableSequence uint64 `json:"earliestReplayableSequence"`
	InputTokens                int    `json:"inputTokens"`
	OutputTokens               int    `json:"outputTokens"`
	PendingAction              string `json:"pendingAction,omitempty"`
	TerminalInfo               string `json:"terminalInfo,omitempty"`
	UpdatedAt                  string `json:"updatedAt"`
}

// JobEventsPageView wraps a page of JobEvent rows with cursor paging metadata.
// SnapshotRequired is true when afterSequence is below the snapshot's
// earliest_replayable_sequence; in that case Events is empty and the caller
// must rebuild from GetJobSnapshot before resuming cursor replay.
type JobEventsPageView struct {
	Events           []JobEventView `json:"events"`
	NextCursor       uint64         `json:"nextCursor"`
	HasMore          bool           `json:"hasMore"`
	SnapshotRequired bool           `json:"snapshotRequired"`
}

// JobSummaryView is the lightweight recoverable-jobs listing row. Agent runs
// and extraction tasks fold into a single feed so the UI can present every
// non-terminal or recoverable job on project open.
type JobSummaryView struct {
	JobID       string `json:"jobId"`
	JobKind     string `json:"jobKind"`
	ProjectID   string `json:"projectId"`
	Status      string `json:"status"`
	MaxSequence uint64 `json:"maxSequence"`
	UpdatedAt   string `json:"updatedAt"`
}

// GetJobSnapshot returns the authoritative projection for a job. The frontend
// uses it on initial mount, after Bridge reattach, or whenever GetJobEvents
// signals that the cursor is earlier than the replayable lower bound.
func (s *Service) GetJobSnapshot(ctx context.Context, jobID string) (JobSnapshotView, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return JobSnapshotView{}, agentdomain.ErrBlankValue
	}
	s.mu.RLock()
	store, err := s.jobEventStoreLocked()
	s.mu.RUnlock()
	if err != nil {
		return JobSnapshotView{}, err
	}
	snapshot, err := store.JobSnapshot(ctx, jobID)
	if err != nil {
		return JobSnapshotView{}, err
	}
	return jobSnapshotView(snapshot), nil
}

// GetJobEvents returns events for a job with sequence_number > afterSequence in
// ascending order, along with paging metadata. When afterSequence is below the
// snapshot's earliest_replayable_sequence, the response is empty and
// SnapshotRequired is true so the caller rebuilds from GetJobSnapshot first.
func (s *Service) GetJobEvents(
	ctx context.Context,
	jobID string,
	afterSequence uint64,
	limit int,
) (JobEventsPageView, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return JobEventsPageView{}, agentdomain.ErrBlankValue
	}
	s.mu.RLock()
	store, err := s.jobEventStoreLocked()
	s.mu.RUnlock()
	if err != nil {
		return JobEventsPageView{}, err
	}
	snapshot, err := store.JobSnapshot(ctx, jobID)
	if err != nil {
		return JobEventsPageView{}, err
	}
	// The cursor is stale when the next needed event (afterSequence + 1) falls
	// below the replayable lower bound — events between the cursor and the lower
	// bound have been cleaned up and can't be replayed, so the caller must
	// rebuild from the snapshot. afterSequence=0 with earliest=1 (the common
	// initial pull) is NOT stale because the next needed event (1) is at the
	// lower bound.
	if afterSequence+1 < snapshot.EarliestReplayableSequence {
		return JobEventsPageView{SnapshotRequired: true}, nil
	}
	if limit < 1 {
		limit = 100
	}
	events, err := store.JobEvents(ctx, jobID, afterSequence, limit)
	if err != nil {
		return JobEventsPageView{}, err
	}
	page := JobEventsPageView{Events: make([]JobEventView, 0, len(events))}
	for _, event := range events {
		page.Events = append(page.Events, jobEventView(event))
	}
	if len(events) > 0 {
		page.NextCursor = events[len(events)-1].SequenceNumber
		page.HasMore = page.NextCursor < snapshot.MaxSequence
	} else {
		page.NextCursor = afterSequence
	}
	return page, nil
}

// ListRecoverableJobs returns non-terminal (queued, running, recoverable) agent
// runs and extraction tasks for the active project, newest first. The frontend
// uses it on project open to discover jobs that need cursor replay.
func (s *Service) ListRecoverableJobs(ctx context.Context) ([]JobSummaryView, error) {
	s.mu.RLock()
	store, err := s.jobEventStoreLocked()
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	project, projectErr := s.active.store.Project(ctx)
	s.mu.RUnlock()
	if projectErr != nil {
		return nil, projectErr
	}
	summaries, err := store.ListRecoverableJobs(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	views := make([]JobSummaryView, 0, len(summaries))
	for _, summary := range summaries {
		views = append(views, jobSummaryView(summary))
	}
	return views, nil
}

// jobEventStoreLocked returns the active store as a JobEventStore. It must be
// called while s.mu is held (at least RLock). The SQLite store implements
// JobEventStore via both AgentStore and ExtractionStore, so the same queries
// serve agent and extraction jobs without a kind switch.
func (s *Service) jobEventStoreLocked() (ports.JobEventStore, error) {
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, ok := s.active.store.(ports.JobEventStore)
	if !ok {
		return nil, ErrAgentStoreUnavailable
	}
	return store, nil
}

func jobEventView(event agentdomain.JobEvent) JobEventView {
	return JobEventView{
		JobID:          event.JobID,
		JobKind:        string(event.JobKind),
		ProjectID:      event.ProjectID,
		SequenceNumber: event.SequenceNumber,
		EventType:      string(event.EventType),
		StepID:         event.StepID,
		Payload:        event.Payload,
		CreatedAt:      event.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func jobSnapshotView(snapshot agentdomain.JobSnapshot) JobSnapshotView {
	return JobSnapshotView{
		JobID:                      snapshot.JobID,
		JobKind:                    string(snapshot.JobKind),
		ProjectID:                  snapshot.ProjectID,
		Status:                     snapshot.Status,
		MaxSequence:                snapshot.MaxSequence,
		EarliestReplayableSequence: snapshot.EarliestReplayableSequence,
		InputTokens:                snapshot.InputTokens,
		OutputTokens:               snapshot.OutputTokens,
		PendingAction:              snapshot.PendingAction,
		TerminalInfo:               snapshot.TerminalInfo,
		UpdatedAt:                  snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func jobSummaryView(summary agentdomain.JobSummary) JobSummaryView {
	return JobSummaryView{
		JobID:       summary.JobID,
		JobKind:     string(summary.JobKind),
		ProjectID:   summary.ProjectID,
		Status:      summary.Status,
		MaxSequence: summary.MaxSequence,
		UpdatedAt:   summary.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
