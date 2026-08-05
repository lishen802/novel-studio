package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestExtractionStorePersistsTaskStepsAndArtifacts(t *testing.T) {
	ctx := context.Background()
	store, project, importJobID, chapterID := createExtractionTestStore(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()

	task, err := extractiondomain.NewTask(
		"task-1", project.ID, importJobID,
		"connection-1", "gpt-4", "提取故事档案",
		[]extractiondomain.StepSpec{
			{
				ID: "step-1", ImportChapterID: "ic-1", CreatedChapterID: chapterID,
				ChapterTitle: "第一章 风起", SourceStartLine: 1, SourceEndLine: 80, ChapterBody: "正文一",
			},
			{
				ID: "step-2", ImportChapterID: "ic-2", CreatedChapterID: "chapter-2",
				ChapterTitle: "第二章 暗流", SourceStartLine: 81, SourceEndLine: 160, ChapterBody: "正文二",
			},
		}, 60000, 120000, now,
	)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	if err := store.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Running the first step stamps provider usage.
	runningStep, err := task.Steps[0].Transition(extractiondomain.StepRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("step transition: %v", err)
	}
	runningStep.Output = `{"characters":[]}`
	runningStep.ProviderRequestID = "req-1"
	runningStep.InputTokens = 120
	runningStep.OutputTokens = 60
	completedStep, err := runningStep.Transition(extractiondomain.StepCompleted, "", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("step complete: %v", err)
	}
	completedStep.Output = runningStep.Output
	completedStep.ProviderRequestID = runningStep.ProviderRequestID
	completedStep.InputTokens = runningStep.InputTokens
	completedStep.OutputTokens = runningStep.OutputTokens
	if err := store.UpdateStep(ctx, completedStep); err != nil {
		t.Fatalf("UpdateStep: %v", err)
	}

	// Transition the task to running and persist usage.
	runningTask, err := task.Transition(extractiondomain.StatusRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("task running: %v", err)
	}
	runningTask = runningTask.RecordUsage(completedStep.InputTokens, completedStep.OutputTokens)
	if err := store.UpdateTask(ctx, runningTask); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	// Persist an audit artifact (context manifest snapshot).
	artifact := ports.ExtractionArtifact{
		ID: "artifact-1", TaskID: task.ID, StepID: completedStep.ID,
		Kind: "context_manifest", Content: `{"chapter":"第一章"}`, SHA256: "hash-1",
		CreatedAt: now.Add(time.Minute),
	}
	if err := store.CreateArtifact(ctx, artifact); err != nil {
		t.Fatalf("CreateArtifact: %v", err)
	}

	// Reload and verify the persisted shape.
	loaded, err := store.Task(ctx, task.ID)
	if err != nil {
		t.Fatalf("Task: %v", err)
	}
	if loaded.Status != extractiondomain.StatusRunning || loaded.InputTokens != 120 || loaded.OutputTokens != 60 {
		t.Fatalf("loaded task = %#v", loaded)
	}
	if loaded.StartedAt == nil || !loaded.StartedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("loaded started_at = %#v", loaded.StartedAt)
	}
	if len(loaded.Steps) != 2 {
		t.Fatalf("loaded steps = %d, want 2", len(loaded.Steps))
	}
	if loaded.Steps[0].Status != extractiondomain.StepCompleted ||
		loaded.Steps[0].Output != `{"characters":[]}` ||
		loaded.Steps[0].ProviderRequestID != "req-1" ||
		loaded.Steps[0].InputTokens != 120 {
		t.Fatalf("loaded step-1 = %#v", loaded.Steps[0])
	}
	if loaded.Steps[1].Status != extractiondomain.StepQueued {
		t.Fatalf("loaded step-2 status = %q, want queued", loaded.Steps[1].Status)
	}

	artifacts, err := store.ListArtifacts(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].Content != artifact.Content {
		t.Fatalf("artifacts = %#v", artifacts)
	}
}

func TestExtractionStoreListsTasksByProjectAndImport(t *testing.T) {
	ctx := context.Background()
	store, project, importJobID, chapterID := createExtractionTestStore(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()

	taskSpecs := []struct {
		id  string
		age time.Duration
	}{
		{id: "task-older", age: time.Minute},
		{id: "task-newer", age: 2 * time.Minute},
	}
	for _, spec := range taskSpecs {
		task, err := extractiondomain.NewTask(
			spec.id, project.ID, importJobID,
			"connection-1", "gpt-4", "",
			[]extractiondomain.StepSpec{{
				ID: spec.id + "-step", ImportChapterID: "ic-1", CreatedChapterID: chapterID,
				ChapterTitle: "第一章", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "正文",
			}}, 1000, 2000, now.Add(spec.age),
		)
		if err != nil {
			t.Fatalf("NewTask %s: %v", spec.id, err)
		}
		if err := store.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask %s: %v", spec.id, err)
		}
	}

	byProject, err := store.ListTasks(ctx, project.ID, 10)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(byProject) != 2 || byProject[0].ID != "task-newer" || byProject[1].ID != "task-older" {
		t.Fatalf("byProject order = %#v", byProject)
	}

	byImport, err := store.TasksForImport(ctx, importJobID)
	if err != nil {
		t.Fatalf("TasksForImport: %v", err)
	}
	if len(byImport) != 2 || byImport[0].ID != "task-newer" {
		t.Fatalf("byImport order = %#v", byImport)
	}
}

func TestExtractionStoreTaskMissingReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	store, _, _, _ := createExtractionTestStore(t)
	defer store.Close()
	if _, err := store.Task(ctx, "missing"); !errors.Is(err, ports.ErrExtractionTaskMissing) {
		t.Fatalf("Task(missing) error = %v, want ErrExtractionTaskMissing", err)
	}
}

func TestExtractionStoreRecoversInterruptedTasks(t *testing.T) {
	ctx := context.Background()
	store, project, importJobID, chapterID := createExtractionTestStore(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()

	task, err := extractiondomain.NewTask(
		"task-1", project.ID, importJobID,
		"connection-1", "gpt-4", "",
		[]extractiondomain.StepSpec{{
			ID: "step-1", ImportChapterID: "ic-1", CreatedChapterID: chapterID,
			ChapterTitle: "第一章", SourceStartLine: 1, SourceEndLine: 2, ChapterBody: "正文",
		}}, 1000, 2000, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	running, err := task.Transition(extractiondomain.StatusRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTask(ctx, running); err != nil {
		t.Fatal(err)
	}
	runningStep, err := task.Steps[0].Transition(extractiondomain.StepRunning, "", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateStep(ctx, runningStep); err != nil {
		t.Fatal(err)
	}

	recoveredAt := now.Add(2 * time.Minute)
	count, err := store.RecoverInterruptedTasks(ctx, recoveredAt)
	if err != nil {
		t.Fatalf("RecoverInterruptedTasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("recovered count = %d, want 1", count)
	}
	loaded, err := store.Task(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != extractiondomain.StatusRecoverable {
		t.Fatalf("recovered task status = %q, want recoverable", loaded.Status)
	}
	if loaded.Steps[0].Status != extractiondomain.StepFailed {
		t.Fatalf("recovered step status = %q, want failed", loaded.Steps[0].Status)
	}
}

func TestExtractionStoreUpdateTaskRejectsMissingTask(t *testing.T) {
	ctx := context.Background()
	store, _, _, _ := createExtractionTestStore(t)
	defer store.Close()
	missing := extractiondomain.Task{
		ID: "missing", ProjectID: "project-1", ImportJobID: "import-1",
		Status: extractiondomain.StatusRunning, UpdatedAt: time.Unix(100, 0),
	}
	if err := store.UpdateTask(ctx, missing); err == nil {
		t.Fatal("UpdateTask on missing task should fail")
	}
}

func createExtractionTestStore(t *testing.T) (*Store, projectdomain.Project, string, string) {
	t.Helper()
	store, project := createTestStore(t)
	ctx := context.Background()
	// Build chapters whose titles/bodies mirror the import candidates so
	// ApplyImportJob's mapping validation passes; ApplyImportJob persists them.
	chapter1 := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter1.Body = "正文"
	chapter2 := mustChapter(t, "chapter-2", project.ID, "第二章", 2)
	chapter2.Body = "正文二"
	now := time.Unix(50, 0).UTC()
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	job, err := importing.NewImportJob(importing.ImportJobParams{
		ID:                   "import-job-1",
		ProjectID:            project.ID,
		SourceName:           "novel.txt",
		SourceFormat:         importing.SourceFormatTXT,
		SourceEncoding:       "UTF-8",
		SourceHash:           hash,
		SourceBytes:          1024,
		ArchivedRelativePath: "imports/novel.txt",
		ChapterCount:         2,
	}, now)
	if err != nil {
		store.Close()
		t.Fatalf("create import job: %v", err)
	}
	ic1, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID: "ic-1", JobID: job.ID, Title: "第一章", Body: "正文",
		Position: 1, SourceStartLine: 1, SourceEndLine: 2,
	})
	if err != nil {
		store.Close()
		t.Fatalf("create import chapter 1: %v", err)
	}
	ic2, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID: "ic-2", JobID: job.ID, Title: "第二章", Body: "正文二",
		Position: 2, SourceStartLine: 3, SourceEndLine: 4,
	})
	if err != nil {
		store.Close()
		t.Fatalf("create import chapter 2: %v", err)
	}
	if err := store.CreateImportJob(ctx, job, []importing.ImportChapter{ic1, ic2}); err != nil {
		store.Close()
		t.Fatalf("persist import job: %v", err)
	}
	// Mark applied so the import job is in the lifecycle state extraction requires.
	appliedJob, err := job.MarkApplied(2, now.Add(time.Minute))
	if err != nil {
		store.Close()
		t.Fatalf("mark applied: %v", err)
	}
	chapters := []projectdomain.Chapter{chapter1, chapter2}
	if _, err := store.ApplyImportJob(
		ctx, appliedJob.ID,
		[]string{"ic-1", "ic-2"}, chapters, now.Add(time.Minute),
	); err != nil {
		store.Close()
		t.Fatalf("apply import job: %v", err)
	}
	return store, project, job.ID, chapter1.ID
}
