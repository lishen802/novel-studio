package agent

import (
	"errors"
	"strings"
	"time"
)

// JobEvent errors are isolated from the legacy run/profile errors so callers
// can distinguish append-only feed validation failures.
var (
	ErrInvalidJobEvent = errors.New("invalid job event")
)

// JobKind identifies which family of long-running work a JobEvent belongs to.
// Agent runs and extraction tasks share the same event feed shape but remain
// distinguishable so snapshots and reducers can route state correctly.
type JobKind string

const (
	JobKindAgent      JobKind = "agent"
	JobKindExtraction JobKind = "extraction"
)

// JobEventType enumerates the append-only events written alongside job state
// transitions. Step deltas intentionally avoid copying large text payloads;
// bulky content lives in agent_artifacts/extraction_artifacts and events only
// reference summaries.
type JobEventType string

const (
	EventJobQueued     JobEventType = "job_queued"
	EventJobStarted    JobEventType = "job_started"
	EventStepStarted   JobEventType = "step_started"
	EventStepDelta     JobEventType = "step_delta"
	EventStepCompleted JobEventType = "step_completed"
	EventStepFailed    JobEventType = "step_failed"
	EventJobCompleted  JobEventType = "job_completed"
	EventJobFailed     JobEventType = "job_failed"
	EventJobCancelled  JobEventType = "job_cancelled"
	EventJobRecovered  JobEventType = "job_recovered"
)

// JobEvent is an append-only record of a single state change for a job. Events
// are assigned a per-job monotonic sequence number in the same transaction that
// mutates agent_runs/extraction_tasks, so the feed is the authoritative history
// once committed.
//
// ProjectID is denormalized onto each row so that deleting a project cascades to
// its event history via the projects(id) foreign key. This is referential
// integrity, not retention cleanup (R1.2 keeps all events for replay).
type JobEvent struct {
	JobID          string
	JobKind        JobKind
	ProjectID      string
	SequenceNumber uint64
	EventType      JobEventType
	StepID         string
	Payload        string
	CreatedAt      time.Time
}

// NewJobEvent constructs a validated JobEvent. jobID/kind/projectID/sequence/
// eventType are required; stepID and payload are optional and trimmed. The
// caller assigns the sequence number inside the persistence transaction;
// NewJobEvent only validates that it is at least 1.
func NewJobEvent(
	jobID string,
	kind JobKind,
	projectID string,
	sequence uint64,
	eventType JobEventType,
	stepID, payload string,
	now time.Time,
) (JobEvent, error) {
	jobID = strings.TrimSpace(jobID)
	projectID = strings.TrimSpace(projectID)
	if jobID == "" || projectID == "" {
		return JobEvent{}, ErrBlankValue
	}
	if !validJobKind(kind) {
		return JobEvent{}, ErrInvalidJobEvent
	}
	if sequence < 1 {
		return JobEvent{}, ErrInvalidJobEvent
	}
	if !validJobEventType(eventType) {
		return JobEvent{}, ErrInvalidJobEvent
	}
	return JobEvent{
		JobID:          jobID,
		JobKind:        kind,
		ProjectID:      projectID,
		SequenceNumber: sequence,
		EventType:      eventType,
		StepID:         strings.TrimSpace(stepID),
		Payload:        strings.TrimSpace(payload),
		CreatedAt:      now.UTC(),
	}, nil
}

// JobSnapshot is the authoritative projection of a job's current state,
// maintained in the same transaction that appends JobEvents. It lets the UI
// rebuild state without replaying the full event history, and tracks the
// earliest replayable sequence so cursor reads can detect when they fall
// below the retained lower bound.
type JobSnapshot struct {
	JobID                      string
	JobKind                    JobKind
	ProjectID                  string
	Status                     string
	MaxSequence                uint64
	EarliestReplayableSequence uint64
	InputTokens                int
	OutputTokens               int
	PendingAction              string
	TerminalInfo               string
	StepStates                 []JobStepState
	PublicProjection           JobPublicProjection
	UpdatedAt                  time.Time
}

type JobPublicProjection struct {
	StageID            PublicStageID `json:"stageId,omitempty"`
	CompletedSteps     int           `json:"completedSteps"`
	TotalSteps         int           `json:"totalSteps"`
	QualityStatus      QualityStatus `json:"qualityStatus,omitempty"`
	Recoverable        bool          `json:"recoverable"`
	CandidateAvailable bool          `json:"candidateAvailable"`
}

// JobStepState is the compact provider-neutral step projection persisted in a
// JobSnapshot, so a remount can rebuild an in-progress job without replaying
// every historical event.
type JobStepState struct {
	StepID string `json:"stepId"`
	Status string `json:"status"`
}

// NewJobSnapshot constructs a validated JobSnapshot. maxSequence must be >=
// earliestReplayableSequence >= 1: every persisted job has an initial
// job_queued event, so the lower bound never drops below 1.
func NewJobSnapshot(
	jobID string,
	kind JobKind,
	projectID, status string,
	maxSequence, earliestReplayableSequence uint64,
	inputTokens, outputTokens int,
	pendingAction, terminalInfo string,
	now time.Time,
) (JobSnapshot, error) {
	jobID = strings.TrimSpace(jobID)
	projectID = strings.TrimSpace(projectID)
	status = strings.TrimSpace(status)
	if jobID == "" || projectID == "" || status == "" {
		return JobSnapshot{}, ErrBlankValue
	}
	if !validJobKind(kind) {
		return JobSnapshot{}, ErrInvalidJobEvent
	}
	if earliestReplayableSequence < 1 {
		return JobSnapshot{}, ErrInvalidJobEvent
	}
	if maxSequence < earliestReplayableSequence {
		return JobSnapshot{}, ErrInvalidJobEvent
	}
	if inputTokens < 0 || outputTokens < 0 {
		return JobSnapshot{}, ErrInvalidJobEvent
	}
	return JobSnapshot{
		JobID:                      jobID,
		JobKind:                    kind,
		ProjectID:                  projectID,
		Status:                     status,
		MaxSequence:                maxSequence,
		EarliestReplayableSequence: earliestReplayableSequence,
		InputTokens:                inputTokens,
		OutputTokens:               outputTokens,
		PendingAction:              strings.TrimSpace(pendingAction),
		TerminalInfo:               strings.TrimSpace(terminalInfo),
		UpdatedAt:                  now.UTC(),
	}, nil
}

// JobEventDraft is a JobEvent whose SequenceNumber has not yet been assigned.
// Callers that emit events alongside a run/task transition construct drafts
// from domain context; the persistence layer assigns the per-job monotonic
// sequence inside the same transaction that mutates the run/task row, then
// materializes them into JobEvent rows. Splitting the draft from the final
// event keeps sequence assignment centralized in the store so concurrent
// transitions cannot race on the next sequence value.
type JobEventDraft struct {
	JobID     string
	JobKind   JobKind
	ProjectID string
	EventType JobEventType
	StepID    string
	Payload   string
	CreatedAt time.Time
}

// NewJobEventDraft constructs a validated draft. jobID/kind/projectID/eventType
// are required; stepID and payload are optional and trimmed. The sequence
// number is intentionally absent and assigned by the store on commit.
func NewJobEventDraft(
	jobID string,
	kind JobKind,
	projectID string,
	eventType JobEventType,
	stepID, payload string,
	now time.Time,
) (JobEventDraft, error) {
	jobID = strings.TrimSpace(jobID)
	projectID = strings.TrimSpace(projectID)
	if jobID == "" || projectID == "" {
		return JobEventDraft{}, ErrBlankValue
	}
	if !validJobKind(kind) {
		return JobEventDraft{}, ErrInvalidJobEvent
	}
	if !validJobEventType(eventType) {
		return JobEventDraft{}, ErrInvalidJobEvent
	}
	return JobEventDraft{
		JobID:     jobID,
		JobKind:   kind,
		ProjectID: projectID,
		EventType: eventType,
		StepID:    strings.TrimSpace(stepID),
		Payload:   strings.TrimSpace(payload),
		CreatedAt: now.UTC(),
	}, nil
}

// JobSummary is the lightweight recoverable-jobs listing row. It folds agent
// runs and extraction tasks into a single feed so the UI can present every
// non-terminal or recoverable job on project open.
type JobSummary struct {
	JobID       string
	JobKind     JobKind
	ProjectID   string
	Status      string
	MaxSequence uint64
	UpdatedAt   time.Time
}

// NewJobSummary constructs a validated JobSummary. Status is kept as a free
// string rather than a typed enum because agent and extraction statuses share
// a vocabulary but are defined on their respective domain types.
func NewJobSummary(
	jobID string,
	kind JobKind,
	projectID, status string,
	maxSequence uint64,
	now time.Time,
) (JobSummary, error) {
	jobID = strings.TrimSpace(jobID)
	projectID = strings.TrimSpace(projectID)
	status = strings.TrimSpace(status)
	if jobID == "" || projectID == "" || status == "" {
		return JobSummary{}, ErrBlankValue
	}
	if !validJobKind(kind) {
		return JobSummary{}, ErrInvalidJobEvent
	}
	return JobSummary{
		JobID:       jobID,
		JobKind:     kind,
		ProjectID:   projectID,
		Status:      status,
		MaxSequence: maxSequence,
		UpdatedAt:   now.UTC(),
	}, nil
}

func validJobKind(kind JobKind) bool {
	return kind == JobKindAgent || kind == JobKindExtraction
}

func validJobEventType(eventType JobEventType) bool {
	switch eventType {
	case EventJobQueued,
		EventJobStarted,
		EventStepStarted,
		EventStepDelta,
		EventStepCompleted,
		EventStepFailed,
		EventJobCompleted,
		EventJobFailed,
		EventJobCancelled,
		EventJobRecovered:
		return true
	default:
		return false
	}
}
