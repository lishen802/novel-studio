package ports

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type CredentialStore interface {
	Save(ctx context.Context, reference, secret string) error
	Get(ctx context.Context, reference string) (string, error)
	Delete(ctx context.Context, reference string) error
}

type ModelGenerateRequest struct {
	Model           string
	System          string
	Input           string
	MaxOutputTokens int
	Temperature     float64
	JSONSchema      json.RawMessage
	SchemaName      string
}

type ModelGenerateResult struct {
	Text              string
	ProviderRequestID string
	InputTokens       int
	OutputTokens      int
}

type ModelProvider interface {
	Generate(
		ctx context.Context,
		apiKey string,
		request ModelGenerateRequest,
		onDelta func(string),
	) (ModelGenerateResult, error)
}

type ModelProviderFactory interface {
	New(connection agentdomain.ProviderConnection) (ModelProvider, error)
}

// JobEvent persistence errors shared by the agent and extraction stores.
// ErrJobEventConflict signals a UNIQUE(job_id, sequence_number) violation;
// ErrJobSnapshotMissing signals that a job has not yet emitted any event.
var (
	ErrJobEventConflict   = errors.New("job event sequence conflict")
	ErrJobSnapshotMissing = errors.New("job snapshot does not exist")
)

// JobEventStore persists the append-only job event feed and the authoritative
// snapshot projection shared by agent runs and extraction tasks. Both
// AgentStore and ExtractionStore embed it so the same feed serves every job
// kind.
type JobEventStore interface {
	// AppendJobEvent inserts a single append-only event row. Callers that need
	// atomic sequence assignment should do so within the same transaction as
	// the run/task mutation (see the store's transactional update path).
	AppendJobEvent(ctx context.Context, event agentdomain.JobEvent) error
	// UpsertJobSnapshot inserts or replaces the authoritative projection for a
	// job, intended to run in the same transaction as the triggering event.
	UpsertJobSnapshot(ctx context.Context, snapshot agentdomain.JobSnapshot) error
	// JobEvents returns events with sequence_number > afterSequence in
	// ascending order for cursor-based replay.
	JobEvents(ctx context.Context, jobID string, afterSequence uint64, limit int) ([]agentdomain.JobEvent, error)
	// JobSnapshot loads the authoritative projection for a job, returning
	// ErrJobSnapshotMissing when no events have been appended yet.
	JobSnapshot(ctx context.Context, jobID string) (agentdomain.JobSnapshot, error)
	// ListRecoverableJobs returns non-terminal (queued, running, recoverable)
	// agent runs and extraction tasks for a project, newest first.
	ListRecoverableJobs(ctx context.Context, projectID string) ([]agentdomain.JobSummary, error)
}

type AgentStore interface {
	JobEventStore
	SaveAgentRuntimeSettings(ctx context.Context, settings agentdomain.RuntimeSettings) error
	AgentRuntimeSettings(ctx context.Context, projectID string) (agentdomain.RuntimeSettings, error)
	UpsertProviderConnection(ctx context.Context, connection agentdomain.ProviderConnection) error
	ListProviderConnections(ctx context.Context) ([]agentdomain.ProviderConnection, error)
	ProviderConnection(ctx context.Context, id string) (agentdomain.ProviderConnection, error)
	DeleteProviderConnection(ctx context.Context, id string) error
	UpsertAgentProfile(ctx context.Context, profile agentdomain.AgentProfile) error
	ListAgentProfiles(ctx context.Context) ([]agentdomain.AgentProfile, error)
	AgentProfile(ctx context.Context, id string) (agentdomain.AgentProfile, error)
	CreateAgentRun(ctx context.Context, run agentdomain.AgentRun) error
	UpdateAgentRun(ctx context.Context, run agentdomain.AgentRun, drafts ...agentdomain.JobEventDraft) error
	AgentRun(ctx context.Context, id string) (agentdomain.AgentRun, error)
	ListAgentRuns(ctx context.Context, limit int) ([]agentdomain.AgentRun, error)
	UpdateAgentRunStep(ctx context.Context, step agentdomain.AgentRunStep, drafts ...agentdomain.JobEventDraft) error
	CreateAgentArtifact(ctx context.Context, artifact agentdomain.AgentArtifact) error
	ListAgentArtifacts(ctx context.Context, runID string) ([]agentdomain.AgentArtifact, error)
	RecoverInterruptedAgentRuns(ctx context.Context, recoveredAt time.Time) (int64, error)
}
