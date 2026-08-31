package creativeharness

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

var (
	ErrUnavailable          = errors.New("creative harness unavailable")
	ErrConfirmationRequired = errors.New("creative harness cost confirmation required")
	ErrRatingRequired       = errors.New("creative harness rating required before reveal")
	ErrRunActive            = errors.New("creative harness run is active")
)

type Store interface {
	quality.HarnessStore
	quality.HarnessReportReader
	ListRuns(context.Context) ([]quality.HarnessRun, error)
	DeleteRun(context.Context, string) error
	GetBlindCaseResult(context.Context, string, string) ([]quality.AnonymousHarnessCandidate, error)
	GetCaseResult(context.Context, string, string) (quality.HarnessCaseResult, error)
	SaveRatingSubmission(context.Context, quality.HarnessRatingSubmission) error
	GetRatingSubmission(context.Context, string, string, string) (quality.HarnessRatingSubmission, error)
	ListRatingSubmissions(context.Context, string) ([]quality.HarnessRatingSubmission, error)
}

type PreviewInput struct {
	ProfileID      string              `json:"profileId"`
	ProfileVersion int64               `json:"profileVersion"`
	VariantID      string              `json:"variantId"`
	Mode           quality.HarnessMode `json:"mode"`
	CaseIDs        []string            `json:"caseIds"`
	Seed           int64               `json:"seed"`
}

type RunInput struct {
	PreviewInput
	ConfirmationDigest string `json:"confirmationDigest"`
	CostConfirmed      bool   `json:"costConfirmed"`
}

type RunPreview struct {
	DatasetID              string                       `json:"datasetId"`
	DatasetVersion         string                       `json:"datasetVersion"`
	DatasetSHA256          string                       `json:"datasetSha256"`
	CaseCount              int                          `json:"caseCount"`
	EstimatedCalls         int                          `json:"estimatedCalls"`
	MaximumOutputTokens    int                          `json:"maximumOutputTokens"`
	RunBudgetUnlimited     bool                         `json:"runBudgetUnlimited"`
	ContextBudgetUnlimited bool                         `json:"contextBudgetUnlimited"`
	MaxRunTokens           int                          `json:"maxRunTokens"`
	MaxContextChars        int                          `json:"maxContextChars"`
	CurrencyEstimate       string                       `json:"currencyEstimate"`
	Provider               string                       `json:"provider"`
	Model                  string                       `json:"model"`
	Roles                  []quality.HarnessRoleBinding `json:"roles"`
	WorkflowID             string                       `json:"workflowId"`
	WorkflowVersion        int                          `json:"workflowVersion"`
	ConfirmationDigest     string                       `json:"confirmationDigest"`
	Binding                quality.HarnessRunBinding    `json:"binding"`
}

type DatasetView struct {
	Dataset  quality.HarnessDataset           `json:"dataset"`
	Variants []quality.HarnessWorkflowVariant `json:"variants"`
}

type RunView struct {
	Run              quality.HarnessRun     `json:"run"`
	CompletedCases   int                    `json:"completedCases"`
	CompletedCaseIDs []string               `json:"completedCaseIds"`
	TotalCases       int                    `json:"totalCases"`
	Report           *quality.HarnessReport `json:"report,omitempty"`
	Ratings          RatingSummary          `json:"ratings"`
}

type RatingSummary struct {
	Submissions      int                `json:"submissions"`
	CandidateRatings int                `json:"candidateRatings"`
	Acceptable       int                `json:"acceptable"`
	Rewrite          int                `json:"rewrite"`
	DimensionAverage map[string]float64 `json:"dimensionAverage"`
}

type ComparisonView struct {
	Automatic         quality.HarnessReportComparison   `json:"automatic"`
	BaseRatings       RatingSummary                     `json:"baseRatings"`
	CandidateRatings  RatingSummary                     `json:"candidateRatings"`
	RatingRegressions []quality.HarnessReportRegression `json:"ratingRegressions"`
}

type CaseView struct {
	Case       quality.HarnessCase                     `json:"case"`
	Candidates []quality.AnonymousHarnessCandidate     `json:"candidates"`
	Rating     *quality.HarnessRatingSubmission        `json:"rating,omitempty"`
	Revealed   map[string]quality.HarnessCandidateKind `json:"revealed,omitempty"`
	Binding    *quality.HarnessRunBinding              `json:"binding,omitempty"`
}

type RatingInput struct {
	RunID        string                           `json:"runId"`
	CaseID       string                           `json:"caseId"`
	LocalRaterID string                           `json:"localRaterId"`
	Ratings      []quality.HarnessCandidateRating `json:"ratings"`
}

type ServiceConfig struct {
	Store       Store
	Profiles    ports.CreativeProfileResolver
	Credentials ports.CredentialStore
	Providers   ports.ModelProviderFactory
	Clock       ports.Clock
	IDs         ports.IDGenerator
	OnUpdated   func(context.Context, string)
}

type Service struct {
	store     Store
	profiles  ports.CreativeProfileResolver
	registry  *agentassets.Registry
	dataset   quality.HarnessDataset
	runner    *quality.HarnessRunner
	clock     ports.Clock
	ids       ports.IDGenerator
	onUpdated func(context.Context, string)
	mu        sync.Mutex
	cancels   map[string]context.CancelFunc
	wg        sync.WaitGroup
}

// Close 取消仍在运行的实验；Runner 会把取消终态写入独立库后再由应用关闭数据库。
func (service *Service) Close() {
	service.mu.Lock()
	for _, cancel := range service.cancels {
		cancel()
	}
	service.mu.Unlock()
	service.wg.Wait()
}

// New 创建独立实验室服务；真实模型调用复用已发布创作方案与 Agent Kernel。
func New(config ServiceConfig) (*Service, error) {
	if config.Store == nil || config.Profiles == nil || config.Credentials == nil || config.Providers == nil || config.Clock == nil || config.IDs == nil {
		return nil, ErrUnavailable
	}
	dataset, err := quality.BuiltinHarnessDatasetV1()
	if err != nil {
		return nil, err
	}
	registry, err := agentassets.Builtin()
	if err != nil {
		return nil, err
	}
	onUpdated := config.OnUpdated
	if onUpdated == nil {
		onUpdated = func(context.Context, string) {}
	}
	executor := &kernelExecutor{profiles: config.Profiles, credentials: config.Credentials, providers: config.Providers, registry: registry, clock: config.Clock, ids: config.IDs}
	service := &Service{store: config.Store, profiles: config.Profiles, registry: registry, dataset: dataset, clock: config.Clock, ids: config.IDs, onUpdated: onUpdated, cancels: make(map[string]context.CancelFunc)}
	runner, err := quality.NewHarnessRunner(quality.HarnessRunnerConfig{Store: config.Store, Executor: executor, Now: config.Clock.Now, NewID: config.IDs.NewID, OnUpdated: func(ctx context.Context, run quality.HarnessRun) { onUpdated(ctx, run.ID) }})
	if err != nil {
		return nil, err
	}
	service.runner = runner
	return service, nil
}

func (service *Service) Datasets() DatasetView {
	return DatasetView{Dataset: service.dataset, Variants: quality.BuiltinHarnessWorkflowVariants()}
}

// RecoverInterrupted 把进程退出时仍为运行中的实验标记为可恢复。
func (service *Service) RecoverInterrupted(ctx context.Context) error {
	runs, err := service.store.ListRuns(ctx)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Status != quality.HarnessRunRunning {
			continue
		}
		run.Status, run.Failure = quality.HarnessRunRecoverable, "应用退出，可从已保存 case 继续"
		if err := service.store.UpdateRun(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) Preview(ctx context.Context, input PreviewInput) (RunPreview, error) {
	if input.Mode != quality.HarnessModeReduced && input.Mode != quality.HarnessModeFull {
		return RunPreview{}, quality.ErrInvalidHarnessRun
	}
	profile, err := service.profiles.ResolveCreativeProfile(ctx, strings.TrimSpace(input.ProfileID), input.ProfileVersion)
	if err != nil {
		return RunPreview{}, fmt.Errorf("resolve creative profile: %w", err)
	}
	variant, err := findVariant(input.VariantID)
	if err != nil {
		return RunPreview{}, err
	}
	if len(profile.Roles) == 0 {
		return RunPreview{}, quality.ErrInvalidHarnessRun
	}
	cases := selectCasesForPreview(service.dataset.Cases, input.Mode, input.CaseIDs)
	if len(cases) == 0 {
		return RunPreview{}, quality.ErrInvalidHarnessRun
	}
	compiled, err := agentkernel.Compile(ctx, agentkernel.Registries{Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry}, variant.WorkflowID, variant.WorkflowVersion)
	if err != nil {
		return RunPreview{}, err
	}
	role := profile.Roles[0]
	for _, candidate := range profile.Roles {
		if candidate.Role == agentdomain.RoleWriter {
			role = candidate
		}
	}
	first := compiled.Nodes[0]
	binding := quality.HarnessRunBinding{VariantID: variant.ID, WorkflowID: variant.WorkflowID, WorkflowRevision: variant.WorkflowVersion, ProfileID: profile.ProfileID, ProfileVersion: profile.Version, Provider: role.ProviderName, Model: role.Model, PromptVersion: fmt.Sprintf("%s@%d", first.Prompt.ID, first.Prompt.Version), PromptSHA256: first.Prompt.Hash, SchemaVersion: fmt.Sprintf("%s@%d", first.Schema.ID, first.Schema.Version), SchemaSHA256: first.Schema.Hash, WorkflowVersion: fmt.Sprintf("%s@%d", compiled.Workflow.ID, compiled.Workflow.Version), WorkflowSHA256: compiled.Workflow.Hash, AssemblerVersion: fmt.Sprintf("%s@%d", first.Assembler.ID, first.Assembler.Version), AssemblerSHA256: first.Assembler.Hash}
	for _, resolvedRole := range profile.Roles {
		binding.Roles = append(binding.Roles, quality.HarnessRoleBinding{
			Role: string(resolvedRole.Role), Provider: resolvedRole.ProviderName, Model: resolvedRole.Model,
			ReasoningEffort: string(resolvedRole.ReasoningEffort.Normalized()),
		})
	}
	estimate := "Provider 未提供稳定单价；执行前仅确认方案冻结的运行预算与调用次数"
	if profile.RunBudgetUnlimited {
		estimate = "当前方案运行 Token 预算为不限制，应用无法封顶本次真实调用费用。"
	}
	maxRunTokens, maxContextChars := profile.MaxRunTokens, profile.MaxContextChars
	if profile.RunBudgetUnlimited {
		maxRunTokens = 0
	}
	if profile.ContextBudgetUnlimited {
		maxContextChars = 0
	}
	preview := RunPreview{
		DatasetID: service.dataset.ID, DatasetVersion: service.dataset.Version, DatasetSHA256: service.dataset.SHA256,
		CaseCount: len(cases), Provider: role.ProviderName, Model: role.Model, Roles: binding.Roles,
		WorkflowID: variant.WorkflowID, WorkflowVersion: variant.WorkflowVersion, Binding: binding,
		CurrencyEstimate: estimate, RunBudgetUnlimited: profile.RunBudgetUnlimited,
		ContextBudgetUnlimited: profile.ContextBudgetUnlimited,
		MaxRunTokens:           maxRunTokens, MaxContextChars: maxContextChars,
	}
	// 费用预览必须按执行器实际选择的工作流逐 case 计算：蓝图 case 会切换
	// 蓝图工作流，章节 case 还会额外运行一次 baseline。
	runCount := 0
	compiledByWorkflow := map[string]agentkernel.CompiledWorkflow{
		fmt.Sprintf("%s@%d", variant.WorkflowID, variant.WorkflowVersion): compiled,
	}
	loadWorkflow := func(id string, version int) (agentkernel.CompiledWorkflow, error) {
		key := fmt.Sprintf("%s@%d", id, version)
		if cached, ok := compiledByWorkflow[key]; ok {
			return cached, nil
		}
		loaded, loadErr := agentkernel.Compile(ctx, agentkernel.Registries{Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry}, id, version)
		if loadErr == nil {
			compiledByWorkflow[key] = loaded
		}
		return loaded, loadErr
	}
	accountWorkflow := func(id string, version int) error {
		actual, loadErr := loadWorkflow(id, version)
		if loadErr != nil {
			return loadErr
		}
		if !profile.RunBudgetUnlimited && profile.MaxRunTokens < actual.RequiredTokenBudget() {
			return fmt.Errorf("%w: 创作方案运行预算至少需要 %d Token", agentkernel.ErrTokenBudget, actual.RequiredTokenBudget())
		}
		preview.EstimatedCalls += len(actual.Nodes)
		runCount++
		return nil
	}
	for _, item := range cases {
		workflowID, workflowVersion := variant.WorkflowID, variant.WorkflowVersion
		if item.TaskKind == quality.HarnessTaskBlueprint {
			workflowID, workflowVersion = agentassets.QualityBlueprintWorkflowID, agentassets.QualityBlueprintWorkflowVersion
		}
		if item.TaskKind == quality.HarnessTaskChapter {
			if err := accountWorkflow(agentassets.BaselineWorkflowID, agentassets.BaselineWorkflowVersion); err != nil {
				return RunPreview{}, err
			}
		}
		if err := accountWorkflow(workflowID, workflowVersion); err != nil {
			return RunPreview{}, err
		}
	}
	if profile.RunBudgetUnlimited {
		preview.MaximumOutputTokens = 0
	} else {
		preview.MaximumOutputTokens = profile.MaxRunTokens * runCount
	}
	payload, _ := json.Marshal(struct {
		Input   PreviewInput
		Binding quality.HarnessRunBinding
		Cases   []quality.HarnessCase
	}{input, binding, cases})
	digest := sha256.Sum256(payload)
	preview.ConfirmationDigest = hex.EncodeToString(digest[:])
	return preview, nil
}

func (service *Service) Create(ctx context.Context, input RunInput) (RunView, error) {
	preview, err := service.Preview(ctx, input.PreviewInput)
	if err != nil {
		return RunView{}, err
	}
	if !input.CostConfirmed || input.ConfirmationDigest != preview.ConfirmationDigest {
		return RunView{}, ErrConfirmationRequired
	}
	run, err := service.runner.Create(ctx, quality.HarnessRunRequest{Dataset: service.dataset, Mode: input.Mode, Seed: input.Seed, Binding: preview.Binding, CaseIDs: input.CaseIDs})
	if err != nil {
		return RunView{}, err
	}
	return service.runView(ctx, run)
}

// Execute 异步启动或恢复实验；页面通过持久化投影观察，不依赖 Bridge 持续在线。
func (service *Service) Execute(ctx context.Context, runID string) (RunView, error) {
	service.mu.Lock()
	if _, running := service.cancels[runID]; running {
		service.mu.Unlock()
		run, err := service.store.GetRun(ctx, runID)
		if err != nil {
			return RunView{}, err
		}
		return service.runView(ctx, run)
	}
	run, err := service.store.GetRun(ctx, runID)
	if err != nil {
		service.mu.Unlock()
		return RunView{}, err
	}
	if run.Status != quality.HarnessRunQueued && run.Status != quality.HarnessRunRecoverable && run.Status != quality.HarnessRunFailed && run.Status != quality.HarnessRunCancelled {
		service.mu.Unlock()
		return RunView{}, quality.ErrInvalidHarnessRun
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	service.cancels[runID] = cancel
	service.wg.Add(1)
	service.mu.Unlock()
	go func() {
		defer service.wg.Done()
		defer func() {
			service.mu.Lock()
			delete(service.cancels, runID)
			service.mu.Unlock()
		}()
		_, _ = service.runner.Execute(runCtx, runID, service.dataset)
	}()
	return service.runView(ctx, run)
}

// DeleteRun 删除未在执行的实验记录；数据库外键负责级联清理盲评与报告。
func (service *Service) DeleteRun(ctx context.Context, runID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	run, err := service.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status == quality.HarnessRunRunning {
		return ErrRunActive
	}
	if _, running := service.cancels[runID]; running {
		return ErrRunActive
	}
	return service.store.DeleteRun(ctx, runID)
}

func (service *Service) Cancel(ctx context.Context, runID string) error {
	service.mu.Lock()
	cancel := service.cancels[runID]
	service.mu.Unlock()
	if cancel == nil {
		return quality.ErrInvalidHarnessRun
	}
	cancel()
	return nil
}

func (service *Service) Runs(ctx context.Context) ([]RunView, error) {
	runs, err := service.store.ListRuns(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]RunView, 0, len(runs))
	for _, run := range runs {
		view, err := service.runView(ctx, run)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (service *Service) Run(ctx context.Context, runID string) (RunView, error) {
	run, err := service.store.GetRun(ctx, runID)
	if err != nil {
		return RunView{}, err
	}
	return service.runView(ctx, run)
}

// RetryCase 创建只包含一个退化或失败 case 的新运行；创建动作本身不发起模型调用。
func (service *Service) RetryCase(ctx context.Context, runID, caseID string) (RunView, error) {
	previous, err := service.store.GetRun(ctx, runID)
	if err != nil {
		return RunView{}, err
	}
	if _, ok := datasetCase(service.dataset, caseID); !ok || !containsCaseID(previous.CaseIDs, caseID) {
		return RunView{}, sql.ErrNoRows
	}
	run, err := service.runner.Create(ctx, quality.HarnessRunRequest{Dataset: service.dataset, Mode: quality.HarnessModeFull, Seed: previous.Seed, Binding: previous.Binding, CaseIDs: []string{caseID}, RetryOf: previous.ID})
	if err != nil {
		return RunView{}, err
	}
	return service.runView(ctx, run)
}

func (service *Service) runView(ctx context.Context, run quality.HarnessRun) (RunView, error) {
	results, err := service.store.ListCaseResults(ctx, run.ID)
	if err != nil {
		return RunView{}, err
	}
	view := RunView{Run: run, CompletedCases: len(results), TotalCases: len(run.CaseIDs)}
	for _, result := range results {
		view.CompletedCaseIDs = append(view.CompletedCaseIDs, result.CaseID)
	}
	if report, err := service.store.GetReport(ctx, run.ID); err == nil {
		view.Report = &report
	} else if !errors.Is(err, sql.ErrNoRows) {
		return RunView{}, err
	}
	submissions, err := service.store.ListRatingSubmissions(ctx, run.ID)
	if err != nil {
		return RunView{}, err
	}
	view.Ratings = summarizeRatings(submissions)
	return view, nil
}

func (service *Service) Case(ctx context.Context, runID, caseID, raterID string) (CaseView, error) {
	item, ok := datasetCase(service.dataset, caseID)
	if !ok {
		return CaseView{}, sql.ErrNoRows
	}
	candidates, err := service.store.GetBlindCaseResult(ctx, runID, caseID)
	if err != nil {
		return CaseView{}, err
	}
	view := CaseView{Case: item, Candidates: candidates}
	rating, err := service.store.GetRatingSubmission(ctx, runID, caseID, raterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return view, nil
		}
		return CaseView{}, err
	}
	view.Rating = &rating
	run, err := service.store.GetRun(ctx, runID)
	if err != nil {
		return CaseView{}, err
	}
	result, err := service.store.GetCaseResult(ctx, runID, caseID)
	if err != nil {
		return CaseView{}, err
	}
	view.Revealed = make(map[string]quality.HarnessCandidateKind, len(result.AnonymousOrder))
	byKind := make(map[string]quality.HarnessCandidate, len(result.Candidates))
	for _, candidate := range result.Candidates {
		byKind[string(candidate.Kind)] = candidate
	}
	for index, kind := range result.AnonymousOrder {
		slot := fmt.Sprintf("候选-%d", index+1)
		view.Revealed[slot] = quality.HarnessCandidateKind(kind)
		view.Candidates[index].Metrics = byKind[kind].Metrics
	}
	view.Binding = &run.Binding
	return view, nil
}

func (service *Service) SaveRating(ctx context.Context, input RatingInput) (CaseView, error) {
	item, ok := datasetCase(service.dataset, input.CaseID)
	if !ok {
		return CaseView{}, sql.ErrNoRows
	}
	candidates, err := service.store.GetBlindCaseResult(ctx, input.RunID, input.CaseID)
	if err != nil {
		return CaseView{}, err
	}
	slots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		slots = append(slots, candidate.Slot)
	}
	submission := quality.HarnessRatingSubmission{ID: service.ids.NewID(), RunID: input.RunID, CaseID: input.CaseID, LocalRaterID: strings.TrimSpace(input.LocalRaterID), Ratings: input.Ratings, CreatedAt: service.clock.Now().UTC()}
	if err := submission.Validate(slots, item.RatingDimensions); err != nil {
		return CaseView{}, err
	}
	if err := service.store.SaveRatingSubmission(ctx, submission); err != nil {
		return CaseView{}, err
	}
	service.onUpdated(ctx, input.RunID)
	return service.Case(ctx, input.RunID, input.CaseID, input.LocalRaterID)
}

func (service *Service) Compare(ctx context.Context, baseRunID, candidateRunID string) (ComparisonView, error) {
	reports, err := quality.NewHarnessReportService(service.store)
	if err != nil {
		return ComparisonView{}, err
	}
	automatic, err := reports.Compare(ctx, baseRunID, candidateRunID)
	if err != nil {
		return ComparisonView{}, err
	}
	base, err := service.store.ListRatingSubmissions(ctx, baseRunID)
	if err != nil {
		return ComparisonView{}, err
	}
	candidate, err := service.store.ListRatingSubmissions(ctx, candidateRunID)
	if err != nil {
		return ComparisonView{}, err
	}
	view := ComparisonView{Automatic: automatic, BaseRatings: summarizeRatings(base), CandidateRatings: summarizeRatings(candidate)}
	dimensions := make([]string, 0, len(view.BaseRatings.DimensionAverage))
	for dimension := range view.BaseRatings.DimensionAverage {
		dimensions = append(dimensions, dimension)
	}
	sort.Strings(dimensions)
	for _, dimension := range dimensions {
		baseAverage := view.BaseRatings.DimensionAverage[dimension]
		candidateAverage, exists := view.CandidateRatings.DimensionAverage[dimension]
		if exists && candidateAverage-baseAverage < -0.5 {
			view.RatingRegressions = append(view.RatingRegressions, quality.HarnessReportRegression{Dimension: "human_rating", Name: dimension, Base: baseAverage, Candidate: candidateAverage, Delta: candidateAverage - baseAverage})
		}
	}
	baseAccept, candidateAccept := ratingRate(view.BaseRatings.Acceptable, view.BaseRatings.CandidateRatings), ratingRate(view.CandidateRatings.Acceptable, view.CandidateRatings.CandidateRatings)
	if candidateAccept-baseAccept < -0.1 {
		view.RatingRegressions = append(view.RatingRegressions, quality.HarnessReportRegression{Dimension: "acceptance_rate", Name: "可接受率", Base: baseAccept, Candidate: candidateAccept, Delta: candidateAccept - baseAccept})
	}
	baseRewrite, candidateRewrite := ratingRate(view.BaseRatings.Rewrite, view.BaseRatings.CandidateRatings), ratingRate(view.CandidateRatings.Rewrite, view.CandidateRatings.CandidateRatings)
	if candidateRewrite-baseRewrite > 0.05 {
		view.RatingRegressions = append(view.RatingRegressions, quality.HarnessReportRegression{Dimension: "rewrite_rate", Name: "重写率", Base: baseRewrite, Candidate: candidateRewrite, Delta: candidateRewrite - baseRewrite})
	}
	return view, nil
}

func (service *Service) Export(ctx context.Context, runID string) (string, error) {
	reports, err := quality.NewHarnessReportService(service.store)
	if err != nil {
		return "", err
	}
	payload, err := reports.Export(ctx, runID)
	if err != nil {
		return "", err
	}
	var report quality.HarnessReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return "", err
	}
	submissions, err := service.store.ListRatingSubmissions(ctx, runID)
	if err != nil {
		return "", err
	}
	export := struct {
		Report                quality.HarnessReport `json:"report"`
		Ratings               RatingSummary         `json:"ratings"`
		GeneratedTextIncluded bool                  `json:"generatedTextIncluded"`
	}{Report: report, Ratings: summarizeRatings(submissions), GeneratedTextIncluded: false}
	encoded, err := json.MarshalIndent(export, "", "  ")
	return string(encoded), err
}

func findVariant(id string) (quality.HarnessWorkflowVariant, error) {
	for _, variant := range quality.BuiltinHarnessWorkflowVariants() {
		if variant.ID == strings.TrimSpace(id) {
			return variant, nil
		}
	}
	return quality.HarnessWorkflowVariant{}, quality.ErrInvalidHarnessVariant
}

func selectCasesForPreview(cases []quality.HarnessCase, mode quality.HarnessMode, requested []string) []quality.HarnessCase {
	wanted := make(map[string]bool, len(requested))
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		wanted[id] = true
	}
	var result []quality.HarnessCase
	for _, item := range cases {
		explicit := wanted[item.ID]
		if len(wanted) > 0 && !explicit {
			continue
		}
		if mode == quality.HarnessModeFull || item.CIReduced || explicit {
			result = append(result, item)
		}
	}
	if len(wanted) > 0 && len(result) != len(wanted) {
		return nil
	}
	return result
}

func datasetCase(dataset quality.HarnessDataset, caseID string) (quality.HarnessCase, bool) {
	for _, item := range dataset.Cases {
		if item.ID == caseID {
			return item, true
		}
	}
	return quality.HarnessCase{}, false
}

func containsCaseID(caseIDs []string, wanted string) bool {
	for _, caseID := range caseIDs {
		if caseID == wanted {
			return true
		}
	}
	return false
}

func summarizeRatings(submissions []quality.HarnessRatingSubmission) RatingSummary {
	result := RatingSummary{Submissions: len(submissions), DimensionAverage: map[string]float64{}}
	counts := map[string]int{}
	for _, submission := range submissions {
		for _, rating := range submission.Ratings {
			result.CandidateRatings++
			if rating.Acceptance == quality.HarnessAcceptDirect || rating.Acceptance == quality.HarnessAcceptMinor {
				result.Acceptable++
			}
			if rating.Acceptance == quality.HarnessAcceptRewrite {
				result.Rewrite++
			}
			for dimension, score := range rating.Scores {
				result.DimensionAverage[dimension] += float64(score)
				counts[dimension]++
			}
		}
	}
	for dimension, total := range result.DimensionAverage {
		result.DimensionAverage[dimension] = total / float64(counts[dimension])
	}
	return result
}

func ratingRate(value, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) / float64(total)
}
