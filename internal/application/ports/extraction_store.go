package ports

import (
	"context"
	"errors"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
)

var (
	ErrExtractionTaskMissing  = errors.New("extraction task does not exist")
	ErrExtractionTaskConflict = errors.New("extraction task state changed concurrently")
	ErrExtractionStepMissing  = errors.New("extraction step does not exist")
	ErrExtractionPersistence  = errors.New("extraction persistence operation failed")
)

// ExtractionArtifact is a snapshot persisted alongside a task for audit. The
// Kind names what the content represents (for example, "context_manifest" or
// "model_response").
type ExtractionArtifact struct {
	ID        string
	TaskID    string
	StepID    string
	Kind      string
	Content   string
	SHA256    string
	CreatedAt time.Time
}

type ExtractionStore interface {
	JobEventStore
	// CreateTask persists a queued task and every step in one transaction.
	CreateTask(ctx context.Context, task extractiondomain.Task) error
	// UpdateTask persists lifecycle transitions for the task. Implementations
	// must enforce that the row exists and that change_set_id/retry_of are
	// only written once. When drafts are provided, the events and an upserted
	// job snapshot commit atomically with the task row update in the same
	// transaction.
	UpdateTask(ctx context.Context, task extractiondomain.Task, drafts ...agentdomain.JobEventDraft) error
	// Task loads a task and its steps ordered by position.
	Task(ctx context.Context, id string) (extractiondomain.Task, error)
	// ListTasks returns the most recent tasks for a project, newest first.
	ListTasks(ctx context.Context, projectID string, limit int) ([]extractiondomain.Task, error)
	// TasksForImport returns tasks bound to the supplied ImportJob ID, ordered
	// newest first so callers can find the latest run for retry/continue UX.
	TasksForImport(ctx context.Context, importJobID string) ([]extractiondomain.Task, error)
	// UpdateStep persists lifecycle transitions for a single step. When drafts
	// are provided, the events and an upserted job snapshot commit atomically
	// with the step row update.
	UpdateStep(ctx context.Context, step extractiondomain.Step, drafts ...agentdomain.JobEventDraft) error
	// CreateArtifact persists an audit snapshot for a task or step.
	CreateArtifact(ctx context.Context, artifact ExtractionArtifact) error
	// ListArtifacts returns artifacts for a task ordered by creation time.
	ListArtifacts(ctx context.Context, taskID string) ([]ExtractionArtifact, error)
	// RecoverInterruptedTasks marks running tasks/steps as recoverable so they
	// can be retried after an unclean shutdown. Returns the affected task count.
	RecoverInterruptedTasks(ctx context.Context, recoveredAt time.Time) (int64, error)
}
