package harnesssqlite

import (
	"context"
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
	for _, table := range []string{"harness_runs", "harness_case_results", "harness_ratings", "harness_reports"} {
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
	if err != nil || loadedRun.Binding.WorkflowSHA256 != run.Binding.WorkflowSHA256 || loadedRun.DatasetSHA256 != run.DatasetSHA256 {
		t.Fatalf("frozen run = %+v, err = %v", loadedRun, err)
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
