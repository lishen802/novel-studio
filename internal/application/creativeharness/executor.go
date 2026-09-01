package creativeharness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

type kernelExecutor struct {
	profiles    ports.CreativeProfileResolver
	credentials ports.CredentialStore
	providers   ports.ModelProviderFactory
	registry    *agentassets.Registry
	clock       ports.Clock
	ids         ports.IDGenerator
}

func (executor *kernelExecutor) Execute(ctx context.Context, item quality.HarnessCase, binding quality.HarnessRunBinding) ([]quality.HarnessCandidate, error) {
	return executor.ExecuteWithProgress(ctx, item, binding, func(quality.HarnessExecutionProgress) {})
}

func (executor *kernelExecutor) ExecuteWithProgress(ctx context.Context, item quality.HarnessCase, binding quality.HarnessRunBinding, onProgress func(quality.HarnessExecutionProgress)) ([]quality.HarnessCandidate, error) {
	profile, err := executor.profiles.ResolveCreativeProfile(ctx, binding.ProfileID, binding.ProfileVersion)
	if err != nil {
		return nil, fmt.Errorf("解析实验创作方案: %w", err)
	}
	workflowID, workflowVersion := binding.WorkflowID, binding.WorkflowRevision
	if item.TaskKind == quality.HarnessTaskBlueprint {
		workflowID, workflowVersion = agentassets.QualityBlueprintWorkflowID, agentassets.QualityBlueprintWorkflowVersion
	}
	if workflowID == "" || workflowVersion < 1 {
		workflowID, workflowVersion = agentassets.QualityChapterWorkflowID, agentassets.QualityChapterWorkflowVersion
	}
	candidates := make([]quality.HarnessCandidate, 0, 3)
	inputOffset, outputOffset, callsOffset := 0, 0, 0
	progress := func(stage string, inputTokens, outputTokens, calls int) {
		onProgress(quality.HarnessExecutionProgress{CaseID: item.ID, PublicStageID: stage, InputTokens: inputOffset + inputTokens, OutputTokens: outputOffset + outputTokens, Calls: callsOffset + calls})
	}
	if item.TaskKind == quality.HarnessTaskChapter {
		baseline, err := executor.executeWorkflow(ctx, item, profile, agentassets.BaselineWorkflowID, agentassets.BaselineWorkflowVersion, quality.CandidateBaseline, progress)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, baseline)
		inputOffset, outputOffset, callsOffset = baseline.Metrics.InputTokens, baseline.Metrics.OutputTokens, baseline.Metrics.Calls
	}
	initial, revised, err := executor.executeQualityWorkflow(ctx, item, profile, workflowID, workflowVersion, progress)
	if err != nil {
		return nil, err
	}
	return append(candidates, initial, revised), nil
}

func (executor *kernelExecutor) executeWorkflow(ctx context.Context, item quality.HarnessCase, profile agentdomain.ResolvedCreativeProfile, workflowID string, workflowVersion int, kind quality.HarnessCandidateKind, onProgress func(string, int, int, int)) (quality.HarnessCandidate, error) {
	store, run, elapsed, err := executor.runKernel(ctx, item, profile, workflowID, workflowVersion, onProgress)
	if err != nil {
		return quality.HarnessCandidate{}, err
	}
	artifacts, _ := store.ListAgentArtifacts(ctx, run.ID)
	content := finalArtifactContent(run, artifacts, item.TaskKind)
	if content == "" {
		return quality.HarnessCandidate{}, errorsNoCandidate(workflowID)
	}
	candidate := candidateFromRun(kind, content, run, artifacts, elapsed, func(agentdomain.RunStep) bool { return true })
	applyCandidateEvidence(&candidate, run, artifacts)
	return candidate, nil
}

func (executor *kernelExecutor) executeQualityWorkflow(ctx context.Context, item quality.HarnessCase, profile agentdomain.ResolvedCreativeProfile, workflowID string, workflowVersion int, onProgress func(string, int, int, int)) (quality.HarnessCandidate, quality.HarnessCandidate, error) {
	store, run, elapsed, err := executor.runKernel(ctx, item, profile, workflowID, workflowVersion, onProgress)
	if err != nil {
		return quality.HarnessCandidate{}, quality.HarnessCandidate{}, err
	}
	artifacts, _ := store.ListAgentArtifacts(ctx, run.ID)
	initialContent := initialArtifactContent(run, artifacts, item.TaskKind)
	revisedContent := revisionArtifactContent(run, artifacts)
	if revisedContent == "" {
		revisedContent = initialContent
	}
	if initialContent == "" || revisedContent == "" {
		return quality.HarnessCandidate{}, quality.HarnessCandidate{}, errorsNoCandidate(workflowID)
	}
	initial := candidateFromRun(quality.CandidateInitial, initialContent, run, artifacts, elapsed, func(step agentdomain.RunStep) bool {
		return step.Capability == agentdomain.CapabilityStoryArchitect || step.Capability == agentdomain.CapabilityChapterPlanner || step.Capability == agentdomain.CapabilityProseWriter
	})
	revised := candidateFromRun(quality.CandidateRevised, revisedContent, run, artifacts, elapsed, func(step agentdomain.RunStep) bool {
		return step.Capability != agentdomain.CapabilityStoryArchitect && step.Capability != agentdomain.CapabilityChapterPlanner && step.Capability != agentdomain.CapabilityProseWriter
	})
	applyCandidateEvidence(&initial, run, artifacts)
	applyCandidateEvidence(&revised, run, artifacts)
	return initial, revised, nil
}

func (executor *kernelExecutor) runKernel(ctx context.Context, item quality.HarnessCase, profile agentdomain.ResolvedCreativeProfile, workflowID string, workflowVersion int, onProgress func(string, int, int, int)) (*memoryAgentStore, agentdomain.Run, time.Duration, error) {
	store := newMemoryAgentStore()
	service := agentkernel.NewService(store, executor.credentials, executor.providers, agentkernel.Registries{Workflows: executor.registry, Prompts: executor.registry, Schemas: executor.registry, Assemblers: executor.registry}, executor.clock, executor.ids, func(callbackContext context.Context, runID string) {
		run, loadErr := store.AgentRun(callbackContext, runID)
		if loadErr == nil {
			stage, calls := kernelRunProgress(run)
			artifacts, _ := store.ListAgentArtifacts(callbackContext, runID)
			calls += countFormatRepairCalls(artifacts, nil)
			onProgress(stage, run.InputTokens, run.OutputTokens, calls)
		}
	})
	runID := executor.ids.NewID()
	request := agentkernel.CreateRequest{
		RunID: runID, ProjectID: "harness", BranchID: "isolated", ChapterID: item.ID, ChapterBaseRevision: 1,
		CanonicalHead: "harness:" + item.ID, AuthorInstruction: harnessInstruction(item), WorkflowID: workflowID, WorkflowVersion: workflowVersion,
		Profile: profile, Sources: harnessSources(item), QualitySpec: harnessQualitySpec(item), Deadline: executor.clock.Now().Add(30 * time.Minute),
	}
	run, err := service.CreateRun(ctx, request)
	if err != nil {
		return store, agentdomain.Run{}, 0, fmt.Errorf("创建实验 Agent 运行: %w", err)
	}
	started := time.Now()
	run, err = service.ExecuteRun(ctx, run.ID, request.CanonicalHead)
	if err != nil {
		cause := fmt.Errorf("执行实验 Agent 运行: %w", err)
		return store, run, time.Since(started), quality.NewHarnessExecutionError(run.ErrorMessage, cause)
	}
	return store, run, time.Since(started), nil
}

func kernelRunProgress(run agentdomain.Run) (string, int) {
	stage := "preparing"
	calls := 0
	for _, step := range run.Steps {
		if step.Status == agentdomain.StepRunning || step.Status == agentdomain.StepFailed || step.Status == agentdomain.StepCancelled {
			stage = string(step.PublicStageID)
		}
		if step.Status == agentdomain.StepCompleted {
			stage = string(step.PublicStageID)
			calls++
		} else if (step.Status == agentdomain.StepFailed || step.Status == agentdomain.StepCancelled) &&
			(step.InputTokens > 0 || step.OutputTokens > 0 || step.ProviderRequestID != "") {
			// 失败不会抹掉已经发生的付费调用；观察器必须如实计数。
			calls++
		}
	}
	if run.Status == agentdomain.RunCompleted {
		stage = "candidate_ready"
	}
	return stage, calls
}

func harnessInstruction(item quality.HarnessCase) string {
	if item.TaskKind == quality.HarnessTaskBlueprint {
		return "根据原创种子和约束生成可持续发展的长篇故事策划。"
	}
	return "根据原创设定、章节目标和人物声音写出完整章节候选。"
}

func harnessSources(item quality.HarnessCase) []agentkernel.Source {
	content := fmt.Sprintf("原创种子：%s\n故事设定：%s\n章节目标：%s\n确认约束：%v\n人物声音：%v", item.OriginalSeed, item.StorySpec, item.ChapterGoal, item.ConfirmedConstraints, item.CharacterVoices)
	return []agentkernel.Source{{Kind: agentdomain.ManifestHarnessSpec, ID: item.ID, Version: item.DatasetVersion, Content: content, Priority: 100, SelectionReason: "固定原创 Harness case", Required: true}}
}

func harnessQualitySpec(item quality.HarnessCase) quality.Spec {
	spec := quality.Spec{TargetRunes: item.TargetLength}
	for index, anchor := range item.RequiredAnchors {
		spec.RequiredAnchors = append(spec.RequiredAnchors, quality.Constraint{ID: fmt.Sprintf("required-%d", index+1), Text: anchor})
	}
	for index, forbidden := range item.ForbiddenItems {
		spec.ForbiddenItems = append(spec.ForbiddenItems, quality.Constraint{ID: fmt.Sprintf("forbidden-%d", index+1), Text: forbidden})
	}
	return spec
}

func initialArtifactContent(run agentdomain.Run, artifacts []agentdomain.Artifact, taskKind quality.HarnessTaskKind) string {
	byID := make(map[string]agentdomain.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	wanted := agentdomain.ArtifactPlan
	if taskKind == quality.HarnessTaskChapter {
		wanted = agentdomain.ArtifactDraft
	}
	for _, step := range run.Steps {
		artifact := byID[step.PrimaryArtifactID]
		if artifact.Kind == wanted {
			return artifact.Content
		}
	}
	return ""
}

func revisionArtifactContent(run agentdomain.Run, artifacts []agentdomain.Artifact) string {
	byID := make(map[string]agentdomain.Artifact, len(artifacts))
	for _, artifact := range artifacts {
		byID[artifact.ID] = artifact
	}
	for index := len(run.Steps) - 1; index >= 0; index-- {
		if artifact := byID[run.Steps[index].PrimaryArtifactID]; artifact.Kind == agentdomain.ArtifactRevision {
			return artifact.Content
		}
	}
	return ""
}

func finalArtifactContent(run agentdomain.Run, artifacts []agentdomain.Artifact, taskKind quality.HarnessTaskKind) string {
	if revised := revisionArtifactContent(run, artifacts); revised != "" {
		return revised
	}
	return initialArtifactContent(run, artifacts, taskKind)
}

func candidateFromRun(kind quality.HarnessCandidateKind, content string, run agentdomain.Run, artifacts []agentdomain.Artifact, elapsed time.Duration, include func(agentdomain.RunStep) bool) quality.HarnessCandidate {
	metrics := quality.HarnessCandidateMetrics{GatePassed: run.QualityStatus == agentdomain.QualityPassed}
	includedSteps := make(map[string]struct{})
	for _, step := range run.Steps {
		if step.Status == agentdomain.StepCompleted && include(step) {
			includedSteps[step.ID] = struct{}{}
			metrics.Calls++
			metrics.InputTokens += step.InputTokens
			metrics.OutputTokens += step.OutputTokens
			if step.StartedAt != nil && step.CompletedAt != nil {
				metrics.DurationMillis += step.CompletedAt.Sub(*step.StartedAt).Milliseconds()
			}
		}
	}
	metrics.Calls += countFormatRepairCalls(artifacts, includedSteps)
	if metrics.DurationMillis == 0 && metrics.Calls > 0 {
		metrics.DurationMillis = elapsed.Milliseconds() * int64(metrics.Calls) / int64(max(1, len(run.Steps)))
	}
	return quality.HarnessCandidate{Kind: kind, Content: content, Metrics: metrics}
}

func countFormatRepairCalls(artifacts []agentdomain.Artifact, includedSteps map[string]struct{}) int {
	calls := 0
	for _, artifact := range artifacts {
		if artifact.Kind != agentdomain.ArtifactFormatRepair {
			continue
		}
		if includedSteps != nil {
			if _, included := includedSteps[artifact.StepID]; !included {
				continue
			}
		}
		calls++
	}
	return calls
}

func errorsNoCandidate(workflowID string) error {
	return fmt.Errorf("工作流 %s 未产生可阅读候选", workflowID)
}

func applyCandidateEvidence(candidate *quality.HarnessCandidate, run agentdomain.Run, artifacts []agentdomain.Artifact) {
	stepID := candidateStepID(run, candidate.Kind)
	for _, artifact := range artifacts {
		if artifact.StepID != stepID || artifact.Kind != agentdomain.ArtifactQualityReport || !json.Valid([]byte(artifact.Content)) {
			continue
		}
		candidate.QualityReportJSON = artifact.Content
		var gate quality.GateReport
		if json.Unmarshal([]byte(artifact.Content), &gate) != nil {
			return
		}
		candidate.Metrics.GatePassed = gate.Status == agentdomain.QualityPassed
		candidate.Metrics.Blockers = gate.Unresolved.Blocker
		candidate.Metrics.Majors = gate.Unresolved.Major
		candidate.Metrics.Minors = gate.Unresolved.Minor
		for _, finding := range gate.Findings {
			if finding.Status == agentdomain.FindingResolved {
				candidate.Metrics.Resolved++
			}
		}
		return
	}
}

func candidateStepID(run agentdomain.Run, kind quality.HarnessCandidateKind) string {
	var initial string
	for _, step := range run.Steps {
		if step.Capability == agentdomain.CapabilityStoryArchitect || step.Capability == agentdomain.CapabilityProseWriter {
			initial = step.ID
		}
		if kind == quality.CandidateRevised && step.Capability == agentdomain.CapabilityBoundedRevision && step.Status == agentdomain.StepCompleted {
			return step.ID
		}
	}
	return initial
}
