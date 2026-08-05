package extraction

import (
	"errors"
	"testing"
	"time"
)

func TestNewTaskCreatesQueuedTaskWithChapterSteps(t *testing.T) {
	now := time.Date(2026, 8, 5, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	steps := []StepSpec{
		{
			ID:               "step-1",
			ImportChapterID:  "import-chapter-1",
			CreatedChapterID: "chapter-1",
			ChapterTitle:     "第一章 风起",
			SourceStartLine:  1,
			SourceEndLine:    80,
			ChapterBody:      "正文一",
		},
		{
			ID:               "step-2",
			ImportChapterID:  "import-chapter-2",
			CreatedChapterID: "chapter-2",
			ChapterTitle:     "第二章 云涌",
			SourceStartLine:  81,
			SourceEndLine:    200,
			ChapterBody:      "正文二",
		},
	}

	task, err := NewTask(
		"task-1", "project-1", "import-job-1",
		"connection-1", "gpt-4", "提取故事档案",
		steps, 60000, 120000, now,
	)
	if err != nil {
		t.Fatalf("NewTask returned error: %v", err)
	}
	if task.Status != StatusQueued {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.ID != "task-1" || task.ProjectID != "project-1" || task.ImportJobID != "import-job-1" {
		t.Fatalf("identifiers = %#v", task)
	}
	if task.ConnectionID != "connection-1" || task.Model != "gpt-4" {
		t.Fatalf("provider binding = %#v", task)
	}
	if task.TokenBudget != 60000 || task.ContextCharBudget != 120000 {
		t.Fatalf("budgets = %#v", task)
	}
	if task.InputTokens != 0 || task.OutputTokens != 0 {
		t.Fatalf("usage should start at zero: %#v", task)
	}
	if task.StartedAt != nil || task.CompletedAt != nil {
		t.Fatalf("queued task must not have started/completed timestamps: %#v", task)
	}
	wantTime := now.UTC()
	if task.CreatedAt != wantTime || task.UpdatedAt != wantTime {
		t.Fatalf("timestamps not canonicalized to UTC: %#v", task)
	}
	if len(task.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(task.Steps))
	}
	for index, step := range task.Steps {
		if step.TaskID != task.ID {
			t.Fatalf("step %d task id = %q, want %q", index, step.TaskID, task.ID)
		}
		if step.Position != index+1 {
			t.Fatalf("step %d position = %d, want %d", index, step.Position, index+1)
		}
		if step.Status != StepQueued {
			t.Fatalf("step %d status = %q, want queued", index, step.Status)
		}
		if step.ChapterBody == "" {
			t.Fatalf("step %d lost chapter body", index)
		}
	}
}

func TestNewTaskRejectsInvalidInputs(t *testing.T) {
	now := time.Unix(100, 0)
	validSteps := []StepSpec{{
		ID: "step-1", ImportChapterID: "ic-1", CreatedChapterID: "c-1",
		ChapterTitle: "标题", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "正文",
	}}

	tests := []struct {
		name      string
		id        string
		projectID string
		steps     []StepSpec
		tokens    int
		chars     int
		wantError error
	}{
		{name: "blank id", id: " ", steps: validSteps, wantError: ErrBlankValue},
		{name: "blank project", id: "task-1", projectID: " ", steps: validSteps, wantError: ErrBlankValue},
		{name: "no steps", steps: nil, wantError: ErrInvalidTask},
		{name: "zero token budget", steps: validSteps, tokens: 0, chars: 100, wantError: ErrInvalidBudget},
		{name: "zero char budget", steps: validSteps, tokens: 100, chars: 0, wantError: ErrInvalidBudget},
		{name: "duplicate import chapter", steps: []StepSpec{
			{ID: "s1", ImportChapterID: "dup", CreatedChapterID: "c1", ChapterTitle: "一", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "a"},
			{ID: "s2", ImportChapterID: "dup", CreatedChapterID: "c2", ChapterTitle: "二", SourceStartLine: 3, SourceEndLine: 4, ChapterBody: "b"},
		}, wantError: ErrInvalidTask},
		{name: "step blank chapter title", steps: []StepSpec{{
			ID: "s1", ImportChapterID: "ic1", CreatedChapterID: "c1",
			ChapterTitle: "  ", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "a",
		}}, wantError: ErrBlankValue},
		{name: "step invalid line range", steps: []StepSpec{{
			ID: "s1", ImportChapterID: "ic1", CreatedChapterID: "c1",
			ChapterTitle: "标题", SourceStartLine: 5, SourceEndLine: 4, ChapterBody: "a",
		}}, wantError: ErrInvalidStep},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id := test.id
			if id == "" {
				id = "task-1"
			}
			projectID := test.projectID
			if projectID == "" {
				projectID = "project-1"
			}
			tokens, chars := test.tokens, test.chars
			if tokens == 0 && chars == 0 {
				tokens, chars = 60000, 120000
			}
			_, err := NewTask(id, projectID, "import-job-1",
				"connection-1", "model", "",
				test.steps, tokens, chars, now)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestTaskTransitionEnforcesStateMachine(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	task, err := NewTask(
		"task-1", "project-1", "import-job-1",
		"connection-1", "model", "",
		[]StepSpec{{
			ID: "step-1", ImportChapterID: "ic-1", CreatedChapterID: "c-1",
			ChapterTitle: "标题", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "正文",
		}}, 1000, 2000, now,
	)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	// queued -> completed is illegal (must run first).
	if _, err := task.Transition(StatusCompleted, "", now.Add(time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("queued→completed error = %v, want ErrInvalidTransition", err)
	}
	// queued -> running is legal and stamps StartedAt.
	running, err := task.Transition(StatusRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("queued→running: %v", err)
	}
	if running.StartedAt == nil || !running.StartedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("StartedAt not set: %#v", running)
	}
	if running.CompletedAt != nil {
		t.Fatalf("CompletedAt should remain nil while running: %#v", running)
	}
	// running -> queued is illegal.
	if _, err := running.Transition(StatusQueued, "", now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("running→queued error = %v, want ErrInvalidTransition", err)
	}
	// running -> recoverable is legal (interrupted).
	recoverable, err := running.Transition(StatusRecoverable, "中断", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("running→recoverable: %v", err)
	}
	if recoverable.Status != StatusRecoverable || recoverable.ErrorMessage != "中断" {
		t.Fatalf("recoverable state = %#v", recoverable)
	}
	// recoverable -> cancelled is the only exit (retry creates a new task).
	cancelled, err := recoverable.Transition(StatusCancelled, "", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("recoverable→cancelled: %v", err)
	}
	if cancelled.CompletedAt == nil {
		t.Fatalf("CompletedAt should be set after cancel: %#v", cancelled)
	}
}

func TestTaskRecordUsageAccumulatesAndCapsRemaining(t *testing.T) {
	task := Task{TokenBudget: 1000, InputTokens: 200, OutputTokens: 100}
	task = task.RecordUsage(50, 25)
	if task.InputTokens != 250 || task.OutputTokens != 125 {
		t.Fatalf("usage = (%d, %d), want (250, 125)", task.InputTokens, task.OutputTokens)
	}
	if remaining := task.RemainingTokens(); remaining != 625 {
		t.Fatalf("remaining = %d, want 625", remaining)
	}
	// Over-budget clamps to zero.
	task = task.RecordUsage(10000, 0)
	if remaining := task.RemainingTokens(); remaining != 0 {
		t.Fatalf("over-budget remaining = %d, want 0", remaining)
	}
}

func TestStepTransitionEnforcesStateMachine(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	step := Step{ID: "s1", Status: StepQueued}
	// queued -> completed is illegal.
	if _, err := step.Transition(StepCompleted, "", now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("queued→completed error = %v", err)
	}
	running, err := step.Transition(StepRunning, "", now)
	if err != nil {
		t.Fatalf("queued→running: %v", err)
	}
	if running.StartedAt == nil {
		t.Fatalf("StartedAt not set")
	}
	completed, err := running.Transition(StepCompleted, "ok", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("running→completed: %v", err)
	}
	if completed.CompletedAt == nil || completed.Output != "" {
		t.Fatalf("completed state = %#v", completed)
	}
	// completed is terminal.
	if _, err := completed.Transition(StepRunning, "", now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completed→running error = %v", err)
	}
}

func TestWithBudgetsValidates(t *testing.T) {
	task := Task{TokenBudget: 1000, ContextCharBudget: 2000}
	if _, err := task.WithBudgets(0, 1000); !errors.Is(err, ErrInvalidBudget) {
		t.Fatalf("zero token budget error = %v", err)
	}
	if _, err := task.WithBudgets(1000, 0); !errors.Is(err, ErrInvalidBudget) {
		t.Fatalf("zero char budget error = %v", err)
	}
	updated, err := task.WithBudgets(5000, 8000)
	if err != nil {
		t.Fatalf("WithBudgets: %v", err)
	}
	if updated.TokenBudget != 5000 || updated.ContextCharBudget != 8000 {
		t.Fatalf("updated budgets = %#v", updated)
	}
}
