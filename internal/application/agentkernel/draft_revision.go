package agentkernel

import (
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// decodeDraftRevisionOutput 在领域校验前收口模型容易违反的修订追踪关系。
func decodeDraftRevisionOutput(
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	content string,
) (agentdomain.TaskOutput, error) {
	var revision agentdomain.ReviseDraftOutput
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
	if strings.TrimSpace(revision.Title) == "" {
		revision.Title = originalDraftTitle(node, latest, artifacts)
	}
	if err := normalizeDraftRevision(decodedReview.(*agentdomain.ReviewBundleOutput), original, &revision); err != nil {
		return nil, err
	}
	if err := revision.Validate(); err != nil {
		return nil, err
	}
	return &revision, nil
}

// originalDraftTitle 从同一冻结运行的结构化草稿中恢复模型偶发遗漏的标题。
func originalDraftTitle(node FrozenNode, latest map[string]agentdomain.RunStep, artifacts []agentdomain.Artifact) string {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || step.Capability != agentdomain.CapabilityProseWriter {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.StepID != step.ID || artifact.Kind != agentdomain.ArtifactStructuredOutput {
				continue
			}
			decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskChapterDraft, []byte(artifact.Content))
			if err == nil {
				return decoded.(*agentdomain.ChapterDraftOutput).Title
			}
		}
	}
	return ""
}

type assignedResolution struct {
	resolution agentdomain.FindingResolution
	modelID    string
}

// normalizeDraftRevision 只保留能够在原文上精确应用的声明变更，并恢复所有越界润色。
func normalizeDraftRevision(
	review *agentdomain.ReviewBundleOutput,
	original string,
	revision *agentdomain.ReviseDraftOutput,
) error {
	if review == nil || revision == nil || strings.TrimSpace(revision.Title) == "" || strings.TrimSpace(original) == "" {
		return agentdomain.ErrInvalidTaskOutput
	}
	assigned := assignDraftResolutions(review.Findings, revision.Resolutions)
	usedChanges := make([]bool, len(revision.Changes))
	normalizedChanges := make([]agentdomain.RevisionChange, 0, len(review.Findings))
	expected := original
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
			resolution.Status = agentdomain.FindingCannotResolve
			continue
		}
		change := revision.Changes[changeIndex]
		if strings.TrimSpace(change.OriginalText) == "" || strings.TrimSpace(change.RevisedText) == "" ||
			change.OriginalText == change.RevisedText || !strings.Contains(expected, change.OriginalText) {
			resolution.Status = agentdomain.FindingCannotResolve
			continue
		}
		usedChanges[changeIndex] = true
		change.FindingID = resolution.FindingID
		change.TargetRef = ""
		expected = strings.Replace(expected, change.OriginalText, change.RevisedText, 1)
		normalizedChanges = append(normalizedChanges, change)
	}
	revision.Resolutions = make([]agentdomain.FindingResolution, len(assigned))
	for index := range assigned {
		revision.Resolutions[index] = assigned[index].resolution
	}
	revision.Changes = normalizedChanges
	revision.Body = expected
	return nil
}

// assignDraftResolutions 优先保留正确 ID，再按评审顺序修复重复、未知或缺失 ID。
func assignDraftResolutions(
	findings []agentdomain.ReviewFinding,
	model []agentdomain.FindingResolution,
) []assignedResolution {
	assigned := make([]assignedResolution, len(findings))
	used := make([]bool, len(model))
	for findingIndex, finding := range findings {
		for modelIndex, resolution := range model {
			if !used[modelIndex] && resolution.FindingID == finding.ID {
				assigned[findingIndex] = assignedResolution{resolution: resolution, modelID: resolution.FindingID}
				assigned[findingIndex].resolution.FindingID = finding.ID
				used[modelIndex] = true
				break
			}
		}
	}
	for findingIndex, finding := range findings {
		if assigned[findingIndex].resolution.FindingID != "" {
			continue
		}
		modelIndex := nextUnusedResolution(used)
		if modelIndex < 0 {
			assigned[findingIndex] = assignedResolution{resolution: agentdomain.FindingResolution{
				FindingID: finding.ID, Status: agentdomain.FindingCannotResolve, Note: "模型未返回该问题的处理结果。",
			}}
			continue
		}
		resolution := model[modelIndex]
		assigned[findingIndex] = assignedResolution{resolution: resolution, modelID: resolution.FindingID}
		assigned[findingIndex].resolution.FindingID = finding.ID
		used[modelIndex] = true
	}
	return assigned
}

func nextUnusedResolution(used []bool) int {
	for index := range used {
		if !used[index] {
			return index
		}
	}
	return -1
}

func matchingDraftChange(changes []agentdomain.RevisionChange, used []bool, modelID, findingID string) int {
	for _, id := range []string{modelID, findingID} {
		if strings.TrimSpace(id) == "" {
			continue
		}
		for index, change := range changes {
			if !used[index] && change.FindingID == id {
				return index
			}
		}
	}
	return -1
}
