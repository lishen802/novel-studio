package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidHarnessDataset = errors.New("invalid harness dataset")
	ErrDatasetHashMismatch   = errors.New("harness dataset hash mismatch")
	ErrInvalidHarnessRun     = errors.New("invalid harness run")
	ErrHarnessRatingExists   = errors.New("harness rating already exists")
	ErrSensitiveReportData   = errors.New("harness report contains sensitive data")
)

// HarnessExecutionError 携带可持久化的脱敏错误说明，同时保留内部错误链用于分类。
type HarnessExecutionError struct {
	PublicMessage string
	Cause         error
}

func (failure *HarnessExecutionError) Error() string {
	if failure == nil {
		return "harness execution failed"
	}
	return failure.PublicMessage
}

func (failure *HarnessExecutionError) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Cause
}

// NewHarnessExecutionError 创建只向运行记录暴露预先脱敏说明的执行错误。
func NewHarnessExecutionError(publicMessage string, cause error) error {
	message := strings.TrimSpace(publicMessage)
	if message == "" {
		message = "case 执行失败"
	}
	return &HarnessExecutionError{PublicMessage: message, Cause: cause}
}

const ProvenanceOriginalFixture = "project_original_fixture"

type HarnessTaskKind string

const (
	HarnessTaskBlueprint HarnessTaskKind = "blueprint"
	HarnessTaskChapter   HarnessTaskKind = "chapter"
)

type HarnessMode string

const (
	HarnessModeReduced HarnessMode = "reduced"
	HarnessModeFull    HarnessMode = "full"
)

type HarnessDataset struct {
	ID              string        `json:"dataset_id"`
	Version         string        `json:"version"`
	OriginalityNote string        `json:"originality_note"`
	Cases           []HarnessCase `json:"cases"`
	SHA256          string        `json:"sha256,omitempty"`
}

type HarnessCase struct {
	ID                    string          `json:"case_id"`
	DatasetVersion        string          `json:"dataset_version"`
	Genre                 string          `json:"genre"`
	TaskKind              HarnessTaskKind `json:"task_kind"`
	OriginalSeed          string          `json:"original_seed"`
	StorySpec             string          `json:"story_spec"`
	ConfirmedConstraints  []string        `json:"confirmed_constraints"`
	ChapterGoal           string          `json:"chapter_goal"`
	CharacterVoices       []string        `json:"character_voices"`
	RequiredAnchors       []string        `json:"required_anchors"`
	SoftAnchors           []string        `json:"soft_anchors"`
	ForbiddenItems        []string        `json:"forbidden_items"`
	TargetLength          int             `json:"target_length"`
	AutomaticExpectations []string        `json:"automatic_expectations"`
	RatingDimensions      []string        `json:"rating_dimensions"`
	Provenance            string          `json:"provenance"`
	CIReduced             bool            `json:"ci_reduced"`
}

// Validate 校验固定数据集结构，并用内容哈希防止在未提升版本时静默改写 fixture。
func (dataset HarnessDataset) Validate() error {
	if blankHarness(dataset.ID) || blankHarness(dataset.Version) || blankHarness(dataset.OriginalityNote) || len(dataset.Cases) < 12 {
		return ErrInvalidHarnessDataset
	}
	seen := make(map[string]struct{}, len(dataset.Cases))
	blueprints, chapters := 0, 0
	for _, item := range dataset.Cases {
		if blankHarness(item.ID) || item.DatasetVersion != dataset.Version || blankHarness(item.Genre) ||
			blankHarness(item.OriginalSeed) || blankHarness(item.StorySpec) || item.Provenance != ProvenanceOriginalFixture ||
			len(item.AutomaticExpectations) == 0 || len(item.RatingDimensions) == 0 {
			return ErrInvalidHarnessDataset
		}
		if _, exists := seen[item.ID]; exists {
			return ErrInvalidHarnessDataset
		}
		seen[item.ID] = struct{}{}
		switch item.TaskKind {
		case HarnessTaskBlueprint:
			blueprints++
		case HarnessTaskChapter:
			chapters++
			if blankHarness(item.ChapterGoal) || item.TargetLength < 1 {
				return ErrInvalidHarnessDataset
			}
		default:
			return ErrInvalidHarnessDataset
		}
	}
	if blueprints < 4 || chapters < 8 {
		return ErrInvalidHarnessDataset
	}
	want, err := harnessDatasetHash(dataset)
	if err != nil {
		return fmt.Errorf("hash harness dataset: %w", err)
	}
	if dataset.SHA256 == "" || dataset.SHA256 != want {
		return ErrDatasetHashMismatch
	}
	return nil
}

func harnessDatasetHash(dataset HarnessDataset) (string, error) {
	dataset.SHA256 = ""
	payload, err := json.Marshal(dataset)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

type HarnessCandidateKind string

const (
	CandidateBaseline HarnessCandidateKind = "baseline"
	CandidateInitial  HarnessCandidateKind = "initial"
	CandidateRevised  HarnessCandidateKind = "revised"
)

type HarnessCandidateMetrics struct {
	GatePassed                  bool  `json:"gate_passed"`
	Blockers                    int   `json:"blockers"`
	Majors                      int   `json:"majors"`
	Minors                      int   `json:"minors"`
	Resolved                    int   `json:"resolved"`
	InputTokens                 int   `json:"input_tokens"`
	OutputTokens                int   `json:"output_tokens"`
	Calls                       int   `json:"calls"`
	DurationMillis              int64 `json:"duration_millis"`
	Retries                     int   `json:"retries"`
	DirectorProtocolPassed      bool  `json:"director_protocol_passed"`
	ProfessionalConstraintsUsed int   `json:"professional_constraints_used"`
	ReviewIndependencePassed    bool  `json:"review_independence_passed"`
	MergeConflicts              int   `json:"merge_conflicts"`
	ClaimedResolved             int   `json:"claimed_resolved"`
	ConfirmedResolved           int   `json:"confirmed_resolved"`
	OverturnedResolutions       int   `json:"overturned_resolutions"`
	RevisionRegressions         int   `json:"revision_regressions"`
}

type HarnessCandidate struct {
	Kind              HarnessCandidateKind    `json:"kind"`
	Content           string                  `json:"content"`
	StructuredOutput  string                  `json:"structured_output,omitempty"`
	Metrics           HarnessCandidateMetrics `json:"metrics"`
	QualityReportJSON string                  `json:"quality_report_json,omitempty"`
}

type AnonymousHarnessCandidate struct {
	Slot              string                  `json:"slot"`
	Content           string                  `json:"content"`
	Metrics           HarnessCandidateMetrics `json:"metrics"`
	QualityReportJSON string                  `json:"quality_report_json,omitempty"`
}

// AnonymizeCandidates 由运行种子和 case ID 生成稳定匿名顺序，返回值不携带真实候选身份。
func AnonymizeCandidates(caseID string, seed int64, candidates []HarnessCandidate) ([]AnonymousHarnessCandidate, error) {
	ordered, err := anonymousCandidateOrder(caseID, seed, candidates)
	if err != nil {
		return nil, err
	}
	projection := make([]AnonymousHarnessCandidate, 0, len(ordered))
	for index, candidate := range ordered {
		projection = append(projection, AnonymousHarnessCandidate{
			Slot: fmt.Sprintf("候选-%d", index+1), Content: candidate.Content,
			QualityReportJSON: candidate.QualityReportJSON,
		})
	}
	return projection, nil
}

func anonymousCandidateOrder(caseID string, seed int64, candidates []HarnessCandidate) ([]HarnessCandidate, error) {
	if blankHarness(caseID) || len(candidates) < 2 {
		return nil, ErrInvalidHarnessRun
	}
	type ranked struct {
		candidate HarnessCandidate
		rank      string
	}
	rankedCandidates := make([]ranked, 0, len(candidates))
	for _, candidate := range candidates {
		if !validCandidateKind(candidate.Kind) || blankHarness(candidate.Content) {
			return nil, ErrInvalidHarnessRun
		}
		material := strconv.FormatInt(seed, 10) + "\x00" + caseID + "\x00" + string(candidate.Kind)
		sum := sha256.Sum256([]byte(material))
		rankedCandidates = append(rankedCandidates, ranked{candidate: candidate, rank: hex.EncodeToString(sum[:])})
	}
	sort.Slice(rankedCandidates, func(left, right int) bool { return rankedCandidates[left].rank < rankedCandidates[right].rank })
	ordered := make([]HarnessCandidate, 0, len(rankedCandidates))
	for _, item := range rankedCandidates {
		ordered = append(ordered, item.candidate)
	}
	return ordered, nil
}

type HarnessRunBinding struct {
	VariantID        string               `json:"variant_id,omitempty"`
	WorkflowID       string               `json:"workflow_id,omitempty"`
	WorkflowRevision int                  `json:"workflow_revision,omitempty"`
	ProfileID        string               `json:"profile_id"`
	ProfileVersion   int64                `json:"profile_version"`
	Provider         string               `json:"provider"`
	Model            string               `json:"model"`
	PromptVersion    string               `json:"prompt_version"`
	PromptSHA256     string               `json:"prompt_sha256"`
	SchemaVersion    string               `json:"schema_version"`
	SchemaSHA256     string               `json:"schema_sha256"`
	WorkflowVersion  string               `json:"workflow_version"`
	WorkflowSHA256   string               `json:"workflow_sha256"`
	AssemblerVersion string               `json:"assembler_version"`
	AssemblerSHA256  string               `json:"assembler_sha256"`
	Roles            []HarnessRoleBinding `json:"roles,omitempty"`
}

type HarnessRoleBinding struct {
	Role            string `json:"role"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// Validate 校验运行冻结绑定；公开方法供应用层在费用确认前复用相同规则。
func (binding HarnessRunBinding) Validate() error { return binding.validate() }

func (binding HarnessRunBinding) validate() error {
	if (blankHarness(binding.WorkflowID) && binding.WorkflowRevision != 0) || (!blankHarness(binding.WorkflowID) && binding.WorkflowRevision < 1) {
		return ErrInvalidHarnessRun
	}
	if blankHarness(binding.ProfileID) || binding.ProfileVersion < 1 || blankHarness(binding.Provider) || blankHarness(binding.Model) ||
		blankHarness(binding.PromptVersion) || blankHarness(binding.SchemaVersion) || blankHarness(binding.WorkflowVersion) || blankHarness(binding.AssemblerVersion) {
		return ErrInvalidHarnessRun
	}
	for _, hash := range []string{binding.PromptSHA256, binding.SchemaSHA256, binding.WorkflowSHA256, binding.AssemblerSHA256} {
		if len(hash) != 64 {
			return ErrInvalidHarnessRun
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return ErrInvalidHarnessRun
		}
	}
	seenRoles := make(map[string]struct{}, len(binding.Roles))
	for _, role := range binding.Roles {
		if blankHarness(role.Role) || blankHarness(role.Provider) || blankHarness(role.Model) {
			return ErrInvalidHarnessRun
		}
		if _, exists := seenRoles[role.Role]; exists {
			return ErrInvalidHarnessRun
		}
		seenRoles[role.Role] = struct{}{}
	}
	return nil
}

type HarnessRunStatus string

const (
	HarnessRunQueued      HarnessRunStatus = "queued"
	HarnessRunRunning     HarnessRunStatus = "running"
	HarnessRunRecoverable HarnessRunStatus = "recoverable"
	HarnessRunCompleted   HarnessRunStatus = "completed"
	HarnessRunFailed      HarnessRunStatus = "failed"
	HarnessRunCancelled   HarnessRunStatus = "cancelled"
)

type HarnessRun struct {
	ID             string            `json:"id"`
	DatasetID      string            `json:"dataset_id"`
	DatasetVersion string            `json:"dataset_version"`
	DatasetSHA256  string            `json:"dataset_sha256"`
	Mode           HarnessMode       `json:"mode"`
	Seed           int64             `json:"seed"`
	Binding        HarnessRunBinding `json:"binding"`
	CaseIDs        []string          `json:"case_ids"`
	Status         HarnessRunStatus  `json:"status"`
	RetryOf        string            `json:"retry_of,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	Failure        string            `json:"failure,omitempty"`
	CurrentCaseID  string            `json:"current_case_id,omitempty"`
	PublicStageID  string            `json:"public_stage_id,omitempty"`
	InputTokens    int               `json:"input_tokens"`
	OutputTokens   int               `json:"output_tokens"`
	Calls          int               `json:"calls"`
}

type HarnessAcceptance string

const (
	HarnessAcceptDirect  HarnessAcceptance = "direct"
	HarnessAcceptMinor   HarnessAcceptance = "minor_changes"
	HarnessAcceptMajor   HarnessAcceptance = "major_changes"
	HarnessAcceptRewrite HarnessAcceptance = "rewrite"
)

type HarnessCandidateRating struct {
	Slot       string            `json:"slot"`
	Scores     map[string]int    `json:"scores"`
	Acceptance HarnessAcceptance `json:"acceptance"`
	Note       string            `json:"note,omitempty"`
}

type HarnessRatingSubmission struct {
	ID           string                   `json:"id"`
	RunID        string                   `json:"run_id"`
	CaseID       string                   `json:"case_id"`
	LocalRaterID string                   `json:"local_rater_id"`
	Ratings      []HarnessCandidateRating `json:"ratings"`
	CreatedAt    time.Time                `json:"created_at"`
}

// Validate 校验一次盲评提交覆盖全部匿名候选，并且每个评分维度都完整有效。
func (submission HarnessRatingSubmission) Validate(expectedSlots, dimensions []string) error {
	if blankHarness(submission.ID) || blankHarness(submission.RunID) || blankHarness(submission.CaseID) ||
		blankHarness(submission.LocalRaterID) || submission.CreatedAt.IsZero() || len(submission.Ratings) != len(expectedSlots) || len(dimensions) == 0 {
		return ErrInvalidHarnessRun
	}
	expected := make(map[string]struct{}, len(expectedSlots))
	for _, slot := range expectedSlots {
		expected[slot] = struct{}{}
	}
	seen := make(map[string]struct{}, len(submission.Ratings))
	for _, rating := range submission.Ratings {
		if _, ok := expected[rating.Slot]; !ok {
			return ErrInvalidHarnessRun
		}
		if _, duplicate := seen[rating.Slot]; duplicate || !validHarnessAcceptance(rating.Acceptance) || len(rating.Scores) != len(dimensions) {
			return ErrInvalidHarnessRun
		}
		seen[rating.Slot] = struct{}{}
		for _, dimension := range dimensions {
			score, ok := rating.Scores[dimension]
			if !ok || score < 1 || score > 5 {
				return ErrInvalidHarnessRun
			}
		}
	}
	return nil
}

func validHarnessAcceptance(value HarnessAcceptance) bool {
	switch value {
	case HarnessAcceptDirect, HarnessAcceptMinor, HarnessAcceptMajor, HarnessAcceptRewrite:
		return true
	default:
		return false
	}
}

type HarnessCaseResult struct {
	RunID          string             `json:"run_id"`
	CaseID         string             `json:"case_id"`
	Genre          string             `json:"genre"`
	TaskKind       HarnessTaskKind    `json:"task_kind"`
	Candidates     []HarnessCandidate `json:"candidates"`
	AnonymousOrder []string           `json:"anonymous_order"`
}

type HarnessBreakdown struct {
	Name       string  `json:"name"`
	Candidates int     `json:"candidates"`
	Passed     int     `json:"passed"`
	PassRate   float64 `json:"pass_rate"`
	Blockers   int     `json:"blockers"`
	Majors     int     `json:"majors"`
	Minors     int     `json:"minors"`
}

type HarnessReport struct {
	RunID            string              `json:"run_id"`
	DatasetID        string              `json:"dataset_id"`
	DatasetVersion   string              `json:"dataset_version"`
	DatasetSHA256    string              `json:"dataset_sha256"`
	Provider         string              `json:"provider"`
	Model            string              `json:"model"`
	Binding          HarnessRunBinding   `json:"binding"`
	CaseCount        int                 `json:"case_count"`
	CandidateCount   int                 `json:"candidate_count"`
	PassedCandidates int                 `json:"passed_candidates"`
	PassRate         float64             `json:"pass_rate"`
	InputTokens      int                 `json:"input_tokens"`
	OutputTokens     int                 `json:"output_tokens"`
	Calls            int                 `json:"calls"`
	DurationMillis   int64               `json:"duration_millis"`
	Retries          int                 `json:"retries"`
	ByGenre          []HarnessBreakdown  `json:"by_genre"`
	ByTask           []HarnessBreakdown  `json:"by_task"`
	ByCandidate      []HarnessBreakdown  `json:"by_candidate"`
	Cases            []HarnessCaseReport `json:"cases"`
	RegressedCaseIDs []string            `json:"regressed_case_ids"`
	GeneratedAt      time.Time           `json:"generated_at"`
}

type HarnessCandidateReport struct {
	Kind                        HarnessCandidateKind `json:"kind"`
	ContentSHA256               string               `json:"content_sha256"`
	RuneCount                   int                  `json:"rune_count"`
	GatePassed                  bool                 `json:"gate_passed"`
	Blockers                    int                  `json:"blockers"`
	Majors                      int                  `json:"majors"`
	Minors                      int                  `json:"minors"`
	Resolved                    int                  `json:"resolved"`
	InputTokens                 int                  `json:"input_tokens"`
	OutputTokens                int                  `json:"output_tokens"`
	Calls                       int                  `json:"calls"`
	DurationMillis              int64                `json:"duration_millis"`
	Retries                     int                  `json:"retries"`
	DirectorProtocolPassed      bool                 `json:"director_protocol_passed"`
	ProfessionalConstraintsUsed int                  `json:"professional_constraints_used"`
	ReviewIndependencePassed    bool                 `json:"review_independence_passed"`
	MergeConflicts              int                  `json:"merge_conflicts"`
	ClaimedResolved             int                  `json:"claimed_resolved"`
	ConfirmedResolved           int                  `json:"confirmed_resolved"`
	OverturnedResolutions       int                  `json:"overturned_resolutions"`
	RevisionRegressions         int                  `json:"revision_regressions"`
}

type HarnessCaseReport struct {
	CaseID                     string                   `json:"case_id"`
	Genre                      string                   `json:"genre"`
	TaskKind                   HarnessTaskKind          `json:"task_kind"`
	Candidates                 []HarnessCandidateReport `json:"candidates"`
	InitialVsBaselineRuneDelta int                      `json:"initial_vs_baseline_rune_delta,omitempty"`
	RevisedVsInitialRuneDelta  int                      `json:"revised_vs_initial_rune_delta"`
	Regressed                  bool                     `json:"regressed"`
}

type HarnessReportRegression struct {
	Dimension string  `json:"dimension"`
	Name      string  `json:"name"`
	Base      float64 `json:"base"`
	Candidate float64 `json:"candidate"`
	Delta     float64 `json:"delta"`
}

type HarnessReportComparison struct {
	BaseRunID      string                    `json:"base_run_id"`
	CandidateRunID string                    `json:"candidate_run_id"`
	Regressions    []HarnessReportRegression `json:"regressions"`
}

type HarnessReportReader interface {
	GetReport(context.Context, string) (HarnessReport, error)
}

type HarnessReportService struct {
	reader HarnessReportReader
}

func NewHarnessReportService(reader HarnessReportReader) (*HarnessReportService, error) {
	if reader == nil {
		return nil, ErrInvalidHarnessRun
	}
	return &HarnessReportService{reader: reader}, nil
}

// Compare 从隔离存储读取两次运行报告，并保留每个题材和任务的退化证据。
func (service *HarnessReportService) Compare(ctx context.Context, baseRunID, candidateRunID string) (HarnessReportComparison, error) {
	base, err := service.reader.GetReport(ctx, baseRunID)
	if err != nil {
		return HarnessReportComparison{}, fmt.Errorf("load base harness report: %w", err)
	}
	candidate, err := service.reader.GetReport(ctx, candidateRunID)
	if err != nil {
		return HarnessReportComparison{}, fmt.Errorf("load candidate harness report: %w", err)
	}
	return CompareHarnessReports(base, candidate)
}

// Export 从隔离存储读取单次运行报告并执行隐私检查后导出 JSON。
func (service *HarnessReportService) Export(ctx context.Context, runID string) ([]byte, error) {
	report, err := service.reader.GetReport(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load harness report for export: %w", err)
	}
	return ExportHarnessReport(report)
}

type HarnessRunRequest struct {
	Dataset HarnessDataset
	Mode    HarnessMode
	Seed    int64
	Binding HarnessRunBinding
	CaseIDs []string
	RetryOf string
}

type HarnessExecutor interface {
	Execute(context.Context, HarnessCase, HarnessRunBinding) ([]HarnessCandidate, error)
}

type HarnessExecutionProgress struct {
	CaseID        string
	PublicStageID string
	InputTokens   int
	OutputTokens  int
	Calls         int
}

type HarnessProgressExecutor interface {
	ExecuteWithProgress(context.Context, HarnessCase, HarnessRunBinding, func(HarnessExecutionProgress)) ([]HarnessCandidate, error)
}

type HarnessStore interface {
	CreateRun(context.Context, HarnessRun) error
	UpdateRun(context.Context, HarnessRun) error
	SaveCaseResult(context.Context, HarnessCaseResult) error
	CompleteRun(context.Context, HarnessRun, HarnessReport) error
	GetRun(context.Context, string) (HarnessRun, error)
	ListCaseResults(context.Context, string) ([]HarnessCaseResult, error)
}

type HarnessRunnerConfig struct {
	Store     HarnessStore
	Executor  HarnessExecutor
	Now       func() time.Time
	NewID     func() string
	OnUpdated func(context.Context, HarnessRun)
}

type HarnessRunner struct {
	store     HarnessStore
	executor  HarnessExecutor
	now       func() time.Time
	newID     func() string
	onUpdated func(context.Context, HarnessRun)
}

func NewHarnessRunner(config HarnessRunnerConfig) (*HarnessRunner, error) {
	if config.Store == nil || config.Executor == nil || config.Now == nil || config.NewID == nil {
		return nil, ErrInvalidHarnessRun
	}
	onUpdated := config.OnUpdated
	if onUpdated == nil {
		onUpdated = func(context.Context, HarnessRun) {}
	}
	return &HarnessRunner{store: config.Store, executor: config.Executor, now: config.Now, newID: config.NewID, onUpdated: onUpdated}, nil
}

// Run 冻结数据集与模型编排版本，逐 case 执行对照组并生成保留分组证据的报告。
func (runner *HarnessRunner) Run(ctx context.Context, request HarnessRunRequest) (HarnessReport, error) {
	run, err := runner.Create(ctx, request)
	if err != nil {
		return HarnessReport{}, err
	}
	return runner.Execute(ctx, run.ID, request.Dataset)
}

// Create 冻结数据集、case、模型与工作流绑定，但不触发任何付费调用。
func (runner *HarnessRunner) Create(ctx context.Context, request HarnessRunRequest) (HarnessRun, error) {
	if err := request.Dataset.Validate(); err != nil || request.Binding.validate() != nil || request.Mode != HarnessModeReduced && request.Mode != HarnessModeFull {
		return HarnessRun{}, ErrInvalidHarnessRun
	}
	cases := selectHarnessCases(request.Dataset.Cases, request.Mode, request.CaseIDs)
	if len(cases) == 0 {
		return HarnessRun{}, ErrInvalidHarnessRun
	}
	now := runner.now().UTC()
	run := HarnessRun{ID: runner.newID(), DatasetID: request.Dataset.ID, DatasetVersion: request.Dataset.Version, DatasetSHA256: request.Dataset.SHA256, Mode: request.Mode, Seed: request.Seed, Binding: request.Binding, Status: HarnessRunQueued, RetryOf: strings.TrimSpace(request.RetryOf), StartedAt: now}
	for _, item := range cases {
		run.CaseIDs = append(run.CaseIDs, item.ID)
	}
	if blankHarness(run.ID) {
		return HarnessRun{}, ErrInvalidHarnessRun
	}
	if err := runner.store.CreateRun(ctx, run); err != nil {
		return HarnessRun{}, fmt.Errorf("create harness run: %w", err)
	}
	runner.onUpdated(ctx, run)
	return run, nil
}

// Execute 从持久化进度继续运行；已完成 case 不会再次调用 Provider。
func (runner *HarnessRunner) Execute(ctx context.Context, runID string, dataset HarnessDataset) (HarnessReport, error) {
	run, err := runner.store.GetRun(ctx, strings.TrimSpace(runID))
	if err != nil || dataset.Validate() != nil || run.DatasetID != dataset.ID || run.DatasetVersion != dataset.Version || run.DatasetSHA256 != dataset.SHA256 || run.Status == HarnessRunCompleted {
		return HarnessReport{}, ErrInvalidHarnessRun
	}
	caseByID := make(map[string]HarnessCase, len(dataset.Cases))
	for _, item := range dataset.Cases {
		caseByID[item.ID] = item
	}
	results, err := runner.store.ListCaseResults(ctx, run.ID)
	if err != nil {
		return HarnessReport{}, fmt.Errorf("load harness progress: %w", err)
	}
	completedCases := make(map[string]struct{}, len(results))
	for _, result := range results {
		completedCases[result.CaseID] = struct{}{}
	}
	run.Status = HarnessRunRunning
	run.CompletedAt = nil
	run.Failure = ""
	if err := runner.store.UpdateRun(ctx, run); err != nil {
		return HarnessReport{}, fmt.Errorf("start harness run: %w", err)
	}
	runner.onUpdated(ctx, run)
	for _, caseID := range run.CaseIDs {
		if _, exists := completedCases[caseID]; exists {
			continue
		}
		item, exists := caseByID[caseID]
		if !exists {
			return HarnessReport{}, ErrInvalidHarnessRun
		}
		if err := ctx.Err(); err != nil {
			runner.stopRun(context.WithoutCancel(ctx), &run, HarnessRunCancelled, "运行已取消")
			return HarnessReport{}, err
		}
		run.CurrentCaseID, run.PublicStageID = item.ID, "preparing"
		_ = runner.store.UpdateRun(ctx, run)
		runner.onUpdated(ctx, run)
		var candidates []HarnessCandidate
		completedInput, completedOutput, completedCalls := harnessUsage(results)
		if executor, ok := runner.executor.(HarnessProgressExecutor); ok {
			candidates, err = executor.ExecuteWithProgress(ctx, item, run.Binding, func(progress HarnessExecutionProgress) {
				run.CurrentCaseID, run.PublicStageID = progress.CaseID, progress.PublicStageID
				run.InputTokens, run.OutputTokens, run.Calls = completedInput+progress.InputTokens, completedOutput+progress.OutputTokens, completedCalls+progress.Calls
				_ = runner.store.UpdateRun(context.WithoutCancel(ctx), run)
				runner.onUpdated(ctx, run)
			})
		} else {
			candidates, err = runner.executor.Execute(ctx, item, run.Binding)
		}
		if err != nil {
			status := HarnessRunFailed
			if errors.Is(err, context.Canceled) {
				status = HarnessRunCancelled
			}
			failureMessage := "case 执行失败"
			var executionFailure *HarnessExecutionError
			if errors.As(err, &executionFailure) && strings.TrimSpace(executionFailure.PublicMessage) != "" {
				failureMessage = executionFailure.PublicMessage
			}
			runner.stopRun(context.WithoutCancel(ctx), &run, status, failureMessage)
			return HarnessReport{}, fmt.Errorf("execute harness case %s: %w", item.ID, err)
		}
		if err := validateCandidates(item.TaskKind, candidates); err != nil {
			runner.stopRun(context.WithoutCancel(ctx), &run, HarnessRunFailed, "候选验证失败")
			return HarnessReport{}, fmt.Errorf("validate harness case %s: %w", item.ID, err)
		}
		ordered, err := anonymousCandidateOrder(item.ID, run.Seed, candidates)
		if err != nil {
			return HarnessReport{}, err
		}
		result := HarnessCaseResult{RunID: run.ID, CaseID: item.ID, Genre: item.Genre, TaskKind: item.TaskKind, Candidates: candidates}
		for _, candidate := range ordered {
			result.AnonymousOrder = append(result.AnonymousOrder, string(candidate.Kind))
		}
		if err := runner.store.SaveCaseResult(ctx, result); err != nil {
			runner.stopRun(context.WithoutCancel(ctx), &run, HarnessRunFailed, "case 保存失败")
			return HarnessReport{}, fmt.Errorf("save harness case %s: %w", item.ID, err)
		}
		results = append(results, result)
		run.InputTokens, run.OutputTokens, run.Calls = harnessUsage(results)
		run.PublicStageID = "case_completed"
		_ = runner.store.UpdateRun(context.WithoutCancel(ctx), run)
		runner.onUpdated(ctx, run)
	}

	completed := runner.now().UTC()
	run.Status = HarnessRunCompleted
	run.CurrentCaseID = ""
	run.PublicStageID = "candidate_ready"
	run.CompletedAt = &completed
	report := buildHarnessReport(run, results, completed)
	if err := runner.store.CompleteRun(ctx, run, report); err != nil {
		return HarnessReport{}, fmt.Errorf("complete harness run: %w", err)
	}
	runner.onUpdated(ctx, run)
	return report, nil
}

func harnessUsage(results []HarnessCaseResult) (int, int, int) {
	var inputTokens, outputTokens, calls int
	for _, result := range results {
		for _, candidate := range result.Candidates {
			inputTokens += candidate.Metrics.InputTokens
			outputTokens += candidate.Metrics.OutputTokens
			calls += candidate.Metrics.Calls
		}
	}
	return inputTokens, outputTokens, calls
}

func (runner *HarnessRunner) stopRun(ctx context.Context, run *HarnessRun, status HarnessRunStatus, failure string) {
	completed := runner.now().UTC()
	run.Status = status
	run.CompletedAt = &completed
	run.Failure = failure
	_ = runner.store.UpdateRun(ctx, *run)
	runner.onUpdated(ctx, *run)
}

func buildHarnessReport(run HarnessRun, results []HarnessCaseResult, generatedAt time.Time) HarnessReport {
	report := HarnessReport{RunID: run.ID, DatasetID: run.DatasetID, DatasetVersion: run.DatasetVersion, DatasetSHA256: run.DatasetSHA256, Provider: run.Binding.Provider, Model: run.Binding.Model, Binding: run.Binding, CaseCount: len(results), GeneratedAt: generatedAt}
	genreGroups := map[string][]HarnessCandidate{}
	taskGroups := map[string][]HarnessCandidate{}
	candidateGroups := map[string][]HarnessCandidate{}
	for _, result := range results {
		genreGroups[result.Genre] = append(genreGroups[result.Genre], result.Candidates...)
		taskGroups[string(result.TaskKind)] = append(taskGroups[string(result.TaskKind)], result.Candidates...)
		caseReport := HarnessCaseReport{CaseID: result.CaseID, Genre: result.Genre, TaskKind: result.TaskKind}
		byKind := make(map[HarnessCandidateKind]HarnessCandidate, len(result.Candidates))
		for _, candidate := range result.Candidates {
			candidateGroups[string(candidate.Kind)] = append(candidateGroups[string(candidate.Kind)], candidate)
			byKind[candidate.Kind] = candidate
			report.CandidateCount++
			if candidate.Metrics.GatePassed {
				report.PassedCandidates++
			}
			report.InputTokens += candidate.Metrics.InputTokens
			report.OutputTokens += candidate.Metrics.OutputTokens
			report.Calls += candidate.Metrics.Calls
			report.DurationMillis += candidate.Metrics.DurationMillis
			report.Retries += candidate.Metrics.Retries
			sum := sha256.Sum256([]byte(candidate.Content))
			caseReport.Candidates = append(caseReport.Candidates, HarnessCandidateReport{
				Kind: candidate.Kind, ContentSHA256: hex.EncodeToString(sum[:]), RuneCount: utf8.RuneCountInString(candidate.Content),
				GatePassed: candidate.Metrics.GatePassed, Blockers: candidate.Metrics.Blockers, Majors: candidate.Metrics.Majors,
				Minors: candidate.Metrics.Minors, Resolved: candidate.Metrics.Resolved, InputTokens: candidate.Metrics.InputTokens,
				OutputTokens: candidate.Metrics.OutputTokens, Calls: candidate.Metrics.Calls, DurationMillis: candidate.Metrics.DurationMillis,
				Retries:                     candidate.Metrics.Retries,
				DirectorProtocolPassed:      candidate.Metrics.DirectorProtocolPassed,
				ProfessionalConstraintsUsed: candidate.Metrics.ProfessionalConstraintsUsed,
				ReviewIndependencePassed:    candidate.Metrics.ReviewIndependencePassed,
				MergeConflicts:              candidate.Metrics.MergeConflicts,
				ClaimedResolved:             candidate.Metrics.ClaimedResolved,
				ConfirmedResolved:           candidate.Metrics.ConfirmedResolved,
				OverturnedResolutions:       candidate.Metrics.OverturnedResolutions,
				RevisionRegressions:         candidate.Metrics.RevisionRegressions,
			})
		}
		if baseline, exists := byKind[CandidateBaseline]; exists {
			caseReport.InitialVsBaselineRuneDelta = utf8.RuneCountInString(byKind[CandidateInitial].Content) - utf8.RuneCountInString(baseline.Content)
		}
		initial := byKind[CandidateInitial]
		revised := byKind[CandidateRevised]
		caseReport.RevisedVsInitialRuneDelta = utf8.RuneCountInString(revised.Content) - utf8.RuneCountInString(initial.Content)
		caseReport.Regressed = initial.Metrics.GatePassed && !revised.Metrics.GatePassed || revised.Metrics.Blockers > initial.Metrics.Blockers || revised.Metrics.Majors > initial.Metrics.Majors
		if caseReport.Regressed {
			report.RegressedCaseIDs = append(report.RegressedCaseIDs, result.CaseID)
		}
		report.Cases = append(report.Cases, caseReport)
	}
	if report.CandidateCount > 0 {
		report.PassRate = float64(report.PassedCandidates) / float64(report.CandidateCount)
	}
	report.ByGenre = harnessBreakdowns(genreGroups)
	report.ByTask = harnessBreakdowns(taskGroups)
	report.ByCandidate = harnessBreakdowns(candidateGroups)
	sort.Strings(report.RegressedCaseIDs)
	return report
}

func harnessBreakdowns(groups map[string][]HarnessCandidate) []HarnessBreakdown {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	breakdowns := make([]HarnessBreakdown, 0, len(names))
	for _, name := range names {
		item := HarnessBreakdown{Name: name, Candidates: len(groups[name])}
		for _, candidate := range groups[name] {
			if candidate.Metrics.GatePassed {
				item.Passed++
			}
			item.Blockers += candidate.Metrics.Blockers
			item.Majors += candidate.Metrics.Majors
			item.Minors += candidate.Metrics.Minors
		}
		if item.Candidates > 0 {
			item.PassRate = float64(item.Passed) / float64(item.Candidates)
		}
		breakdowns = append(breakdowns, item)
	}
	return breakdowns
}

// ExportHarnessReport 仅序列化白名单报告字段，并拒绝疑似密钥或凭据引用。
func ExportHarnessReport(report HarnessReport) ([]byte, error) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode harness report: %w", err)
	}
	if containsSensitiveReportData(string(payload)) {
		return nil, ErrSensitiveReportData
	}
	return append(payload, '\n'), nil
}

func CompareHarnessReports(base, candidate HarnessReport) (HarnessReportComparison, error) {
	if blankHarness(base.RunID) || blankHarness(candidate.RunID) || base.DatasetID != candidate.DatasetID {
		return HarnessReportComparison{}, ErrInvalidHarnessRun
	}
	comparison := HarnessReportComparison{BaseRunID: base.RunID, CandidateRunID: candidate.RunID}
	compareBreakdown := func(dimension string, baseItems, candidateItems []HarnessBreakdown) {
		baseByName := make(map[string]HarnessBreakdown, len(baseItems))
		for _, item := range baseItems {
			baseByName[item.Name] = item
		}
		for _, item := range candidateItems {
			previous, exists := baseByName[item.Name]
			if exists && item.PassRate < previous.PassRate {
				comparison.Regressions = append(comparison.Regressions, HarnessReportRegression{Dimension: dimension, Name: item.Name, Base: previous.PassRate, Candidate: item.PassRate, Delta: item.PassRate - previous.PassRate})
			}
		}
	}
	compareBreakdown("genre", base.ByGenre, candidate.ByGenre)
	compareBreakdown("task", base.ByTask, candidate.ByTask)
	sort.Slice(comparison.Regressions, func(left, right int) bool {
		if comparison.Regressions[left].Dimension == comparison.Regressions[right].Dimension {
			return comparison.Regressions[left].Name < comparison.Regressions[right].Name
		}
		return comparison.Regressions[left].Dimension < comparison.Regressions[right].Dimension
	})
	return comparison, nil
}

func selectHarnessCases(cases []HarnessCase, mode HarnessMode, requested []string) []HarnessCase {
	wanted := make(map[string]struct{}, len(requested))
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		wanted[id] = struct{}{}
	}
	selected := make([]HarnessCase, 0, len(cases))
	for _, item := range cases {
		_, explicitlyRequested := wanted[item.ID]
		if len(wanted) > 0 && !explicitlyRequested {
			continue
		}
		if mode == HarnessModeFull || item.CIReduced || explicitlyRequested {
			selected = append(selected, item)
		}
	}
	if len(wanted) > 0 && len(selected) != len(wanted) {
		return nil
	}
	return selected
}

func validateCandidates(kind HarnessTaskKind, candidates []HarnessCandidate) error {
	required := map[HarnessCandidateKind]bool{CandidateInitial: false, CandidateRevised: false}
	if kind == HarnessTaskChapter {
		required[CandidateBaseline] = false
	}
	for _, candidate := range candidates {
		if !validCandidateKind(candidate.Kind) || blankHarness(candidate.Content) || required[candidate.Kind] {
			return ErrInvalidHarnessRun
		}
		if _, expected := required[candidate.Kind]; !expected {
			return ErrInvalidHarnessRun
		}
		required[candidate.Kind] = true
	}
	for _, exists := range required {
		if !exists {
			return ErrInvalidHarnessRun
		}
	}
	return nil
}

func validCandidateKind(kind HarnessCandidateKind) bool {
	return kind == CandidateBaseline || kind == CandidateInitial || kind == CandidateRevised
}

func blankHarness(value string) bool { return strings.TrimSpace(value) == "" }

func containsFold(value, substring string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(substring))
}

func containsSensitiveReportData(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"sk-", "api_key", "apikey", "credential_ref", "credential-ref", "secret://", "bearer "} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return !utf8.ValidString(value)
}
