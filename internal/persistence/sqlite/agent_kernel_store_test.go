package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestAgentKernelBundlePersistsAndReconstructsAtomically(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(1_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	artifact := testAgentArtifact("artifact-input", run.ID, "", agentdomain.ArtifactTaskInput, "冻结任务输入", now)
	manifest := testContextManifest(run.ID, run.Steps[0].ID, now)
	finding := testAgentFinding(run.ID, run.Steps[0].ID, now)

	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{
		Run: run, Artifacts: []agentdomain.Artifact{artifact},
		Manifests: []agentdomain.ContextManifest{manifest},
		Findings:  []agentdomain.ReviewFindingRecord{finding},
	}); err != nil {
		t.Fatalf("CreateAgentRunBundle: %v", err)
	}
	snapshot, err := store.JobSnapshot(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.PublicProjection.StageID != agentdomain.PublicStageChapterDirecting || snapshot.PublicProjection.TotalSteps != len(run.Steps) {
		t.Fatalf("public projection = %+v", snapshot.PublicProjection)
	}

	loaded, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IsLegacy() || loaded.WorkflowBinding != run.WorkflowBinding || loaded.Steps[0].PromptBinding != run.Steps[0].PromptBinding {
		t.Fatalf("reconstructed run lost frozen bindings: %#v", loaded)
	}
	artifacts, err := store.ListAgentArtifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 1 || artifacts[0].Content != artifact.Content {
		t.Fatalf("artifacts = %#v, error %v", artifacts, err)
	}
	manifests, err := store.ListContextManifests(ctx, run.ID)
	if err != nil || len(manifests) != 1 || len(manifests[0].Items) != 1 {
		t.Fatalf("manifests = %#v, error %v", manifests, err)
	}
	findings, err := store.ListAgentReviewFindings(ctx, run.ID)
	if err != nil || len(findings) != 1 || findings[0].Message != finding.Message {
		t.Fatalf("findings = %#v, error %v", findings, err)
	}
}

func TestArtifactRelationsPreserveCandidateLineage(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(1_500, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	base := testAgentArtifact("artifact-base", run.ID, run.Steps[0].ID, agentdomain.ArtifactDraft, "原稿", now)
	revision := testAgentArtifact("artifact-revision", run.ID, run.Steps[0].ID, agentdomain.ArtifactRevision, "修订稿", now.Add(time.Second))
	relation := agentdomain.ArtifactRelationRecord{
		ArtifactID: revision.ID, Relation: agentdomain.ArtifactRevises,
		RelatedArtifactID: base.ID, BaseDigest: base.SHA256, CreatedAt: now.Add(time.Second),
	}
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run, Artifacts: []agentdomain.Artifact{base, revision}, Relations: []agentdomain.ArtifactRelationRecord{relation}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.ListAgentArtifactRelations(ctx, run.ID)
	if err != nil || len(loaded) != 1 || loaded[0].RelatedArtifactID != base.ID || loaded[0].BaseDigest != base.SHA256 {
		t.Fatalf("relations = %+v, err = %v", loaded, err)
	}
}

func TestAgentKernelStepBundleKeepsFullArtifactAndBoundedPreview(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(2_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run}); err != nil {
		t.Fatal(err)
	}

	content := strings.Repeat("正文段落。", 300)
	artifact := testAgentArtifact("artifact-draft", run.ID, run.Steps[0].ID, agentdomain.ArtifactDraft, content, now.Add(time.Second))
	step := run.Steps[0]
	step.Status = agentdomain.StepCompleted
	step.Output = content
	step.OutputDigest = artifact.SHA256
	step.PrimaryArtifactID = artifact.ID
	step.QualityStatus = agentdomain.QualityPassed
	step.UpdatedAt = now.Add(time.Second)
	if err := store.SaveAgentStepBundle(ctx, agentdomain.StepBundle{
		Step: step, Artifacts: []agentdomain.Artifact{artifact},
	}); err != nil {
		t.Fatalf("SaveAgentStepBundle: %v", err)
	}

	loaded, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len([]rune(loaded.Steps[0].Output)); got != agentdomain.MaxStepPreviewRunes {
		t.Fatalf("step preview runes = %d", got)
	}
	artifacts, err := store.ListAgentArtifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 1 || artifacts[0].Content != content {
		t.Fatalf("full artifact was not preserved: %#v, error %v", artifacts, err)
	}
}

func TestAppendAgentRunStepPreservesInterruptedAttempt(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(2_200, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run}); err != nil {
		t.Fatal(err)
	}
	interrupted := run.Steps[0]
	interrupted.Status = agentdomain.StepFailed
	interrupted.ErrorCode = "app_interrupted"
	interrupted.ErrorMessage = "应用退出导致步骤中断。"
	interrupted.QualityStatus = agentdomain.QualityBlocked
	completedAt := now.Add(time.Second)
	interrupted.CompletedAt = &completedAt
	interrupted.UpdatedAt = now.Add(time.Second)
	if err := store.UpdateAgentRunStep(ctx, interrupted); err != nil {
		t.Fatal(err)
	}
	next := interrupted
	next.ID = "step-attempt-2"
	next.Attempt = 2
	next.Status = agentdomain.StepQueued
	next.ErrorCode = ""
	next.ErrorMessage = ""
	next.QualityStatus = ""
	next.CompletedAt = nil
	next.CreatedAt = now.Add(2 * time.Second)
	next.UpdatedAt = next.CreatedAt
	if err := store.AppendAgentRunStep(ctx, next); err != nil {
		t.Fatalf("AppendAgentRunStep: %v", err)
	}
	loaded, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Steps) != len(run.Steps)+1 || loaded.Steps[0].Status != agentdomain.StepFailed || loaded.Steps[1].Attempt != 2 {
		t.Fatalf("attempt history = %#v", loaded.Steps)
	}
}

func TestLegacyAgentStepOutputRemainsLossless(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(2_500, 0).UTC()
	run, err := agentdomain.NewChapterDraftRun(
		"legacy-output-run", project.ID, chapter.ID, chapter.Revision, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "legacy-plan", Role: agentdomain.RolePlanner, ProfileID: "profile-plan"},
			{ID: "legacy-write", Role: agentdomain.RoleWriter, ProfileID: "profile-write"},
			{ID: "legacy-review", Role: agentdomain.RoleContinuityReviewer, ProfileID: "profile-review"},
		}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("旧运行正文。", 300)
	step := run.Steps[0]
	step.Output = content
	if err := store.UpdateAgentRunStep(ctx, step); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Steps[0].Output != content {
		t.Fatalf("legacy output was truncated: %d != %d", len([]rune(loaded.Steps[0].Output)), len([]rune(content)))
	}
}

func TestAgentKernelBundleRollsBackOnDuplicateAttempt(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(3_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	run.Steps[1].NodeID = run.Steps[0].NodeID
	run.Steps[1].Attempt = run.Steps[0].Attempt

	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run}); err == nil {
		t.Fatal("CreateAgentRunBundle succeeded with duplicate node attempt")
	}
	if _, err := store.AgentRun(ctx, run.ID); !errors.Is(err, ErrAgentRecordMissing) {
		t.Fatalf("run after rolled back transaction = %v", err)
	}
}

func TestAgentKernelBundleRejectsHashAndReferenceErrorsWithoutPartialWrite(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(4_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	artifact := testAgentArtifact("artifact-invalid", run.ID, "unknown-step", agentdomain.ArtifactDraft, "正文", now)
	artifact.SHA256 = strings.Repeat("0", 64)

	err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run, Artifacts: []agentdomain.Artifact{artifact}})
	if !errors.Is(err, agentdomain.ErrInvalidAgentPersistence) {
		t.Fatalf("CreateAgentRunBundle error = %v", err)
	}
	if _, err := store.AgentRun(ctx, run.ID); !errors.Is(err, ErrAgentRecordMissing) {
		t.Fatalf("run after rejected bundle = %v", err)
	}
}

func TestAgentKernelRecordsSurvivePortableDatabaseBackup(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	now := time.Unix(5_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	manifest := testContextManifest(run.ID, run.Steps[0].ID, now)
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{
		Run: run, Manifests: []agentdomain.ContextManifest{manifest},
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "portable-project.db")
	if err := store.Backup(ctx, target); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	opened, err := (Factory{}).Open(ctx, target)
	if err != nil {
		t.Fatalf("open portable backup: %v", err)
	}
	backup := opened.(*Store)
	defer backup.Close()
	manifests, err := backup.ListContextManifests(ctx, run.ID)
	if err != nil || len(manifests) != 1 || len(manifests[0].Items) != 1 {
		t.Fatalf("portable backup manifests = %#v, error %v", manifests, err)
	}
}

func TestAgentKernelSchemaRejectsInvalidHashesAndFindingReferences(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(6_000, 0).UTC()
	run := newR3PersistenceRun(t, project.ID, chapter.ID, chapter.Revision, now)
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE agent_runs SET workflow_hash = 'bad' WHERE id = ?`, run.ID); err == nil {
		t.Fatal("invalid workflow hash was accepted")
	}
	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO agent_review_findings(
			id, run_id, step_id, attempt, category, severity, status, source,
			message, constraint_ref, created_at, updated_at
		) VALUES ('bad-finding', ?, 'missing-step', 1, 'continuity', 'major', 'open',
			'model_review', '引用不存在的步骤', 'canon-1', ?, ?)
	`, run.ID, formatTime(now), formatTime(now)); err == nil {
		t.Fatal("finding with missing step reference was accepted")
	}
}

func newR3PersistenceRun(t *testing.T, projectID, chapterID string, revision int64, now time.Time) agentdomain.Run {
	t.Helper()
	run, err := agentdomain.NewChapterDraftRun(
		"run-r3", projectID, chapterID, revision, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "step-plan", Role: agentdomain.RolePlanner, ProfileID: "profile-plan"},
			{ID: "step-write", Role: agentdomain.RoleWriter, ProfileID: "profile-write"},
			{ID: "step-review", Role: agentdomain.RoleContinuityReviewer, ProfileID: "profile-review"},
		}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	hash := agentdomain.SHA256Hex([]byte("frozen"))
	run.TaskKind = agentdomain.TaskChapterDraft
	run.WorkflowBinding = agentdomain.VersionBinding{ID: "chapter-quality", Version: 1, Hash: hash}
	run.CanonicalHead = "revision-1"
	run.InputDigest = hash
	run.BudgetPolicyVersion = 1
	run.QualityPolicyVersion = 1
	run.MaxRevisionAttempts = 2
	capabilities := []agentdomain.Capability{
		agentdomain.CapabilityChapterPlanner,
		agentdomain.CapabilityProseWriter,
		agentdomain.CapabilityContinuityReview,
	}
	stages := []agentdomain.PublicStageID{
		agentdomain.PublicStageChapterDirecting,
		agentdomain.PublicStageProseWriting,
		agentdomain.PublicStageIndependentReview,
	}
	for index := range run.Steps {
		run.Steps[index].NodeID = "node-" + run.Steps[index].ID
		run.Steps[index].PublicStageID = stages[index]
		run.Steps[index].Capability = capabilities[index]
		run.Steps[index].Attempt = 1
		run.Steps[index].PromptBinding = agentdomain.VersionBinding{ID: "prompt-" + run.Steps[index].ID, Version: 1, Hash: hash}
		run.Steps[index].SchemaBinding = agentdomain.VersionBinding{ID: "schema-" + run.Steps[index].ID, Version: 1, Hash: hash}
		run.Steps[index].AssemblerID = "context-v1"
		run.Steps[index].AssemblerVersion = 1
		run.Steps[index].InputDigest = hash
	}
	return run
}

func testAgentArtifact(id, runID, stepID string, kind agentdomain.ArtifactKind, content string, now time.Time) agentdomain.Artifact {
	return agentdomain.Artifact{
		ID: id, RunID: runID, StepID: stepID, Kind: kind, Content: content,
		SHA256: agentdomain.SHA256Hex([]byte(content)), CreatedAt: now,
	}
}

func testContextManifest(runID, stepID string, now time.Time) agentdomain.ContextManifest {
	hash := agentdomain.SHA256Hex([]byte("上下文"))
	return agentdomain.ContextManifest{
		ID: "manifest-1", Version: 1, RunID: runID, StepID: stepID, Attempt: 1,
		CanonicalHead: "revision-1", AssemblerID: "context-v1", AssemblerVersion: 1,
		CharacterBudget: 10_000, EstimatedTokenBudget: 2_500, UsedChars: 3,
		AssembledContentHash: hash, CreatedAt: now,
		Items: []agentdomain.ContextManifestItem{{
			ManifestID: "manifest-1", SourceKind: agentdomain.ManifestStoryAsset,
			SourceID: "asset-1", SourceVersion: "1", ContentHash: hash,
			Position: 1, Priority: 100, SelectionReason: "章节必要设定", Included: true,
		}},
	}
}

func testAgentFinding(runID, stepID string, now time.Time) agentdomain.ReviewFindingRecord {
	return agentdomain.ReviewFindingRecord{
		ReviewFinding: agentdomain.ReviewFinding{
			ID: "finding-1", Category: agentdomain.FindingContinuity,
			Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen,
			Source: agentdomain.FindingModelReview, Message: "人物状态与上一章冲突",
			Suggestion: "按上一章定稿状态修订", ConstraintRef: "canon-1",
		},
		RunID: runID, StepID: stepID, Attempt: 1, CreatedAt: now, UpdatedAt: now,
	}
}
