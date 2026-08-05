// Package extraction defines the manuscript extraction task domain.
//
// An ExtractionTask drives AI-driven asset extraction over the chapters created
// by an applied ImportJob. Each task holds one Step per chapter; steps run in
// order so that long-form manuscripts can be processed chapter-by-chapter and
// resumed after interruption. The merged candidate output is materialized as a
// revision.ChangeSet by the application layer, leaving authors in control of
// what becomes canonical.
package extraction

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue        = errors.New("required extraction value must not be blank")
	ErrInvalidBudget     = errors.New("invalid extraction budget")
	ErrInvalidTask       = errors.New("invalid extraction task")
	ErrInvalidStep       = errors.New("invalid extraction step")
	ErrInvalidTransition = errors.New("invalid extraction status transition")
)

const (
	DefaultMaxRunTokens    = 60000
	DefaultMaxContextChars = 120000
)

// Status is the lifecycle state of an ExtractionTask.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusRunning     Status = "running"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
	StatusCancelled   Status = "cancelled"
	StatusRecoverable Status = "recoverable"
)

// StepStatus is the lifecycle state of a single ExtractionStep.
type StepStatus string

const (
	StepQueued    StepStatus = "queued"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepCancelled StepStatus = "cancelled"
)

// StepSpec describes a chapter to extract from when constructing a Task.
type StepSpec struct {
	ID                string
	ImportChapterID   string
	CreatedChapterID  string
	ChapterTitle      string
	SourceStartLine   int
	SourceEndLine     int
	ChapterBody       string
}

// Task is an AI extraction job bound to an applied ImportJob.
type Task struct {
	ID                string
	ProjectID         string
	ImportJobID       string
	ConnectionID      string
	Model             string
	Prompt            string
	Status            Status
	ErrorCode         string
	ErrorMessage      string
	ChangeSetID       string
	RetryOf           string
	TokenBudget       int
	ContextCharBudget int
	InputTokens       int
	OutputTokens      int
	Steps             []Step
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
}

// Step extracts candidates from a single chapter.
type Step struct {
	ID                string
	TaskID            string
	ImportChapterID   string
	CreatedChapterID  string
	ChapterTitle      string
	Position          int
	SourceStartLine   int
	SourceEndLine     int
	ChapterBody       string
	Status            StepStatus
	Output            string
	ErrorCode         string
	ErrorMessage      string
	ProviderRequestID string
	InputTokens       int
	OutputTokens      int
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewTask constructs a queued ExtractionTask with one Step per supplied chapter.
func NewTask(
	id, projectID, importJobID, connectionID, model, prompt string,
	steps []StepSpec,
	tokenBudget, contextCharBudget int,
	now time.Time,
) (Task, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	importJobID = strings.TrimSpace(importJobID)
	connectionID = strings.TrimSpace(connectionID)
	model = strings.TrimSpace(model)
	prompt = strings.TrimSpace(prompt)
	if id == "" || projectID == "" || importJobID == "" || connectionID == "" || model == "" {
		return Task{}, ErrBlankValue
	}
	if len(steps) == 0 {
		return Task{}, ErrInvalidTask
	}
	if tokenBudget < 1 || contextCharBudget < 1 {
		return Task{}, ErrInvalidBudget
	}

	now = now.UTC()
	domainSteps := make([]Step, 0, len(steps))
	seenImportChapter := make(map[string]struct{}, len(steps))
	for index, spec := range steps {
		step, err := newStep(spec, id, index+1, now)
		if err != nil {
			return Task{}, err
		}
		if _, exists := seenImportChapter[step.ImportChapterID]; exists {
			return Task{}, ErrInvalidTask
		}
		seenImportChapter[step.ImportChapterID] = struct{}{}
		domainSteps = append(domainSteps, step)
	}
	return Task{
		ID:                id,
		ProjectID:         projectID,
		ImportJobID:       importJobID,
		ConnectionID:      connectionID,
		Model:             model,
		Prompt:            prompt,
		Status:            StatusQueued,
		TokenBudget:       tokenBudget,
		ContextCharBudget: contextCharBudget,
		Steps:             domainSteps,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func newStep(spec StepSpec, taskID string, position int, now time.Time) (Step, error) {
	spec.ID = strings.TrimSpace(spec.ID)
	spec.ImportChapterID = strings.TrimSpace(spec.ImportChapterID)
	spec.CreatedChapterID = strings.TrimSpace(spec.CreatedChapterID)
	spec.ChapterTitle = strings.TrimSpace(spec.ChapterTitle)
	spec.ChapterBody = strings.TrimSpace(spec.ChapterBody)
	if spec.ID == "" || spec.ImportChapterID == "" || spec.CreatedChapterID == "" || spec.ChapterTitle == "" {
		return Step{}, ErrBlankValue
	}
	if position < 1 || spec.SourceStartLine < 1 || spec.SourceEndLine < spec.SourceStartLine {
		return Step{}, ErrInvalidStep
	}
	return Step{
		ID:               spec.ID,
		TaskID:           taskID,
		ImportChapterID:  spec.ImportChapterID,
		CreatedChapterID: spec.CreatedChapterID,
		ChapterTitle:     spec.ChapterTitle,
		Position:         position,
		SourceStartLine:  spec.SourceStartLine,
		SourceEndLine:    spec.SourceEndLine,
		ChapterBody:      spec.ChapterBody,
		Status:           StepQueued,
		CreatedAt:        now.UTC(),
		UpdatedAt:        now.UTC(),
	}, nil
}

// WithBudgets overrides the token and context character budgets.
func (task Task) WithBudgets(tokenBudget, contextCharBudget int) (Task, error) {
	if tokenBudget < 1 || contextCharBudget < 1 {
		return Task{}, ErrInvalidBudget
	}
	task.TokenBudget = tokenBudget
	task.ContextCharBudget = contextCharBudget
	return task, nil
}

// RecordUsage accumulates token usage from a completed step.
func (task Task) RecordUsage(inputTokens, outputTokens int) Task {
	if inputTokens > 0 {
		task.InputTokens += inputTokens
	}
	if outputTokens > 0 {
		task.OutputTokens += outputTokens
	}
	return task
}

// RemainingTokens reports the unused token budget.
func (task Task) RemainingTokens() int {
	remaining := task.TokenBudget - task.InputTokens - task.OutputTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Transition moves the task to a new lifecycle state.
func (task Task) Transition(status Status, message string, now time.Time) (Task, error) {
	if !validTaskTransition(task.Status, status) {
		return Task{}, ErrInvalidTransition
	}
	now = now.UTC()
	task.Status = status
	task.ErrorMessage = strings.TrimSpace(message)
	task.UpdatedAt = now
	if status == StatusRunning {
		task.StartedAt = &now
	}
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		task.CompletedAt = &now
	}
	return task, nil
}

func validTaskTransition(from, to Status) bool {
	switch from {
	case StatusQueued:
		return to == StatusRunning || to == StatusCancelled
	case StatusRunning:
		return to == StatusCompleted || to == StatusFailed || to == StatusCancelled || to == StatusRecoverable
	case StatusRecoverable:
		return to == StatusCancelled
	default:
		return false
	}
}

// Transition moves the step to a new lifecycle state.
func (step Step) Transition(status StepStatus, message string, now time.Time) (Step, error) {
	if !validStepTransition(step.Status, status) {
		return Step{}, ErrInvalidTransition
	}
	now = now.UTC()
	step.Status = status
	step.ErrorMessage = strings.TrimSpace(message)
	step.UpdatedAt = now
	if status == StepRunning {
		step.StartedAt = &now
	}
	if status == StepCompleted || status == StepFailed || status == StepCancelled {
		step.CompletedAt = &now
	}
	return step, nil
}

func validStepTransition(from, to StepStatus) bool {
	switch from {
	case StepQueued:
		return to == StepRunning || to == StepCancelled
	case StepRunning:
		return to == StepCompleted || to == StepFailed || to == StepCancelled
	default:
		return false
	}
}
