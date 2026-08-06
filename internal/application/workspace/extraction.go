package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
	importingdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// ExtractionRunInput captures the author's selection for a new extraction run.
type ExtractionRunInput struct {
	ImportJobID  string `json:"importJobId"`
	ConnectionID string `json:"connectionId"`
	Model        string `json:"model"`
	Instruction  string `json:"instruction"`
}

type ExtractionStepView struct {
	ID                string `json:"id"`
	ImportChapterID   string `json:"importChapterId"`
	CreatedChapterID  string `json:"createdChapterId"`
	ChapterTitle      string `json:"chapterTitle"`
	Position          int    `json:"position"`
	Status            string `json:"status"`
	Output            string `json:"output"`
	ErrorCode         string `json:"errorCode"`
	ErrorMessage      string `json:"errorMessage"`
	ProviderRequestID string `json:"providerRequestId"`
	InputTokens       int    `json:"inputTokens"`
	OutputTokens      int    `json:"outputTokens"`
}

type ExtractionTaskView struct {
	ID                string               `json:"id"`
	ImportJobID       string               `json:"importJobId"`
	ConnectionID      string               `json:"connectionId"`
	Model             string               `json:"model"`
	Prompt            string               `json:"prompt"`
	Status            string               `json:"status"`
	ErrorCode         string               `json:"errorCode"`
	ErrorMessage      string               `json:"errorMessage"`
	ChangeSetID       string               `json:"changeSetId"`
	RetryOf           string               `json:"retryOf"`
	TokenBudget       int                  `json:"tokenBudget"`
	ContextCharBudget int                  `json:"contextCharBudget"`
	InputTokens       int                  `json:"inputTokens"`
	OutputTokens      int                  `json:"outputTokens"`
	Steps             []ExtractionStepView `json:"steps"`
	CreatedAt         string               `json:"createdAt"`
	UpdatedAt         string               `json:"updatedAt"`
}

type EvidenceView struct {
	ChapterID    string `json:"chapterId"`
	ChapterTitle string `json:"chapterTitle"`
	StartLine    int    `json:"startLine"`
	EndLine      int    `json:"endLine"`
	Excerpt      string `json:"excerpt"`
}

type ExtractionRunView struct {
	Task      ExtractionTaskView `json:"task"`
	ChangeSet *ChangeSetView     `json:"changeSet,omitempty"`
}

func (s *Service) StartExtractionRun(ctx context.Context, input ExtractionRunInput) (ExtractionRunView, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return ExtractionRunView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		return ExtractionRunView{}, err
	}
	agentStore, err := s.agentStoreLocked()
	if err != nil {
		return ExtractionRunView{}, err
	}
	if s.credentials == nil || s.providers == nil {
		return ExtractionRunView{}, ErrAgentRuntimeUnavailable
	}
	importStore, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return ExtractionRunView{}, err
	}
	importJobID := strings.TrimSpace(input.ImportJobID)
	details, err := importStore.ImportJob(ctx, importJobID)
	if err != nil {
		return ExtractionRunView{}, stableImportStoreError(err)
	}
	if details.Job.ProjectID != project.ID {
		return ExtractionRunView{}, ErrImportJobProjectMismatch
	}
	if details.Job.Status != importingdomain.JobStatusApplied {
		return ExtractionRunView{}, fmt.Errorf("import job %s is not applied", importJobID)
	}
	if details.Job.ExtractionStatus == importingdomain.ExtractionStatusRunning {
		return ExtractionRunView{}, ErrExtractionRunActive
	}

	stepSpecs := make([]extractiondomain.StepSpec, 0, len(details.Chapters))
	for _, chapter := range details.Chapters {
		if !chapter.Selected || chapter.CreatedChapterID == "" {
			continue
		}
		createdChapter, err := s.active.store.Chapter(ctx, chapter.CreatedChapterID)
		if err != nil {
			return ExtractionRunView{}, fmt.Errorf("read created chapter %s: %w", chapter.CreatedChapterID, err)
		}
		stepSpecs = append(stepSpecs, extractiondomain.StepSpec{
			ID:               s.ids.NewID(),
			ImportChapterID:  chapter.ID,
			CreatedChapterID: chapter.CreatedChapterID,
			ChapterTitle:     chapter.Title,
			SourceStartLine:  chapter.SourceStartLine,
			SourceEndLine:    chapter.SourceEndLine,
			ChapterBody:      createdChapter.Body,
		})
	}
	if len(stepSpecs) == 0 {
		return ExtractionRunView{}, fmt.Errorf("import job %s has no chapters to extract", importJobID)
	}

	settings, err := agentStore.AgentRuntimeSettings(ctx, project.ID)
	if err != nil {
		return ExtractionRunView{}, err
	}
	task, err := extractiondomain.NewTask(
		s.ids.NewID(), project.ID, importJobID,
		strings.TrimSpace(input.ConnectionID), strings.TrimSpace(input.Model),
		strings.TrimSpace(input.Instruction),
		stepSpecs,
		settings.MaxRunTokens, settings.MaxContextChars,
		s.clock.Now(),
	)
	if err != nil {
		return ExtractionRunView{}, err
	}
	if err := store.CreateTask(ctx, task); err != nil {
		return ExtractionRunView{}, fmt.Errorf("create extraction task: %w", err)
	}
	return extractionRunView(task, nil), nil
}

func (s *Service) ExecuteExtractionRun(ctx context.Context, id string) (ExtractionRunView, error) {
	taskID := strings.TrimSpace(id)
	executionCtx, err := s.beginExtractionJob(ctx, taskID)
	if err != nil {
		return ExtractionRunView{}, err
	}
	defer s.endExtractionJob(taskID)
	ctx = executionCtx

	// Phase 1: short write lock. Capture the extraction/agent/import stores,
	// the project store and generation, acquire a job lease, transition the
	// task to Running and persist.
	s.mu.Lock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	agentStore, err := s.agentStoreLocked()
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	if s.credentials == nil || s.providers == nil {
		s.mu.Unlock()
		return ExtractionRunView{}, ErrAgentRuntimeUnavailable
	}

	task, err := store.Task(ctx, taskID)
	if err != nil {
		if ctx.Err() != nil {
			s.mu.Unlock()
			return s.persistCancelledExtractionByID(context.WithoutCancel(ctx), store, taskID)
		}
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	if task.Status != extractiondomain.StatusQueued {
		s.mu.Unlock()
		return ExtractionRunView{}, fmt.Errorf("extraction task is not queued: %s", task.Status)
	}

	projectStore := s.active.store
	project, err := projectStore.Project(ctx)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	importStore, ok := projectStore.(ports.ImportStore)
	if !ok {
		s.mu.Unlock()
		return ExtractionRunView{}, ErrImportStoreUnavailable
	}
	importDetails, err := importStore.ImportJob(ctx, task.ImportJobID)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, stableImportStoreError(err)
	}

	connection, err := agentStore.ProviderConnection(ctx, task.ConnectionID)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	apiKey, err := s.credentials.Get(ctx, connection.CredentialRef)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	provider, err := s.providers.New(connection)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}

	generation := s.active.generation
	lease, err := s.leases.acquire(taskID, generation)
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	task, _ = task.Transition(extractiondomain.StatusRunning, "", s.clock.Now())
	if err := store.UpdateTask(ctx, task); err != nil {
		s.mu.Unlock()
		if ctx.Err() != nil {
			return s.persistCancelledExtractionByID(context.WithoutCancel(ctx), store, taskID)
		}
		return ExtractionRunView{}, err
	}
	currentJob, err := importDetails.Job.MarkExtractionStarted(task.ID, s.clock.Now())
	if err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, err
	}
	if err := importStore.UpdateImportJobExtraction(ctx, currentJob); err != nil {
		s.mu.Unlock()
		return ExtractionRunView{}, stableImportStoreError(err)
	}
	s.mu.Unlock()

	// stale reports whether the store generation has changed or the lease was
	// force-marked stale by a drain timeout.
	stale := func() bool {
		return lease.IsStale() || s.currentGeneration() != generation
	}

	// failStale transitions the task to Recoverable without touching the
	// (possibly closed) captured stores.
	failStale := func() (ExtractionRunView, error) {
		task, _ = task.Transition(extractiondomain.StatusRecoverable, ErrAgentRunStaleStore.Error(), s.clock.Now())
		return extractionRunView(task, nil), ErrAgentRunStaleStore
	}

	// failed is the exit path for errors during the lock-free phase.
	failed := func(cause error) (ExtractionRunView, error) {
		if stale() {
			return failStale()
		}
		if errors.Is(cause, context.Canceled) {
			persistCtx := context.WithoutCancel(ctx)
			if cancelErr := s.persistCancelledExtraction(persistCtx, store, task); cancelErr != nil {
				return ExtractionRunView{}, cancelErr
			}
			task, _ = store.Task(persistCtx, task.ID)
			return extractionRunView(task, nil), nil
		}
		task.ErrorCode = "provider_error"
		task, _ = task.Transition(extractiondomain.StatusFailed, cause.Error(), s.clock.Now())
		_ = store.UpdateTask(context.WithoutCancel(ctx), task)
		failedJob, _ := currentJob.MarkExtractionFailed(s.clock.Now())
		_ = importStore.UpdateImportJobExtraction(context.WithoutCancel(ctx), failedJob)
		return extractionRunView(task, nil), cause
	}

	// Phase 2: lock-free context assembly and provider calls.
	for index := range task.Steps {
		step := task.Steps[index]
		if ctx.Err() != nil {
			return failed(ctx.Err())
		}
		runningStep, err := step.Transition(extractiondomain.StepRunning, "", s.clock.Now())
		if err != nil {
			return failed(err)
		}
		if stale() {
			return failStale()
		}
		if err := store.UpdateStep(ctx, runningStep); err != nil {
			return failed(err)
		}

		contextJSON, err := assembleExtractionContext(runningStep, task.Prompt, project.ID)
		if err != nil {
			return failed(err)
		}
		if stale() {
			return failStale()
		}
		if err := store.CreateArtifact(ctx, newExtractionArtifact(
			s.ids.NewID(), task.ID, runningStep.ID, "context_manifest", contextJSON, s.clock.Now(),
		)); err != nil {
			return failed(err)
		}

		// Provider call — lock-free.
		generated, err := provider.Generate(ctx, apiKey, ports.ModelGenerateRequest{
			Model:           task.Model,
			System:          extractionSystemPrompt,
			Input:           contextJSON,
			MaxOutputTokens: task.TokenBudget,
			Temperature:     0.2,
			SchemaName:      "manuscript_extraction",
			JSONSchema:      json.RawMessage(extractionSchema),
		}, nil)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				task.Steps[index] = runningStep
				return failed(err)
			}
			failedStep, _ := runningStep.Transition(extractiondomain.StepFailed, err.Error(), s.clock.Now())
			failedStep.ErrorCode = "provider_error"
			failedStep.ErrorMessage = err.Error()
			if !stale() {
				_ = store.UpdateStep(context.WithoutCancel(ctx), failedStep)
			}
			task.Steps[index] = failedStep
			return failed(err)
		}

		completedStep, err := runningStep.Transition(extractiondomain.StepCompleted, "", s.clock.Now())
		if err != nil {
			return failed(err)
		}
		completedStep.Output = generated.Text
		completedStep.ProviderRequestID = generated.ProviderRequestID
		completedStep.InputTokens = generated.InputTokens
		completedStep.OutputTokens = generated.OutputTokens
		task = task.RecordUsage(generated.InputTokens, generated.OutputTokens)
		task.Steps[index] = completedStep

		if stale() {
			return failStale()
		}
		persistCtx := context.WithoutCancel(ctx)
		if err := store.UpdateStep(persistCtx, completedStep); err != nil {
			return ExtractionRunView{}, err
		}
		if err := store.UpdateTask(persistCtx, task); err != nil {
			return ExtractionRunView{}, err
		}
		if err := store.CreateArtifact(persistCtx, newExtractionArtifact(
			s.ids.NewID(), task.ID, completedStep.ID, "model_response", generated.Text, s.clock.Now(),
		)); err != nil {
			return ExtractionRunView{}, err
		}
		if err := ctx.Err(); err != nil {
			return failed(err)
		}
		if task.RemainingTokens() == 0 && index < len(task.Steps)-1 {
			return failed(ErrTokenBudgetExceeded)
		}
	}

	// Completion: merge results, create change set, transition to Completed.
	if stale() {
		return failStale()
	}
	specs, err := mergeExtractionResults(task.Steps, s.ids.NewID)
	if err != nil {
		return failed(err)
	}
	if len(specs) == 0 {
		task, _ = task.Transition(extractiondomain.StatusFailed, "抽取未产生任何候选", s.clock.Now())
		_ = store.UpdateTask(context.WithoutCancel(ctx), task)
		return extractionRunView(task, nil), fmt.Errorf("extraction produced no candidates")
	}

	changeSet, err := revisiondomain.NewChangeSet(
		s.ids.NewID(), project.ID,
		"导入抽取候选", "由分章抽取合并产生，等待作者逐项确认。",
		"extraction-task:"+task.ID, specs, s.clock.Now(),
	)
	if err != nil {
		return failed(err)
	}
	if err := projectStore.CreateChangeSet(ctx, changeSet); err != nil {
		return failed(err)
	}
	task.ChangeSetID = changeSet.ID
	task, err = task.Transition(extractiondomain.StatusCompleted, "", s.clock.Now())
	if err != nil {
		return failed(err)
	}
	if err := store.UpdateTask(context.WithoutCancel(ctx), task); err != nil {
		return ExtractionRunView{}, err
	}
	completedJob, err := currentJob.MarkExtractionCompleted(s.clock.Now())
	if err != nil {
		return ExtractionRunView{}, err
	}
	if err := importStore.UpdateImportJobExtraction(context.WithoutCancel(ctx), completedJob); err != nil {
		return ExtractionRunView{}, stableImportStoreError(err)
	}

	changeSetView := changeSetView(changeSet)
	return extractionRunView(task, &changeSetView), nil
}

func (s *Service) RetryExtractionRun(ctx context.Context, id string) (ExtractionRunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		return ExtractionRunView{}, err
	}
	original, err := store.Task(ctx, strings.TrimSpace(id))
	if err != nil {
		return ExtractionRunView{}, err
	}
	switch original.Status {
	case extractiondomain.StatusFailed, extractiondomain.StatusCancelled, extractiondomain.StatusRecoverable:
	default:
		return ExtractionRunView{}, fmt.Errorf("extraction task cannot be retried from status %s", original.Status)
	}
	stepSpecs := make([]extractiondomain.StepSpec, 0, len(original.Steps))
	for _, step := range original.Steps {
		chapter, err := s.active.store.Chapter(ctx, step.CreatedChapterID)
		if err != nil {
			return ExtractionRunView{}, fmt.Errorf("read chapter for retry: %w", err)
		}
		stepSpecs = append(stepSpecs, extractiondomain.StepSpec{
			ID:               s.ids.NewID(),
			ImportChapterID:  step.ImportChapterID,
			CreatedChapterID: step.CreatedChapterID,
			ChapterTitle:     step.ChapterTitle,
			SourceStartLine:  step.SourceStartLine,
			SourceEndLine:    step.SourceEndLine,
			ChapterBody:      chapter.Body,
		})
	}
	task, err := extractiondomain.NewTask(
		s.ids.NewID(), original.ProjectID, original.ImportJobID,
		original.ConnectionID, original.Model, original.Prompt,
		stepSpecs,
		original.TokenBudget, original.ContextCharBudget,
		s.clock.Now(),
	)
	if err != nil {
		return ExtractionRunView{}, err
	}
	task.RetryOf = original.ID
	if err := store.CreateTask(ctx, task); err != nil {
		return ExtractionRunView{}, fmt.Errorf("create retry extraction task: %w", err)
	}
	return extractionRunView(task, nil), nil
}

func (s *Service) CancelExtractionRun(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.extractionJobsMu.Lock()
	cancel := s.extractionJobs[id]
	s.extractionJobsMu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		return err
	}
	task, err := store.Task(ctx, id)
	if err != nil {
		return err
	}
	if task.Status != extractiondomain.StatusQueued && task.Status != extractiondomain.StatusRecoverable {
		return fmt.Errorf("extraction task cannot be cancelled from status %s", task.Status)
	}
	return s.persistCancelledExtraction(ctx, store, task)
}

func (s *Service) ExtractionTasks(ctx context.Context) ([]ExtractionTaskView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		return nil, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := store.ListTasks(ctx, project.ID, 50)
	if err != nil {
		return nil, err
	}
	views := make([]ExtractionTaskView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, extractionTaskView(task))
	}
	return views, nil
}

func (s *Service) ExtractionTask(ctx context.Context, id string) (ExtractionRunView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.extractionStoreLocked()
	if err != nil {
		return ExtractionRunView{}, err
	}
	task, err := store.Task(ctx, strings.TrimSpace(id))
	if err != nil {
		return ExtractionRunView{}, err
	}
	var csView *ChangeSetView
	if task.ChangeSetID != "" {
		changeSet, err := s.active.store.ChangeSet(ctx, task.ChangeSetID)
		if err == nil {
			view := changeSetView(changeSet)
			csView = &view
		}
	}
	return extractionRunView(task, csView), nil
}

func (s *Service) extractionStoreLocked() (ports.ExtractionStore, error) {
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, ok := s.active.store.(ports.ExtractionStore)
	if !ok {
		return nil, ErrExtractionStoreUnavailable
	}
	return store, nil
}

func (s *Service) persistCancelledExtraction(
	ctx context.Context,
	store ports.ExtractionStore,
	task extractiondomain.Task,
) error {
	now := s.clock.Now().UTC()
	for index := range task.Steps {
		step := task.Steps[index]
		if step.Status == extractiondomain.StepCompleted ||
			step.Status == extractiondomain.StepFailed ||
			step.Status == extractiondomain.StepCancelled {
			continue
		}
		cancelled, _ := step.Transition(extractiondomain.StepCancelled, "作者已取消抽取任务。", now)
		task.Steps[index] = cancelled
		_ = store.UpdateStep(ctx, cancelled)
	}
	task.ErrorCode = "cancelled"
	cancelled, err := task.Transition(extractiondomain.StatusCancelled, "作者已取消抽取任务。", now)
	if err != nil {
		return err
	}
	return store.UpdateTask(ctx, cancelled)
}

func (s *Service) persistCancelledExtractionByID(
	ctx context.Context,
	store ports.ExtractionStore,
	taskID string,
) (ExtractionRunView, error) {
	task, err := store.Task(ctx, taskID)
	if err != nil {
		return ExtractionRunView{}, err
	}
	if task.Status == extractiondomain.StatusCancelled {
		return extractionRunView(task, nil), nil
	}
	if err := s.persistCancelledExtraction(ctx, store, task); err != nil {
		return ExtractionRunView{}, err
	}
	task, err = store.Task(ctx, taskID)
	if err != nil {
		return ExtractionRunView{}, err
	}
	return extractionRunView(task, nil), nil
}

func assembleExtractionContext(step extractiondomain.Step, instruction, projectID string) (string, error) {
	manifest := map[string]any{
		"contract": map[string]any{
			"extractAssetsOnly":         true,
			"preserveSourceLineRange":   true,
			"referenceCharactersByName": true,
		},
		"projectId": projectID,
		"chapter": map[string]any{
			"id":              step.CreatedChapterID,
			"title":           step.ChapterTitle,
			"sourceStartLine": step.SourceStartLine,
			"sourceEndLine":   step.SourceEndLine,
			"body":            step.ChapterBody,
		},
		"authorInstruction": instruction,
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("encode extraction context: %w", err)
	}
	return string(encoded), nil
}

func newExtractionArtifact(id, taskID, stepID, kind, content string, now time.Time) ports.ExtractionArtifact {
	sum := sha256.Sum256([]byte(content))
	return ports.ExtractionArtifact{
		ID: id, TaskID: taskID, StepID: stepID, Kind: kind, Content: content,
		SHA256: hex.EncodeToString(sum[:]), CreatedAt: now.UTC(),
	}
}

func extractionRunView(task extractiondomain.Task, changeSet *ChangeSetView) ExtractionRunView {
	return ExtractionRunView{
		Task:      extractionTaskView(task),
		ChangeSet: changeSet,
	}
}

func extractionTaskView(task extractiondomain.Task) ExtractionTaskView {
	steps := make([]ExtractionStepView, 0, len(task.Steps))
	for _, step := range task.Steps {
		steps = append(steps, ExtractionStepView{
			ID: step.ID, ImportChapterID: step.ImportChapterID,
			CreatedChapterID: step.CreatedChapterID, ChapterTitle: step.ChapterTitle,
			Position: step.Position, Status: string(step.Status), Output: step.Output,
			ErrorCode: step.ErrorCode, ErrorMessage: step.ErrorMessage,
			ProviderRequestID: step.ProviderRequestID,
			InputTokens:       step.InputTokens, OutputTokens: step.OutputTokens,
		})
	}
	return ExtractionTaskView{
		ID: task.ID, ImportJobID: task.ImportJobID, ConnectionID: task.ConnectionID,
		Model: task.Model, Prompt: task.Prompt, Status: string(task.Status),
		ErrorCode: task.ErrorCode, ErrorMessage: task.ErrorMessage,
		ChangeSetID: task.ChangeSetID, RetryOf: task.RetryOf,
		TokenBudget: task.TokenBudget, ContextCharBudget: task.ContextCharBudget,
		InputTokens: task.InputTokens, OutputTokens: task.OutputTokens,
		Steps:     steps,
		CreatedAt: task.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: task.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

const extractionSystemPrompt = `你是长篇小说故事档案抽取智能体。从给定章节正文中抽取大纲、人物、关系、世界规则、事件、事实和伏笔候选。关系通过人物名称引用（sourceName/targetName），伏笔通过章节标题引用（plantedChapterTitle）。只返回符合 JSON Schema 的结果，不要解释。`
