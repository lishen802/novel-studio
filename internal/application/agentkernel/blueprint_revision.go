package agentkernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type blueprintFieldChange struct {
	ref    string
	before string
	after  string
}

// decodeBlueprintRevisionOutput 在严格领域校验前修复模型常见的追踪 ID 与安全结构漂移。
func decodeBlueprintRevisionOutput(
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	content string,
) (agentdomain.TaskOutput, error) {
	var revision agentdomain.ReviseBlueprintOutput
	if err := agentdomain.UnmarshalModelJSON([]byte(content), &revision); err != nil {
		return nil, err
	}
	original, reviewJSON, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return nil, err
	}
	decodedReview, err := agentdomain.DecodeTaskOutput(agentdomain.TaskReviewBundle, []byte(reviewJSON))
	if err != nil {
		return nil, err
	}
	if err := normalizeBlueprintRevision(decodedReview.(*agentdomain.ReviewBundleOutput), original, &revision); err != nil {
		return nil, err
	}
	if err := revision.Validate(); err != nil {
		return nil, err
	}
	return &revision, nil
}

// normalizeBlueprintRevision 将可确定恢复的格式漂移收口，并只保留能映射到方案字段的修订。
func normalizeBlueprintRevision(
	review *agentdomain.ReviewBundleOutput,
	original string,
	revision *agentdomain.ReviseBlueprintOutput,
) error {
	if review == nil || revision == nil || strings.TrimSpace(original) == "" {
		return agentdomain.ErrInvalidTaskOutput
	}
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryBlueprint, []byte(original))
	if err != nil {
		return err
	}
	before := decoded.(*agentdomain.StoryBlueprintOutput)
	normalizeBlueprintShape(&revision.Blueprint, *before)
	changed := make(map[string]blueprintFieldChange)
	for _, field := range blueprintFields(*before, revision.Blueprint) {
		if field.before != field.after {
			changed[field.ref] = field
		}
	}
	assigned := assignDraftResolutions(review.Findings, revision.Resolutions)
	usedChanges := make([]bool, len(revision.Changes))
	normalizedChanges := make([]agentdomain.RevisionChange, 0, len(review.Findings))
	for index := range assigned {
		item := &assigned[index]
		resolution := &item.resolution
		if strings.TrimSpace(resolution.Note) == "" {
			resolution.Note = "模型未提供处理说明。"
		}
		if resolution.Status != agentdomain.FindingResolved &&
			resolution.Status != agentdomain.FindingRetained &&
			resolution.Status != agentdomain.FindingCannotResolve {
			resolution.Status = agentdomain.FindingCannotResolve
		}
		if resolution.Status != agentdomain.FindingResolved {
			continue
		}
		changeIndex := matchingDraftChange(revision.Changes, usedChanges, item.modelID, resolution.FindingID)
		if changeIndex < 0 {
			changeIndex = nextUnusedResolution(usedChanges)
		}
		var declared agentdomain.RevisionChange
		if changeIndex >= 0 {
			declared = revision.Changes[changeIndex]
		}
		field, resolveErr := resolveBlueprintField(review.Findings[index], declared, changed)
		if resolveErr != nil {
			resolution.Status = agentdomain.FindingCannotResolve
			continue
		}
		if changeIndex >= 0 {
			usedChanges[changeIndex] = true
		}
		normalizedChanges = append(normalizedChanges, agentdomain.RevisionChange{
			FindingID: resolution.FindingID, OriginalText: field.before, RevisedText: field.after,
		})
	}
	revision.Resolutions = make([]agentdomain.FindingResolution, len(assigned))
	for index := range assigned {
		revision.Resolutions[index] = assigned[index].resolution
	}
	revision.Changes = normalizedChanges
	return nil
}

// normalizeBlueprintShape 对空字段和重复列表做无歧义恢复，不替模型创造新方案内容。
func normalizeBlueprintShape(target *agentdomain.StoryBlueprintOutput, fallback agentdomain.StoryBlueprintOutput) {
	if strings.TrimSpace(target.Title) == "" {
		target.Title = fallback.Title
	}
	if strings.TrimSpace(target.Positioning) == "" {
		target.Positioning = fallback.Positioning
	}
	if strings.TrimSpace(target.CoreConflict) == "" {
		target.CoreConflict = fallback.CoreConflict
	}
	if strings.TrimSpace(target.ProtagonistArc) == "" {
		target.ProtagonistArc = fallback.ProtagonistArc
	}
	target.WorldRules = normalizeBlueprintList(target.WorldRules, fallback.WorldRules, 1)
	target.MajorMilestones = normalizeBlueprintList(target.MajorMilestones, fallback.MajorMilestones, 3)
	// 修订协议只覆盖六个基础字段，作品方案 v2 的其余字段必须沿用规划稿。
	target.SellPoint = fallback.SellPoint
	target.Genre = fallback.Genre
	target.GrowthCost = fallback.GrowthCost
	target.Pacing = fallback.Pacing
	target.Ending = fallback.Ending
	target.Subplots = append([]string(nil), fallback.Subplots...)
	target.Forbidden = append([]string(nil), fallback.Forbidden...)
	target.Anchors = append([]agentdomain.BlueprintAnchor(nil), fallback.Anchors...)
}

func normalizeBlueprintList(values, fallback []string, minimum int) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) < minimum || len(normalized) > agentdomain.MaxBlueprintItems {
		return append([]string(nil), fallback...)
	}
	return normalized
}

// validateBlueprintRevision 按结构字段核对修订边界，并把模型证据归一化为精确字段前后值。
func validateBlueprintRevision(
	original string,
	review *agentdomain.ReviewBundleOutput,
	revision *agentdomain.ReviseBlueprintOutput,
	sources []Source,
) error {
	if review == nil || revision == nil {
		return agentdomain.ErrInvalidTaskOutput
	}
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryBlueprint, []byte(original))
	if err != nil {
		return err
	}
	if err := validateResolutionCoverage(review.Findings, revision.Resolutions); err != nil {
		return err
	}
	before := decoded.(*agentdomain.StoryBlueprintOutput)
	fields := blueprintFields(*before, revision.Blueprint)
	changed := make(map[string]blueprintFieldChange)
	for _, field := range fields {
		if field.before != field.after {
			changed[field.ref] = field
		}
	}
	findings := make(map[string]agentdomain.ReviewFinding, len(review.Findings))
	for _, finding := range review.Findings {
		findings[finding.ID] = finding
	}
	changeIndexes := make(map[string]int, len(revision.Changes))
	for index, change := range revision.Changes {
		changeIndexes[change.FindingID] = index
	}
	covered := make(map[string]struct{}, len(changed))
	for _, resolution := range revision.Resolutions {
		if resolution.Status != agentdomain.FindingResolved {
			continue
		}
		finding, exists := findings[resolution.FindingID]
		changeIndex, hasChange := changeIndexes[resolution.FindingID]
		if !exists || !hasChange {
			return agentdomain.ErrInvalidTaskOutput
		}
		field, err := resolveBlueprintField(finding, revision.Changes[changeIndex], changed)
		if err != nil {
			return fmt.Errorf("revision finding %s: %w", resolution.FindingID, err)
		}
		revision.Changes[changeIndex].OriginalText = field.before
		revision.Changes[changeIndex].RevisedText = field.after
		covered[field.ref] = struct{}{}
	}
	for ref := range changed {
		if _, exists := covered[ref]; !exists {
			// 模型偶发润色无关字段时不让整次付费运行失败，确定性恢复原值。
			restoreBlueprintField(&revision.Blueprint, *before, ref)
		}
	}
	encoded, err := json.Marshal(revision.Blueprint)
	if err != nil {
		return err
	}
	revised := string(encoded)
	for _, source := range sources {
		if source.Kind == agentdomain.ManifestAnchor && source.Required && strings.Contains(original, source.Content) && !strings.Contains(revised, source.Content) {
			return fmt.Errorf("revision lost required anchor %s", source.ID)
		}
	}
	return nil
}

func restoreBlueprintField(target *agentdomain.StoryBlueprintOutput, before agentdomain.StoryBlueprintOutput, ref string) {
	switch ref {
	case "title":
		target.Title = before.Title
	case "positioning":
		target.Positioning = before.Positioning
	case "core_conflict":
		target.CoreConflict = before.CoreConflict
	case "protagonist_arc":
		target.ProtagonistArc = before.ProtagonistArc
	case "world_rules":
		target.WorldRules = append([]string(nil), before.WorldRules...)
	case "major_milestones":
		target.MajorMilestones = append([]string(nil), before.MajorMilestones...)
	}
}

func blueprintFields(before, after agentdomain.StoryBlueprintOutput) []blueprintFieldChange {
	return []blueprintFieldChange{
		{ref: "title", before: before.Title, after: after.Title},
		{ref: "positioning", before: before.Positioning, after: after.Positioning},
		{ref: "core_conflict", before: before.CoreConflict, after: after.CoreConflict},
		{ref: "protagonist_arc", before: before.ProtagonistArc, after: after.ProtagonistArc},
		{ref: "world_rules", before: encodeBlueprintList(before.WorldRules), after: encodeBlueprintList(after.WorldRules)},
		{ref: "major_milestones", before: encodeBlueprintList(before.MajorMilestones), after: encodeBlueprintList(after.MajorMilestones)},
	}
}

func encodeBlueprintList(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func resolveBlueprintField(
	finding agentdomain.ReviewFinding,
	declared agentdomain.RevisionChange,
	changed map[string]blueprintFieldChange,
) (blueprintFieldChange, error) {
	if ref := blueprintConstraintField(finding.ConstraintRef); ref != "" {
		if field, exists := changed[ref]; exists {
			return field, nil
		}
		return blueprintFieldChange{}, errors.New("finding targets an unchanged blueprint field")
	}
	candidates := make(map[string]blueprintFieldChange)
	for ref, field := range changed {
		quoteMatches := strings.TrimSpace(finding.Quote) != "" && strings.Contains(field.before, finding.Quote)
		declaredMatches := strings.Contains(field.before, declared.OriginalText) && strings.Contains(field.after, declared.RevisedText)
		if quoteMatches || declaredMatches {
			candidates[ref] = field
		}
	}
	if len(candidates) == 1 {
		for _, field := range candidates {
			return field, nil
		}
	}
	if len(candidates) > 1 {
		return blueprintFieldChange{}, errors.New("change evidence matches multiple blueprint fields")
	}
	if len(changed) == 1 {
		for _, field := range changed {
			return field, nil
		}
	}
	return blueprintFieldChange{}, errors.New("cannot map change evidence to a blueprint field")
}

func blueprintConstraintField(ref string) string {
	const prefix = "blueprint:"
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	field := strings.TrimPrefix(ref, prefix)
	switch field {
	case "title", "positioning", "core_conflict", "protagonist_arc", "world_rules", "major_milestones":
		return field
	default:
		return ""
	}
}
