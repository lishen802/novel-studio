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
