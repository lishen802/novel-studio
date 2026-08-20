package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrInvalidTaskOutput = errors.New("invalid agent task output")

const (
	MaxBlueprintItems = 100
	MaxChapterScenes  = 50
	MaxReviewFindings = 200
	MaxResolutions    = 200
)

type TaskOutput interface{ Validate() error }

type StoryBlueprintOutput struct {
	Title           string   `json:"title"`
	Positioning     string   `json:"positioning"`
	CoreConflict    string   `json:"core_conflict"`
	ProtagonistArc  string   `json:"protagonist_arc"`
	WorldRules      []string `json:"world_rules"`
	MajorMilestones []string `json:"major_milestones"`
}

func (output StoryBlueprintOutput) Validate() error {
	if blank(output.Title) || blank(output.Positioning) || blank(output.CoreConflict) || blank(output.ProtagonistArc) ||
		len(output.WorldRules) == 0 || len(output.WorldRules) > MaxBlueprintItems || len(output.MajorMilestones) < 3 ||
		len(output.MajorMilestones) > MaxBlueprintItems || hasBlankOrDuplicate(output.WorldRules) || hasBlankOrDuplicate(output.MajorMilestones) {
		return ErrInvalidTaskOutput
	}
	return nil
}

type SceneBeat struct {
	ID       string `json:"id"`
	Goal     string `json:"goal"`
	Conflict string `json:"conflict"`
	Outcome  string `json:"outcome"`
}

type ChapterPlanOutput struct {
	ChapterGoal string      `json:"chapter_goal"`
	OpeningHook string      `json:"opening_hook"`
	EndingHook  string      `json:"ending_hook"`
	Scenes      []SceneBeat `json:"scenes"`
}

func (output ChapterPlanOutput) Validate() error {
	if blank(output.ChapterGoal) || blank(output.OpeningHook) || blank(output.EndingHook) || len(output.Scenes) == 0 || len(output.Scenes) > MaxChapterScenes {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	for _, scene := range output.Scenes {
		if blank(scene.ID) || blank(scene.Goal) || blank(scene.Conflict) || blank(scene.Outcome) || seen[scene.ID] {
			return ErrInvalidTaskOutput
		}
		seen[scene.ID] = true
	}
	return nil
}

type ChapterDraftOutput struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	CompletionNote string `json:"completion_note"`
}

func (output ChapterDraftOutput) Validate() error {
	if blank(output.Title) || blank(output.Body) || blank(output.CompletionNote) {
		return ErrInvalidTaskOutput
	}
	return nil
}

type ReviewBundleOutput struct {
	QualityStatus QualityStatus   `json:"quality_status"`
	Summary       string          `json:"summary"`
	Findings      []ReviewFinding `json:"findings"`
}

func (output ReviewBundleOutput) Validate() error {
	if !ValidQualityStatus(output.QualityStatus) || blank(output.Summary) || len(output.Findings) > MaxReviewFindings {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	for _, finding := range output.Findings {
		if err := finding.Validate(); err != nil {
			return err
		}
		if seen[finding.ID] {
			return ErrInvalidTaskOutput
		}
		seen[finding.ID] = true
	}
	if output.QualityStatus == QualityPassed && len(output.Findings) > 0 {
		return ErrInvalidTaskOutput
	}
	if output.QualityStatus != QualityPassed && len(output.Findings) == 0 {
		return ErrInvalidTaskOutput
	}
	return nil
}

type FindingResolution struct {
	FindingID string        `json:"finding_id"`
	Status    FindingStatus `json:"status"`
	Note      string        `json:"note"`
}

type ReviseDraftOutput struct {
	Title       string              `json:"title"`
	Body        string              `json:"body"`
	Resolutions []FindingResolution `json:"resolutions"`
}

func (output ReviseDraftOutput) Validate() error {
	if blank(output.Title) || blank(output.Body) || len(output.Resolutions) == 0 || len(output.Resolutions) > MaxResolutions {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	for _, resolution := range output.Resolutions {
		if blank(resolution.FindingID) || blank(resolution.Note) || !validFindingStatus(resolution.Status) ||
			resolution.Status == FindingOpen || seen[resolution.FindingID] {
			return ErrInvalidTaskOutput
		}
		seen[resolution.FindingID] = true
	}
	return nil
}

// DecodeTaskOutput 严格解码模型 JSON，拒绝未知字段、尾随内容和领域约束不合法的结果。
func DecodeTaskOutput(kind TaskKind, data []byte) (TaskOutput, error) {
	var output TaskOutput
	switch kind {
	case TaskStoryBlueprint:
		output = &StoryBlueprintOutput{}
	case TaskChapterPlan:
		output = &ChapterPlanOutput{}
	case TaskChapterDraft, TaskBaselineDraft:
		output = &ChapterDraftOutput{}
	case TaskReviewBundle:
		output = &ReviewBundleOutput{}
	case TaskReviseDraft:
		output = &ReviseDraftOutput{}
	default:
		return nil, ErrInvalidTaskOutput
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTaskOutput, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := output.Validate(); err != nil {
		return nil, err
	}
	return output, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ErrInvalidTaskOutput
	}
	return nil
}

func hasBlankOrDuplicate(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}
