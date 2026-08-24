package harnesssqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/quality"
)

func TestOpenCreatesIsolatedHarnessSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app", "harness.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open harness database: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"harness_runs", "harness_case_results", "harness_ratings", "harness_rating_submissions", "harness_reports"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d", table, count)
		}
	}
}

func TestStoreRoundTripBlindProjectionAndDeleteIsolation(t *testing.T) {
	directory := t.TempDir()
	projectPath := filepath.Join(directory, "project.db")
	if err := os.WriteFile(projectPath, []byte("project sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), filepath.Join(directory, "harness.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	started := time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)
	completed := started.Add(time.Minute)
	run := quality.HarnessRun{
		ID: "run-1", DatasetID: "set", DatasetVersion: "1.0.0", DatasetSHA256: hash('a'), Mode: quality.HarnessModeReduced,
		Seed: 23, Binding: binding(), CaseIDs: []string{"case-1"}, Status: quality.HarnessRunRunning, StartedAt: started,
		CurrentCaseID: "case-1", PublicStageID: "prose_writing", InputTokens: 12, OutputTokens: 8, Calls: 1,
	}
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	result := quality.HarnessCaseResult{
		RunID: "run-1", CaseID: "case-1", Genre: "东方幻想", TaskKind: quality.HarnessTaskChapter,
		Candidates: []quality.HarnessCandidate{
			{Kind: quality.CandidateBaseline, Content: "正文甲"},
			{Kind: quality.CandidateInitial, Content: "正文乙"},
			{Kind: quality.CandidateRevised, Content: "正文丙"},
		},
		AnonymousOrder: []string{"revised", "baseline", "initial"},
	}
	if err := store.SaveCaseResult(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	run.Status = quality.HarnessRunCompleted
	run.CompletedAt = &completed
	report := quality.HarnessReport{RunID: run.ID, DatasetID: run.DatasetID, DatasetVersion: run.DatasetVersion, DatasetSHA256: run.DatasetSHA256, GeneratedAt: completed}
	if err := store.CompleteRun(context.Background(), run, report); err != nil {
		t.Fatal(err)
	}

	blind, err := store.GetBlindCaseResult(context.Background(), "run-1", "case-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(blind) != 3 || blind[0].Content != "正文丙" || blind[0].Slot != "候选-1" {
		t.Fatalf("blind projection = %+v", blind)
	}
	loaded, err := store.GetReport(context.Background(), "run-1")
	if err != nil || loaded.DatasetSHA256 != run.DatasetSHA256 {
		t.Fatalf("report = %+v, err = %v", loaded, err)
	}
	loadedRun, err := store.GetRun(context.Background(), "run-1")
	if err != nil || loadedRun.Binding.WorkflowSHA256 != run.Binding.WorkflowSHA256 || loadedRun.DatasetSHA256 != run.DatasetSHA256 || loadedRun.PublicStageID != "prose_writing" || loadedRun.InputTokens != 12 {
		t.Fatalf("frozen run = %+v, err = %v", loadedRun, err)
	}
	runs, err := store.ListRuns(context.Background())
	if err != nil || len(runs) != 1 || runs[0].Status != quality.HarnessRunCompleted {
		t.Fatalf("runs = %+v, err = %v", runs, err)
	}
	results, err := store.ListCaseResults(context.Background(), run.ID)
	if err != nil || len(results) != 1 || results[0].CaseID != "case-1" {
		t.Fatalf("results = %+v, err = %v", results, err)
	}
	submission := quality.HarnessRatingSubmission{
		ID: "rating-1", RunID: run.ID, CaseID: "case-1", LocalRaterID: "local", CreatedAt: completed,
		Ratings: []quality.HarnessCandidateRating{{Slot: "候选-1", Scores: map[string]int{"叙事": 4}, Acceptance: quality.HarnessAcceptDirect}},
	}
	if err := store.SaveRatingSubmission(context.Background(), submission); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRatingSubmission(context.Background(), submission); !errors.Is(err, quality.ErrHarnessRatingExists) {
		t.Fatalf("duplicate rating error = %v", err)
	}
	loadedRating, err := store.GetRatingSubmission(context.Background(), run.ID, "case-1", "local")
	if err != nil || len(loadedRating.Ratings) != 1 {
		t.Fatalf("rating = %+v, err = %v", loadedRating, err)
	}
	ratings, err := store.ListRatingSubmissions(context.Background(), run.ID)
	if err != nil || len(ratings) != 1 || ratings[0].ID != submission.ID {
		t.Fatalf("ratings = %+v, err = %v", ratings, err)
	}

	if err := store.DeleteAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetReport(context.Background(), "run-1"); err == nil {
		t.Fatal("deleted report still exists")
	}
	payload, err := os.ReadFile(projectPath)
	if err != nil || string(payload) != "project sentinel" {
		t.Fatalf("project file changed: %q, %v", payload, err)
	}
}

func binding() quality.HarnessRunBinding {
	return quality.HarnessRunBinding{
		ProfileID: "profile", ProfileVersion: 1, Provider: "provider", Model: "model",
		PromptVersion: "p1", PromptSHA256: hash('b'), SchemaVersion: "s1", SchemaSHA256: hash('c'),
		WorkflowVersion: "w1", WorkflowSHA256: hash('d'), AssemblerVersion: "a1", AssemblerSHA256: hash('e'),
	}
}

func hash(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
