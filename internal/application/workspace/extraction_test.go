package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	importingdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

// TestExtractionRunProducesReviewableChangeSet exercises the full P4-2 flow:
// an applied ImportJob drives a chapter-by-chapter model extraction, the
// per-chapter JSON is merged into create_* operations with evidence, and the
// author can apply individual operations to materialize story assets.
func TestExtractionRunProducesReviewableChangeSet(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"extraction-model": `{
				"outline": [],
				"characters": [
					{"name":"林默","aliases":["墨老"],"role":"protagonist","summary":"沉默的修仙者","motivation":"夺回肉身","status":"active"}
				],
				"relationships": [],
				"worldRules": [
					{"name":"灵气浓度","description":"此地灵气稀薄","scopeType":"project"}
				],
				"storyEvents": [
					{"title":"入宗","summary":"林默拜入宗门","storyOrder":1,"narrativeOrder":1,"status":"occurred"}
				],
				"facts": [
					{"text":"林默曾是凡人","category":"background","status":"ai_inferred","confidence":0.8}
				],
				"foreshadowingHooks": [
					{"title":"暗门","setup":"宗门暗门后藏有名册","plantedChapterTitle":"第一章"}
				]
			}`,
		},
	}
	ids := &sequenceIDs{ids: []string{
		"project-1",
		"provider-1",
		"settings-1",
		"import-job-1", "import-chapter-1",
		"chapter-1",
		"extraction-task-1", "extraction-step-1",
		"artifact-context-1", "artifact-response-1",
		"change-set-1",
		"op-character-1", "op-worldrule-1", "op-storyevent-1", "op-fact-1", "op-hook-1",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(300, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "墨老夺舍")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Seed an applied ImportJob with one chapter so the extraction flow has a
	// real created chapter to read manuscript body from.
	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	importJob, importChapters := seedAppliedImportJob(t, store, project.ID, "import-job-1", "import-chapter-1", "第一章", "宗门暗门后藏有名册。")
	applied, err := service.ApplyImportJob(ctx, importJob.ID, []string{importChapters[0].ID})
	if err != nil {
		t.Fatalf("apply import job: %v", err)
	}
	if applied.Job.Status != "applied" || applied.Job.Chapters[0].CreatedChapterID == "" {
		t.Fatalf("apply result = %#v", applied)
	}

	// Start an extraction run bound to the applied import job.
	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  importJob.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
		Instruction:  "抽取故事档案候选。",
	})
	if err != nil {
		t.Fatalf("start extraction run: %v", err)
	}
	if run.Task.Status != "queued" || len(run.Task.Steps) != 1 {
		t.Fatalf("queued task = %#v", run.Task)
	}
	if run.Task.Steps[0].CreatedChapterID != applied.Job.Chapters[0].CreatedChapterID {
		t.Fatalf("step created chapter ID = %q, want %q",
			run.Task.Steps[0].CreatedChapterID, applied.Job.Chapters[0].CreatedChapterID)
	}

	// Execute the run: fake provider returns one candidate per asset kind.
	// ExecuteExtractionRun returns immediately with a running view; the
	// provider pipeline runs in a background goroutine, so wait for the
	// terminal status and reload the task to inspect the produced change set.
	run, err = service.ExecuteExtractionRun(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("execute extraction run: %v", err)
	}
	waitForExtractionTaskStatus(t, service, run.Task.ID, "completed", 5*time.Second)
	run, err = service.ExtractionTask(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("reload extraction task: %v", err)
	}
	if run.Task.Status != "completed" {
		t.Fatalf("task status = %q, want completed", run.Task.Status)
	}
	if run.ChangeSet == nil {
		t.Fatalf("extraction did not produce a change set")
	}
	if len(run.ChangeSet.Operations) < 5 {
		t.Fatalf("change set has %d operations, want at least 5", len(run.ChangeSet.Operations))
	}

	// Verify evidence is attached and points back at the created chapter.
	var characterOpID string
	var characterEvidence []revisiondomain.Evidence
	for _, op := range run.ChangeSet.Operations {
		if op.Kind == string(revisiondomain.OperationCreateCharacter) {
			characterOpID = op.ID
			characterEvidence = op.Evidence
		}
		if len(op.Evidence) == 0 {
			t.Fatalf("operation %s (%s) has no evidence", op.ID, op.Kind)
		}
	}
	if characterOpID == "" {
		t.Fatalf("change set missing create_character operation")
	}
	if characterEvidence[0].ChapterID != applied.Job.Chapters[0].CreatedChapterID {
		t.Fatalf("character evidence chapter ID = %q, want %q",
			characterEvidence[0].ChapterID, applied.Job.Chapters[0].CreatedChapterID)
	}
	if characterEvidence[0].ChapterTitle != "第一章" {
		t.Fatalf("character evidence chapter title = %q", characterEvidence[0].ChapterTitle)
	}

	// Confirm the foreshadowing hook resolved plantedChapterTitle → plantedChapterId.
	var hookAfterValue string
	for _, op := range run.ChangeSet.Operations {
		if op.Kind == string(revisiondomain.OperationCreateForeshadowingHook) {
			hookAfterValue = op.AfterValue
		}
	}
	if hookAfterValue == "" {
		t.Fatalf("change set missing create_foreshadowing_hook operation")
	}
	if !strings.Contains(hookAfterValue, `"plantedChapterId"`) {
		t.Fatalf("hook after_value did not resolve plantedChapterId: %s", hookAfterValue)
	}
	if strings.Contains(hookAfterValue, "plantedChapterTitle") {
		t.Fatalf("hook after_value still carries plantedChapterTitle: %s", hookAfterValue)
	}

	// Apply the character operation and verify the asset lands in the store.
	appliedResult, err := service.ApplyChangeOperation(ctx, run.ChangeSet.ID, characterOpID)
	if err != nil {
		t.Fatalf("apply change operation: %v", err)
	}
	if appliedResult.ChangeSet.Status != string(revisiondomain.ChangeSetPartiallyApplied) {
		t.Fatalf("change set status = %q, want partially_applied", appliedResult.ChangeSet.Status)
	}
	storyWorld, err := service.StoryWorld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(storyWorld.Characters) != 1 {
		t.Fatalf("story world has %d characters, want 1", len(storyWorld.Characters))
	}
	if storyWorld.Characters[0].Name != "林默" {
		t.Fatalf("character name = %q", storyWorld.Characters[0].Name)
	}

	// Reject the world rule and confirm it never lands.
	var worldRuleOpID string
	for _, op := range run.ChangeSet.Operations {
		if op.Kind == string(revisiondomain.OperationCreateWorldRule) {
			worldRuleOpID = op.ID
		}
	}
	if worldRuleOpID == "" {
		t.Fatalf("change set missing create_world_rule operation")
	}
	if err := service.RejectChangeOperation(ctx, run.ChangeSet.ID, worldRuleOpID); err != nil {
		t.Fatalf("reject change operation: %v", err)
	}
	worldAssets, err := service.WorldAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(worldAssets.Rules) != 0 {
		t.Fatalf("world rules = %d, want 0", len(worldAssets.Rules))
	}

	// Verify the import job reflects completed extraction.
	jobView, err := service.ImportJob(ctx, importJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if jobView.ExtractionStatus != "completed" || jobView.ExtractionRunID != run.Task.ID {
		t.Fatalf("import job view = %#v", jobView)
	}
}

// TestExtractionRunMergesDuplicatesAcrossChapters verifies the cross-chapter
// merger deduplicates by name and accumulates evidence from each contributor.
func TestExtractionRunMergesDuplicatesAcrossChapters(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"extraction-model": `{
				"outline": [],
				"characters": [
					{"name":"林默","role":"protagonist","summary":"沉默的修仙者","status":"active"}
				],
				"relationships": [],
				"worldRules": [],
				"storyEvents": [],
				"facts": [],
				"foreshadowingHooks": []
			}`,
		},
	}
	ids := &sequenceIDs{ids: []string{
		"project-2",
		"provider-2",
		"settings-2",
		"import-job-2", "import-chapter-2", "import-chapter-3",
		"chapter-2", "chapter-3",
		"extraction-task-2", "extraction-step-2", "extraction-step-3",
		"artifact-context-2", "artifact-response-2",
		"artifact-context-3", "artifact-response-3",
		"change-set-2",
		"op-character-2",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(400, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "多章节合并")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}

	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	job, chapters := seedPreparedImportJob(t, store, project.ID, "import-job-2",
		[]importSeedChapter{
			{id: "import-chapter-2", title: "第一章", body: "墨老入宗。", position: 1},
			{id: "import-chapter-3", title: "第二章", body: "墨老悟道。", position: 2},
		})
	applied, err := service.ApplyImportJob(ctx, job.ID, []string{chapters[0].ID, chapters[1].ID})
	if err != nil {
		t.Fatalf("apply import job: %v", err)
	}
	if len(applied.Job.Chapters) != 2 {
		t.Fatalf("applied chapters = %d", len(applied.Job.Chapters))
	}

	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  job.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(run.Task.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(run.Task.Steps))
	}

	run, err = service.ExecuteExtractionRun(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	waitForExtractionTaskStatus(t, service, run.Task.ID, "completed", 5*time.Second)
	run, err = service.ExtractionTask(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("reload extraction task: %v", err)
	}
	if run.ChangeSet == nil {
		t.Fatalf("no change set produced")
	}
	var characterEvidence []revisiondomain.Evidence
	characterCount := 0
	for _, op := range run.ChangeSet.Operations {
		if op.Kind == string(revisiondomain.OperationCreateCharacter) {
			characterCount++
			characterEvidence = op.Evidence
		}
	}
	if characterCount != 1 {
		t.Fatalf("expected 1 character operation, got %d", characterCount)
	}
	if len(characterEvidence) != 2 {
		t.Fatalf("character evidence count = %d, want 2", len(characterEvidence))
	}
}

// TestExtractionRunRejectsNonAppliedImportJob ensures the extraction flow
// only starts on ImportJobs that have been applied.
func TestExtractionRunRejectsNonAppliedImportJob(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{outputs: map[string]string{}}
	ids := &sequenceIDs{ids: []string{"project-3", "provider-3", "settings-3", "import-job-3", "import-chapter-3"}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(500, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "未应用")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-c",
	})
	if err != nil {
		t.Fatal(err)
	}

	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	job, _ := seedPreparedImportJob(t, store, project.ID, "import-job-3",
		[]importSeedChapter{
			{id: "import-chapter-3", title: "第一章", body: "未应用章节。", position: 1},
		})
	if job.Status != importingdomain.JobStatusPrepared {
		t.Fatalf("job status = %q", job.Status)
	}

	_, err = service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  job.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
	})
	if err == nil {
		t.Fatalf("expected error starting extraction on prepared job")
	}
	if !strings.Contains(err.Error(), "not applied") {
		t.Fatalf("expected 'not applied' error, got %v", err)
	}
}

// TestCancelExtractionRunMarksTaskCancelled verifies that cancelling a queued
// task persists the cancelled status without executing model calls.
func TestCancelExtractionRunMarksTaskCancelled(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{outputs: map[string]string{}}
	ids := &sequenceIDs{ids: []string{
		"project-4", "provider-4", "settings-4",
		"import-job-4", "import-chapter-4", "chapter-4",
		"extraction-task-4", "extraction-step-4",
		"change-set-4", "op-1",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(600, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "取消")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-d",
	})
	if err != nil {
		t.Fatal(err)
	}

	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	job, chapters := seedAppliedImportJob(t, store, project.ID, "import-job-4", "import-chapter-4", "第一章", "可取消的章节。")
	applied, err := service.ApplyImportJob(ctx, job.ID, []string{chapters[0].ID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	_ = applied

	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  job.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := service.CancelExtractionRun(ctx, run.Task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	loaded, err := service.ExtractionTask(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Task.Status != "cancelled" {
		t.Fatalf("task status = %q, want cancelled", loaded.Task.Status)
	}
}

// TestRetryExtractionRunRequiresTerminalState ensures retry is only allowed
// from failed/cancelled/recoverable statuses.
func TestRetryExtractionRunRequiresTerminalState(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{outputs: map[string]string{}}
	ids := &sequenceIDs{ids: []string{
		"project-5", "provider-5", "settings-5",
		"import-job-5", "import-chapter-5", "chapter-5",
		"extraction-task-5", "extraction-step-5",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(700, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "重试")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-e",
	})
	if err != nil {
		t.Fatal(err)
	}

	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	job, chapters := seedAppliedImportJob(t, store, project.ID, "import-job-5", "import-chapter-5", "第一章", "可重试。")
	applied, err := service.ApplyImportJob(ctx, job.ID, []string{chapters[0].ID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	_ = applied

	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  job.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Task.Status != "queued" {
		t.Fatalf("status = %q", run.Task.Status)
	}

	// A queued task cannot be retried — it must transition to a terminal state first.
	_, err = service.RetryExtractionRun(ctx, run.Task.ID)
	if err == nil {
		t.Fatalf("expected retry to fail on queued task")
	}
	if !strings.Contains(err.Error(), "cannot be retried") {
		t.Fatalf("expected 'cannot be retried' error, got %v", err)
	}

	// Cancel it, then retry should succeed.
	if err := service.CancelExtractionRun(ctx, run.Task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	ids.ids = append(ids.ids, "extraction-task-5-retry", "extraction-step-5-retry")
	retried, err := service.RetryExtractionRun(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("retry after cancel: %v", err)
	}
	if retried.Task.RetryOf != run.Task.ID {
		t.Fatalf("retry_of = %q, want %q", retried.Task.RetryOf, run.Task.ID)
	}
	if retried.Task.Status != "queued" {
		t.Fatalf("retried status = %q", retried.Task.Status)
	}
}

// TestExtractionRunEditChangeOperation verifies an author can edit a
// candidate's after_value before applying it.
func TestExtractionRunEditChangeOperation(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"extraction-model": `{
				"outline": [],
				"characters": [
					{"name":"林默","role":"protagonist","summary":"原名未定","status":"active"}
				],
				"relationships": [],
				"worldRules": [],
				"storyEvents": [],
				"facts": [],
				"foreshadowingHooks": []
			}`,
		},
	}
	ids := &sequenceIDs{ids: []string{
		"project-6", "provider-6", "settings-6",
		"import-job-6", "import-chapter-6", "chapter-6",
		"extraction-task-6", "extraction-step-6",
		"artifact-context-6", "artifact-response-6",
		"change-set-6", "op-character-6",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(800, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "编辑候选")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	connection, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-f",
	})
	if err != nil {
		t.Fatal(err)
	}

	store, ok := service.active.store.(ports.ImportStore)
	if !ok {
		t.Fatal("active store does not implement ImportStore")
	}
	job, chapters := seedAppliedImportJob(t, store, project.ID, "import-job-6", "import-chapter-6", "第一章", "可编辑候选。")
	applied, err := service.ApplyImportJob(ctx, job.ID, []string{chapters[0].ID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	_ = applied

	run, err := service.StartExtractionRun(ctx, ExtractionRunInput{
		ImportJobID:  job.ID,
		ConnectionID: connection.ID,
		Model:        "extraction-model",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	run, err = service.ExecuteExtractionRun(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	waitForExtractionTaskStatus(t, service, run.Task.ID, "completed", 5*time.Second)
	run, err = service.ExtractionTask(ctx, run.Task.ID)
	if err != nil {
		t.Fatalf("reload extraction task: %v", err)
	}

	var characterOpID string
	var characterFound bool
	for _, op := range run.ChangeSet.Operations {
		if op.Kind == string(revisiondomain.OperationCreateCharacter) {
			characterOpID = op.ID
			characterFound = true
		}
	}
	if !characterFound {
		t.Fatalf("no create_character operation")
	}

	// Edit the character summary before applying.
	edited, err := service.EditChangeOperation(ctx, ChangeOperationEditInput{
		ChangeSetID: run.ChangeSet.ID,
		OperationID: characterOpID,
		Character: &CharacterInput{
			Name:    "林默",
			Role:    "protagonist",
			Summary: "编辑后的简介",
			Status:  "active",
		},
	})
	if err != nil {
		t.Fatalf("edit change operation: %v", err)
	}
	var updatedAfterValue string
	for _, op := range edited.Operations {
		if op.ID == characterOpID {
			updatedAfterValue = op.AfterValue
		}
	}
	if updatedAfterValue == "" {
		t.Fatalf("edited operation not found")
	}
	if !strings.Contains(updatedAfterValue, "编辑后的简介") {
		t.Fatalf("after_value not updated: %s", updatedAfterValue)
	}

	// Apply the edited operation and verify the summary landed.
	if _, err := service.ApplyChangeOperation(ctx, run.ChangeSet.ID, characterOpID); err != nil {
		t.Fatalf("apply edited: %v", err)
	}
	storyWorld, err := service.StoryWorld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(storyWorld.Characters) != 1 || storyWorld.Characters[0].Summary != "编辑后的简介" {
		t.Fatalf("story world characters = %#v", storyWorld.Characters)
	}
}

// importSeedChapter is a fixture for seeding ImportChapters in tests.
type importSeedChapter struct {
	id       string
	title    string
	body     string
	position int
}

func seedPreparedImportJob(
	t *testing.T,
	store ports.ImportStore,
	projectID, jobID string,
	chapters []importSeedChapter,
) (importingdomain.ImportJob, []importingdomain.ImportChapter) {
	t.Helper()
	now := time.Unix(200, 0)
	// Deterministic 64-char SHA256 for the source hash.
	digest := sha256.Sum256([]byte(jobID + projectID))
	sourceHash := hex.EncodeToString(digest[:])

	importChapters := make([]importingdomain.ImportChapter, 0, len(chapters))
	for _, seed := range chapters {
		chapter, err := importingdomain.NewImportChapter(importingdomain.ImportChapterParams{
			ID:              seed.id,
			JobID:           jobID,
			Title:           seed.title,
			Body:            seed.body,
			Position:        seed.position,
			SourceStartLine: seed.position * 2,
			SourceEndLine:   seed.position*2 + 1,
		})
		if err != nil {
			t.Fatalf("seed import chapter %s: %v", seed.id, err)
		}
		importChapters = append(importChapters, chapter)
	}
	job, err := importingdomain.NewImportJob(importingdomain.ImportJobParams{
		ID:                   jobID,
		ProjectID:            projectID,
		SourceName:           "novel.txt",
		SourceFormat:         importingdomain.SourceFormatTXT,
		SourceEncoding:       "utf-8",
		SourceHash:           sourceHash,
		SourceBytes:          1024,
		ArchivedRelativePath: "imports/novel.txt",
		ChapterCount:         len(importChapters),
	}, now)
	if err != nil {
		t.Fatalf("seed import job: %v", err)
	}
	if err := store.CreateImportJob(context.Background(), job, importChapters); err != nil {
		t.Fatalf("create import job: %v", err)
	}
	return job, importChapters
}

func seedAppliedImportJob(
	t *testing.T,
	store ports.ImportStore,
	projectID, jobID, chapterID, title, body string,
) (importingdomain.ImportJob, []importingdomain.ImportChapter) {
	t.Helper()
	job, chapters := seedPreparedImportJob(t, store, projectID, jobID,
		[]importSeedChapter{{id: chapterID, title: title, body: body, position: 1}})
	return job, chapters
}

// Compile-time guard: the fake provider must satisfy the ModelProvider port.
var _ ports.ModelProvider = (*fakeModelProvider)(nil)
var _ ports.ModelProviderFactory = (*fakeModelProviderFactory)(nil)
var _ agentdomain.ProviderConnection = agentdomain.ProviderConnection{}
