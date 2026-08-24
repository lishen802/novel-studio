package quality

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBuiltinHarnessDatasetV1IsOriginalVersionedAndComplete(t *testing.T) {
	dataset, err := BuiltinHarnessDatasetV1()
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	if dataset.ID != "creative-quality" || dataset.Version != "1.0.0" || dataset.SHA256 == "" {
		t.Fatalf("unexpected dataset identity: %+v", dataset)
	}
	if len(dataset.Cases) != 12 {
		t.Fatalf("case count = %d, want 12", len(dataset.Cases))
	}

	blueprints, chapters, reduced := 0, 0, 0
	genres := map[string]bool{}
	for _, item := range dataset.Cases {
		if item.Provenance != ProvenanceOriginalFixture {
			t.Fatalf("case %s provenance = %q", item.ID, item.Provenance)
		}
		if item.DatasetVersion != dataset.Version {
			t.Fatalf("case %s version = %q", item.ID, item.DatasetVersion)
		}
		genres[item.Genre] = true
		switch item.TaskKind {
		case HarnessTaskBlueprint:
			blueprints++
		case HarnessTaskChapter:
			chapters++
		}
		if item.CIReduced {
			reduced++
		}
	}
	if blueprints != 4 || chapters != 8 || reduced < 3 {
		t.Fatalf("blueprints=%d chapters=%d reduced=%d", blueprints, chapters, reduced)
	}
	for _, genre := range []string{"东方幻想", "都市悬疑", "现实情感", "轻科幻"} {
		if !genres[genre] {
			t.Fatalf("missing genre %q", genre)
		}
	}
}

func TestValidateHarnessDatasetRejectsChangedContentWithoutVersionHash(t *testing.T) {
	dataset, err := BuiltinHarnessDatasetV1()
	if err != nil {
		t.Fatal(err)
	}
	dataset.Cases[0].StorySpec += "篡改"
	if err := dataset.Validate(); !errors.Is(err, ErrDatasetHashMismatch) {
		t.Fatalf("validate error = %v, want hash mismatch", err)
	}
}

func TestAnonymousCandidatesAreDeterministicAndHideIdentity(t *testing.T) {
	candidates := []HarnessCandidate{
		{Kind: CandidateBaseline, Content: "第一份正文"},
		{Kind: CandidateInitial, Content: "第二份正文"},
		{Kind: CandidateRevised, Content: "第三份正文"},
	}
	one, err := AnonymizeCandidates("chapter-hook", 417, candidates)
	if err != nil {
		t.Fatal(err)
	}
	two, err := AnonymizeCandidates("chapter-hook", 417, candidates)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(one)
	right, _ := json.Marshal(two)
	if string(left) != string(right) {
		t.Fatalf("anonymous projections differ:\n%s\n%s", left, right)
	}
	for _, candidate := range one {
		if candidate.Slot == "" || candidate.Content == "" {
			t.Fatalf("invalid candidate: %+v", candidate)
		}
	}
	encoded := string(left)
	for _, forbidden := range []string{"baseline", "initial", "revised", "provider", "workflow"} {
		if containsFold(encoded, forbidden) {
			t.Fatalf("blind projection leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestHarnessRunnerRunsReducedSetAndBuildsBreakdowns(t *testing.T) {
	dataset, err := BuiltinHarnessDatasetV1()
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryHarnessStore{}
	runner, err := NewHarnessRunner(HarnessRunnerConfig{
		Store:    store,
		Executor: deterministicExecutor{},
		Now:      func() time.Time { return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC) },
		NewID:    func() string { return "run-001" },
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), HarnessRunRequest{
		Dataset: dataset,
		Mode:    HarnessModeReduced,
		Seed:    417,
		Binding: testHarnessBinding(),
	})
	if err != nil {
		t.Fatalf("run harness: %v", err)
	}
	if report.RunID != "run-001" || report.CaseCount < 3 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.ByGenre) < 2 || len(report.ByTask) != 2 {
		t.Fatalf("report must preserve genre/task breakdowns: %+v", report)
	}
	if len(report.ByCandidate) != 3 || len(report.Cases) != report.CaseCount {
		t.Fatalf("report must preserve candidate/case evidence: %+v", report)
	}
	if len(store.results) != report.CaseCount || store.run.DatasetSHA256 != dataset.SHA256 {
		t.Fatalf("run was not frozen or fully persisted: %+v", store.run)
	}
	exported, err := ExportHarnessReport(report)
	if err != nil || !json.Valid(exported) {
		t.Fatalf("export report: valid=%v err=%v", json.Valid(exported), err)
	}
}

func TestHarnessRunnerPersistsCancellationAndResumesWithoutRepeatingCompletedCase(t *testing.T) {
	dataset, err := BuiltinHarnessDatasetV1()
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryHarnessStore{}
	executor := &interruptOnceExecutor{}
	runner, err := NewHarnessRunner(HarnessRunnerConfig{Store: store, Executor: executor, Now: time.Now, NewID: func() string { return "run-resume" }})
	if err != nil {
		t.Fatal(err)
	}
	run, err := runner.Create(context.Background(), HarnessRunRequest{Dataset: dataset, Mode: HarnessModeReduced, Seed: 7, Binding: testHarnessBinding()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Execute(context.Background(), run.ID, dataset); !errors.Is(err, context.Canceled) {
		t.Fatalf("first execute error = %v", err)
	}
	if store.run.Status != HarnessRunCancelled || len(store.results) != 1 {
		t.Fatalf("cancelled progress = %+v, results=%d", store.run, len(store.results))
	}
	firstCaseCalls := executor.calls[store.results[0].CaseID]
	if _, err := runner.Execute(context.Background(), run.ID, dataset); err != nil {
		t.Fatal(err)
	}
	if store.run.Status != HarnessRunCompleted || executor.calls[store.results[0].CaseID] != firstCaseCalls {
		t.Fatalf("resume repeated completed case: status=%s calls=%v", store.run.Status, executor.calls)
	}
}

func TestHarnessRunnerPersistsPublicStageAndUsageProgress(t *testing.T) {
	dataset, err := BuiltinHarnessDatasetV1()
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryHarnessStore{}
	var updates []HarnessRun
	runner, err := NewHarnessRunner(HarnessRunnerConfig{Store: store, Executor: progressExecutor{}, Now: time.Now, NewID: func() string { return "run-progress" }, OnUpdated: func(_ context.Context, run HarnessRun) { updates = append(updates, run) }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), HarnessRunRequest{Dataset: dataset, Mode: HarnessModeReduced, Binding: testHarnessBinding()}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, update := range updates {
		if update.PublicStageID == "prose_writing" && update.CurrentCaseID != "" && update.InputTokens == 7 && update.OutputTokens == 5 && update.Calls == 1 {
			found = true
		}
	}
	if !found || store.run.PublicStageID != "candidate_ready" {
		t.Fatalf("progress updates = %+v, final = %+v", updates, store.run)
	}
}

func TestHarnessRatingSubmissionRequiresWholeBlindSet(t *testing.T) {
	submission := HarnessRatingSubmission{ID: "rating", RunID: "run", CaseID: "case", LocalRaterID: "local", CreatedAt: time.Now(), Ratings: []HarnessCandidateRating{{Slot: "候选-1", Scores: map[string]int{"人物动机": 4}, Acceptance: HarnessAcceptDirect}}}
	if err := submission.Validate([]string{"候选-1", "候选-2"}, []string{"人物动机"}); !errors.Is(err, ErrInvalidHarnessRun) {
		t.Fatalf("partial rating error = %v", err)
	}
	submission.Ratings = append(submission.Ratings, HarnessCandidateRating{Slot: "候选-2", Scores: map[string]int{"人物动机": 3}, Acceptance: HarnessAcceptMinor})
	if err := submission.Validate([]string{"候选-1", "候选-2"}, []string{"人物动机"}); err != nil {
		t.Fatal(err)
	}
}

func TestExportHarnessReportRejectsSecretShapedMetadata(t *testing.T) {
	report := HarnessReport{RunID: "run-1", DatasetID: "set", DatasetVersion: "1", GeneratedAt: time.Now().UTC(), Provider: "sk-live-secret"}
	if _, err := ExportHarnessReport(report); !errors.Is(err, ErrSensitiveReportData) {
		t.Fatalf("export error = %v", err)
	}
}

func TestCompareHarnessReportsDoesNotHideGenreRegression(t *testing.T) {
	base := HarnessReport{RunID: "base", DatasetID: "set", DatasetVersion: "1", ByGenre: []HarnessBreakdown{{Name: "东方幻想", PassRate: 1}, {Name: "都市悬疑", PassRate: .5}}}
	candidate := HarnessReport{RunID: "next", DatasetID: "set", DatasetVersion: "1", ByGenre: []HarnessBreakdown{{Name: "东方幻想", PassRate: .5}, {Name: "都市悬疑", PassRate: 1}}}
	comparison, err := CompareHarnessReports(base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(comparison.Regressions) != 1 || comparison.Regressions[0].Name != "东方幻想" {
		t.Fatalf("regressions = %+v", comparison.Regressions)
	}
}

type deterministicExecutor struct{}

func (deterministicExecutor) Execute(_ context.Context, item HarnessCase, _ HarnessRunBinding) ([]HarnessCandidate, error) {
	kinds := []HarnessCandidateKind{CandidateInitial, CandidateRevised}
	if item.TaskKind == HarnessTaskChapter {
		kinds = append([]HarnessCandidateKind{CandidateBaseline}, kinds...)
	}
	result := make([]HarnessCandidate, 0, len(kinds))
	for _, kind := range kinds {
		passed := !(item.Genre == "东方幻想" && kind == CandidateBaseline)
		result = append(result, HarnessCandidate{
			Kind: kind, Content: item.ID + "-" + string(kind),
			Metrics: HarnessCandidateMetrics{GatePassed: passed, InputTokens: 10, OutputTokens: 20, Calls: 1, DurationMillis: 5},
		})
	}
	return result, nil
}

type interruptOnceExecutor struct {
	interrupted bool
	calls       map[string]int
}

type progressExecutor struct{ deterministicExecutor }

func (progressExecutor) ExecuteWithProgress(ctx context.Context, item HarnessCase, binding HarnessRunBinding, onProgress func(HarnessExecutionProgress)) ([]HarnessCandidate, error) {
	onProgress(HarnessExecutionProgress{CaseID: item.ID, PublicStageID: "prose_writing", InputTokens: 7, OutputTokens: 5, Calls: 1})
	return deterministicExecutor{}.Execute(ctx, item, binding)
}

func (executor *interruptOnceExecutor) Execute(ctx context.Context, item HarnessCase, binding HarnessRunBinding) ([]HarnessCandidate, error) {
	if executor.calls == nil {
		executor.calls = map[string]int{}
	}
	executor.calls[item.ID]++
	if !executor.interrupted && len(executor.calls) > 1 {
		executor.interrupted = true
		return nil, context.Canceled
	}
	return deterministicExecutor{}.Execute(ctx, item, binding)
}

type memoryHarnessStore struct {
	run     HarnessRun
	results []HarnessCaseResult
	report  HarnessReport
}

func (store *memoryHarnessStore) CreateRun(_ context.Context, run HarnessRun) error {
	store.run = run
	return nil
}
func (store *memoryHarnessStore) UpdateRun(_ context.Context, run HarnessRun) error {
	store.run = run
	return nil
}
func (store *memoryHarnessStore) GetRun(_ context.Context, runID string) (HarnessRun, error) {
	if store.run.ID != runID {
		return HarnessRun{}, errors.New("run 不存在")
	}
	return store.run, nil
}
func (store *memoryHarnessStore) ListCaseResults(_ context.Context, runID string) ([]HarnessCaseResult, error) {
	if store.run.ID != runID {
		return nil, errors.New("run 不存在")
	}
	return append([]HarnessCaseResult(nil), store.results...), nil
}
func (store *memoryHarnessStore) SaveCaseResult(_ context.Context, result HarnessCaseResult) error {
	store.results = append(store.results, result)
	return nil
}
func (store *memoryHarnessStore) CompleteRun(_ context.Context, run HarnessRun, report HarnessReport) error {
	store.run = run
	store.report = report
	return nil
}

func testHarnessBinding() HarnessRunBinding {
	return HarnessRunBinding{
		ProfileID: "profile-1", ProfileVersion: 3, Provider: "openai", Model: "model-a",
		PromptVersion: "prompt-v1", PromptSHA256: testHash('a'), SchemaVersion: "schema-v1", SchemaSHA256: testHash('b'),
		WorkflowVersion: "workflow-v1", WorkflowSHA256: testHash('c'), AssemblerVersion: "assembler-v1", AssemblerSHA256: testHash('d'),
	}
}

func testHash(value byte) string {
	bytes := make([]byte, 64)
	for index := range bytes {
		bytes[index] = value
	}
	return string(bytes)
}
