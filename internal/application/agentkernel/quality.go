package agentkernel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

// frozenQualitySpec 合并显式质量规格与章节硬锚点，形成可重放输入。
func frozenQualitySpec(request CreateRequest, taskKind agentdomain.TaskKind) (quality.Spec, error) {
	spec := cloneQualitySpec(request.QualitySpec)
	if taskKind != agentdomain.TaskChapterDraft && taskKind != agentdomain.TaskBaselineDraft {
		return spec, spec.Validate()
	}
	anchorByID := make(map[string]string, len(spec.RequiredAnchors))
	for _, anchor := range spec.RequiredAnchors {
		anchorByID[anchor.ID] = anchor.Text
	}
	for _, source := range request.Sources {
		if source.Kind != agentdomain.ManifestAnchor || !source.Required {
			continue
		}
		if existing, exists := anchorByID[source.ID]; exists {
			if existing != source.Content {
				return quality.Spec{}, quality.ErrInvalidInput
			}
			continue
		}
		spec.RequiredAnchors = append(spec.RequiredAnchors, quality.Constraint{ID: source.ID, Text: source.Content})
		anchorByID[source.ID] = source.Content
	}
	return spec, spec.Validate()
}

func cloneQualitySpec(spec quality.Spec) quality.Spec {
	spec.RequiredAnchors = append([]quality.Constraint(nil), spec.RequiredAnchors...)
	spec.ForbiddenItems = append([]quality.Constraint(nil), spec.ForbiddenItems...)
	spec.ReservedNames = append([]quality.Constraint(nil), spec.ReservedNames...)
	return spec
}

func (service *Service) applyQuality(
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	decoded agentdomain.TaskOutput,
	primaryContent string,
	modelQuality agentdomain.QualityStatus,
	modelFindings []agentdomain.ReviewFindingRecord,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	switch node.Capability {
	case agentdomain.CapabilityProseWriter:
		stage := quality.StageInitial
		if run.TaskKind == agentdomain.TaskBaselineDraft {
			stage = quality.StageBaseline
		}
		return service.lintCandidate(run, input, compiled, step, stage, primaryContent, modelFindings)
	case agentdomain.CapabilitySkeletonWriter:
		skeleton, ok := decoded.(*agentdomain.ChapterSkeletonOutput)
		if !ok {
			return "", "", nil, nil, agentdomain.ErrInvalidTaskOutput
		}
		report, err := service.quality.EvaluateSkeleton(compiled.QualityPolicyVersion, step.ID, *skeleton, input.QualitySpec.TargetRunes)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
		}
		return service.applyDeterministicReport(run, step, primaryContent, modelFindings, report)
	case agentdomain.CapabilityIntegrationWriter:
		return service.lintIntegratedSlots(run, input, compiled, node, step, latest, artifacts, primaryContent, modelFindings)
	case agentdomain.CapabilityContinuityReview, agentdomain.CapabilityContentReview, agentdomain.CapabilityStyleReview:
		return service.gateReview(run, node, step, latest, artifacts, decoded, modelFindings)
	case agentdomain.CapabilityBoundedRevision:
		return service.gateRevision(run, input, compiled, node, step, latest, artifacts, decoded, primaryContent, modelFindings)
	case agentdomain.CapabilityRevisionVerify:
		return service.gateVerification(run, input, compiled, node, step, latest, artifacts, decoded, primaryContent, modelFindings)
	default:
		return primaryContent, modelQuality, modelFindings, nil, nil
	}
}

func (service *Service) applyDeterministicReport(run agentdomain.Run, step agentdomain.RunStep, content string, existing []agentdomain.ReviewFindingRecord, report quality.Report) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	gate := quality.Decide(quality.StageInitial, report.Findings, !report.Truncated, true)
	artifacts, err := service.qualityArtifacts(run.ID, step.ID, report, gate)
	if err != nil {
		return "", "", nil, nil, err
	}
	findings := append(existing, findingRecords(report.Findings, run, step, service.clock.Now())...)
	return content, gate.Status, findings, artifacts, nil
}

func (service *Service) lintIntegratedSlots(run agentdomain.Run, input persistedInput, compiled CompiledWorkflow, node FrozenNode, step agentdomain.RunStep, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact, content string, existing []agentdomain.ReviewFindingRecord) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	base, err := service.quality.Evaluate(compiled.QualityPolicyVersion, quality.Input{Namespace: step.ID, Stage: quality.StageInitial, Text: content, Spec: input.QualitySpec})
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
	}
	skeleton, groups, join, err := slotQualityInputs(node, step, latest, artifacts)
	if err != nil {
		return "", "", nil, nil, err
	}
	slotReport, err := service.quality.EvaluateSlotIntegration(compiled.QualityPolicyVersion, step.ID+"-slot", skeleton, groups, join.Body, content)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
	}
	base.Findings = append(base.Findings, slotReport.Findings...)
	base.Truncated = base.Truncated || slotReport.Truncated
	return service.applyDeterministicReport(run, step, content, existing, base)
}

func slotQualityInputs(node FrozenNode, step agentdomain.RunStep, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (agentdomain.ChapterSkeletonOutput, []agentdomain.FillSlotOutput, agentdomain.SlotJoin, error) {
	skeleton, err := upstreamSkeleton(node, latest, artifacts)
	if err != nil {
		return agentdomain.ChapterSkeletonOutput{}, nil, agentdomain.SlotJoin{}, err
	}
	groups := make([]agentdomain.FillSlotOutput, 0, 5)
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		upstream, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || upstream.Capability != agentdomain.CapabilitySlotWriter {
			continue
		}
		artifact, exists := artifactByID(artifacts, upstream.PrimaryArtifactID)
		if !exists {
			return agentdomain.ChapterSkeletonOutput{}, nil, agentdomain.SlotJoin{}, fmt.Errorf("slot artifact missing")
		}
		decoded, decodeErr := agentdomain.DecodeTaskOutput(agentdomain.TaskFillSlot, []byte(artifact.Content))
		if decodeErr != nil {
			return agentdomain.ChapterSkeletonOutput{}, nil, agentdomain.SlotJoin{}, decodeErr
		}
		groups = append(groups, *decoded.(*agentdomain.FillSlotOutput))
	}
	joinArtifact, exists := slotJoinArtifact(step.ID, artifacts)
	if !exists {
		return agentdomain.ChapterSkeletonOutput{}, nil, agentdomain.SlotJoin{}, fmt.Errorf("slot join artifact missing")
	}
	var join agentdomain.SlotJoin
	if json.Unmarshal([]byte(joinArtifact.Content), &join) != nil {
		return agentdomain.ChapterSkeletonOutput{}, nil, agentdomain.SlotJoin{}, fmt.Errorf("slot join artifact invalid")
	}
	return skeleton, groups, join, nil
}

func (service *Service) lintCandidate(
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	step agentdomain.RunStep,
	stage quality.Stage,
	content string,
	existing []agentdomain.ReviewFindingRecord,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	report, err := service.quality.Evaluate(compiled.QualityPolicyVersion, quality.Input{
		Namespace: step.ID, Stage: stage, Text: content, Spec: input.QualitySpec,
	})
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
	}
	gate := quality.Decide(stage, report.Findings, !report.Truncated, true)
	qualityArtifacts, err := service.qualityArtifacts(run.ID, step.ID, report, gate)
	if err != nil {
		return "", "", nil, nil, err
	}
	findings := append(existing, findingRecords(report.Findings, run, step, service.clock.Now())...)
	return content, gate.Status, findings, qualityArtifacts, nil
}

func (service *Service) gateReview(
	run agentdomain.Run,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	decoded agentdomain.TaskOutput,
	modelFindings []agentdomain.ReviewFindingRecord,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	review, ok := decoded.(*agentdomain.ReviewBundleOutput)
	if !ok {
		return "", "", nil, nil, agentdomain.ErrInvalidTaskOutput
	}
	combined := append([]agentdomain.ReviewFinding(nil), review.Findings...)
	truncated := false
	if lint, found := candidateLintReport(node, latest, artifacts); found {
		combined = append(combined, lint.Findings...)
		truncated = lint.Truncated
	}
	combined, mergeTruncated, err := normalizeFindings(combined)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	truncated = truncated || mergeTruncated
	gate := quality.Decide(quality.StageInitial, combined, !truncated, true)
	normalized := *review
	normalized.QualityStatus = gate.Status
	if len(combined) > 0 && normalized.QualityStatus == agentdomain.QualityPassed {
		normalized.QualityStatus = agentdomain.QualityNeedsReview
	}
	normalized.Findings = combined
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", "", nil, nil, err
	}
	artifact, err := service.gateArtifact(run.ID, step.ID, gate)
	if err != nil {
		return "", "", nil, nil, err
	}
	return string(encoded), gate.Status, modelFindings, []agentdomain.Artifact{artifact}, nil
}

func (service *Service) gateRevision(
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	decoded agentdomain.TaskOutput,
	primaryContent string,
	existing []agentdomain.ReviewFindingRecord,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	// 作者定向修订没有待核实的评审问题：边界已由 validateBoundedRevision 冻结，
	// 门禁只记录本次修订的确定性通过状态。
	if input.RevisionDirective != nil && input.RevisionDirective.Mode == agentdomain.RevisionAuthorDirected {
		gate := quality.Decide(quality.StageFinal, nil, true, true)
		gateArtifact, err := service.gateArtifact(run.ID, step.ID, gate)
		if err != nil {
			return "", "", nil, nil, err
		}
		return primaryContent, gate.Status, existing, []agentdomain.Artifact{gateArtifact}, nil
	}
	_, reviewJSON, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return "", "", nil, nil, err
	}
	reviewOutput, err := agentdomain.DecodeTaskOutput(agentdomain.TaskReviewBundle, []byte(reviewJSON))
	if err != nil {
		return "", "", nil, nil, err
	}
	resolutions, err := revisionResolutions(decoded)
	if err != nil {
		return "", "", nil, nil, err
	}
	var lintReport quality.Report
	if run.TaskKind == agentdomain.TaskChapterDraft || run.TaskKind == agentdomain.TaskBaselineDraft {
		lintReport, err = service.quality.Evaluate(compiled.QualityPolicyVersion, quality.Input{
			Namespace: step.ID, Stage: quality.StageFinal, Text: primaryContent, Spec: input.QualitySpec,
		})
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
		}
	}
	allFindings, err := quality.Reconcile(reviewOutput.(*agentdomain.ReviewBundleOutput).Findings, resolutions, lintReport.Findings)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	reviewComplete := !lintReport.Truncated
	if priorGate, found := candidateGateReport(node, latest, artifacts); found {
		reviewComplete = reviewComplete && priorGate.ReviewComplete
	}
	gate := quality.Decide(quality.StageFinal, allFindings, reviewComplete, true)
	qualityArtifacts := make([]agentdomain.Artifact, 0, 2)
	if lintReport.PolicyVersion != 0 {
		lintArtifact, marshalErr := service.lintArtifact(run.ID, step.ID, lintReport)
		if marshalErr != nil {
			return "", "", nil, nil, marshalErr
		}
		qualityArtifacts = append(qualityArtifacts, lintArtifact)
	}
	gateArtifact, err := service.gateArtifact(run.ID, step.ID, gate)
	if err != nil {
		return "", "", nil, nil, err
	}
	qualityArtifacts = append(qualityArtifacts, gateArtifact)
	findings := append(existing, findingRecords(lintReport.Findings, run, step, service.clock.Now())...)
	return primaryContent, gate.Status, findings, qualityArtifacts, nil
}

// gateVerification 用独立验证结论取代修订自述作为最终质量结论。
// 修订模型声称已解决只是待核实声明；只有验证者也确认，问题才退出未解决计数。
func (service *Service) gateVerification(
	run agentdomain.Run,
	input persistedInput,
	compiled CompiledWorkflow,
	node FrozenNode,
	step agentdomain.RunStep,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	decoded agentdomain.TaskOutput,
	primaryContent string,
	existing []agentdomain.ReviewFindingRecord,
) (string, agentdomain.QualityStatus, []agentdomain.ReviewFindingRecord, []agentdomain.Artifact, error) {
	verdict, ok := decoded.(*agentdomain.RevisionVerdictOutput)
	if !ok {
		return "", "", nil, nil, agentdomain.ErrInvalidTaskOutput
	}
	findings, resolutions, _, revised, err := verificationInputs(node, latest, artifacts)
	if err != nil {
		return "", "", nil, nil, err
	}
	var lintReport quality.Report
	if run.TaskKind == agentdomain.TaskChapterDraft || run.TaskKind == agentdomain.TaskBaselineDraft {
		lintReport, err = service.quality.Evaluate(compiled.QualityPolicyVersion, quality.Input{
			Namespace: step.ID, Stage: quality.StageFinal, Text: revised, Spec: input.QualitySpec,
		})
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("%w: %v", ErrQualityPolicy, err)
		}
	}
	allFindings, verification, err := quality.ReconcileVerified(
		findings, resolutions, verdict.Verdicts, verdict.Regressions, lintReport.Findings,
	)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	reviewComplete := !lintReport.Truncated
	if priorGate, found := verificationPriorGate(node, latest, artifacts); found {
		reviewComplete = reviewComplete && priorGate.ReviewComplete
	}
	gate := quality.Decide(quality.StageFinal, allFindings, reviewComplete, true)
	gate.Verification = &verification
	qualityArtifacts := make([]agentdomain.Artifact, 0, 2)
	if lintReport.PolicyVersion != 0 {
		lintArtifact, marshalErr := service.lintArtifact(run.ID, step.ID, lintReport)
		if marshalErr != nil {
			return "", "", nil, nil, marshalErr
		}
		qualityArtifacts = append(qualityArtifacts, lintArtifact)
	}
	gateArtifact, err := service.gateArtifact(run.ID, step.ID, gate)
	if err != nil {
		return "", "", nil, nil, err
	}
	qualityArtifacts = append(qualityArtifacts, gateArtifact)
	now := service.clock.Now()
	records := append(existing, findingRecords(verdict.Regressions, run, step, now)...)
	records = append(records, findingRecords(lintReport.Findings, run, step, now)...)
	return primaryContent, gate.Status, records, qualityArtifacts, nil
}

// verificationPriorGate 继承修订门禁的审阅完整性，避免上游截断在复核阶段被遗忘。
func verificationPriorGate(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (quality.GateReport, bool) {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || step.Capability != agentdomain.CapabilityBoundedRevision {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.StepID != step.ID || artifact.Kind != agentdomain.ArtifactQualityReport {
				continue
			}
			var report quality.GateReport
			if json.Unmarshal([]byte(artifact.Content), &report) == nil {
				return report, true
			}
		}
	}
	return quality.GateReport{}, false
}

func revisionResolutions(output agentdomain.TaskOutput) ([]agentdomain.FindingResolution, error) {
	switch value := output.(type) {
	case *agentdomain.ReviseDraftOutput:
		return append([]agentdomain.FindingResolution(nil), value.Resolutions...), nil
	case *agentdomain.ReviseBlueprintOutput:
		return append([]agentdomain.FindingResolution(nil), value.Resolutions...), nil
	default:
		return nil, agentdomain.ErrInvalidTaskOutput
	}
}

func candidateLintReport(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (quality.Report, bool) {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || (step.Capability != agentdomain.CapabilityProseWriter && step.Capability != agentdomain.CapabilityIntegrationWriter) {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.StepID != step.ID || artifact.Kind != agentdomain.ArtifactLintReport {
				continue
			}
			var report quality.Report
			if json.Unmarshal([]byte(artifact.Content), &report) == nil {
				return report, true
			}
		}
	}
	return quality.Report{}, false
}

func candidateGateReport(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) (quality.GateReport, bool) {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || (step.Capability != agentdomain.CapabilityContinuityReview && step.Capability != agentdomain.CapabilityContentReview && step.Capability != agentdomain.CapabilityStyleReview) {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.StepID != step.ID || artifact.Kind != agentdomain.ArtifactQualityReport {
				continue
			}
			var report quality.GateReport
			if json.Unmarshal([]byte(artifact.Content), &report) == nil {
				return report, true
			}
		}
	}
	return quality.GateReport{}, false
}

func normalizeFindings(findings []agentdomain.ReviewFinding) ([]agentdomain.ReviewFinding, bool, error) {
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		if _, exists := seen[finding.ID]; exists {
			return nil, false, fmt.Errorf("duplicate finding %s", finding.ID)
		}
		seen[finding.ID] = struct{}{}
	}
	if len(findings) <= agentdomain.MaxReviewFindings {
		return findings, false, nil
	}
	ordered := append([]agentdomain.ReviewFinding(nil), findings...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return severityRank(ordered[left].Severity) < severityRank(ordered[right].Severity)
	})
	return ordered[:agentdomain.MaxReviewFindings], true, nil
}

func severityRank(severity agentdomain.FindingSeverity) int {
	switch severity {
	case agentdomain.SeverityBlocker:
		return 0
	case agentdomain.SeverityMajor:
		return 1
	case agentdomain.SeverityMinor:
		return 2
	default:
		return 3
	}
}

func (service *Service) qualityArtifacts(runID, stepID string, report quality.Report, gate quality.GateReport) ([]agentdomain.Artifact, error) {
	lint, err := service.lintArtifact(runID, stepID, report)
	if err != nil {
		return nil, err
	}
	qualityReport, err := service.gateArtifact(runID, stepID, gate)
	if err != nil {
		return nil, err
	}
	return []agentdomain.Artifact{lint, qualityReport}, nil
}

func (service *Service) lintArtifact(runID, stepID string, report quality.Report) (agentdomain.Artifact, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return agentdomain.Artifact{}, err
	}
	return newArtifact(service.ids.NewID(), runID, stepID, agentdomain.ArtifactLintReport, string(encoded), service.clock.Now()), nil
}

func (service *Service) gateArtifact(runID, stepID string, report quality.GateReport) (agentdomain.Artifact, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return agentdomain.Artifact{}, err
	}
	return newArtifact(service.ids.NewID(), runID, stepID, agentdomain.ArtifactQualityReport, string(encoded), service.clock.Now()), nil
}

func findingRecords(findings []agentdomain.ReviewFinding, run agentdomain.Run, step agentdomain.RunStep, now time.Time) []agentdomain.ReviewFindingRecord {
	result := make([]agentdomain.ReviewFindingRecord, 0, len(findings))
	for _, finding := range findings {
		result = append(result, agentdomain.ReviewFindingRecord{
			ReviewFinding: finding, RunID: run.ID, StepID: step.ID, Attempt: step.Attempt,
			CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		})
	}
	return result
}
