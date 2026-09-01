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
	MaxBlueprintItems  = 100
	MaxChapterScenes   = 50
	MaxReviewFindings  = 200
	MaxResolutions     = 200
	MaxRevisionChanges = 200
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
	ID                 string   `json:"id"`
	Goal               string   `json:"goal"`
	Conflict           string   `json:"conflict"`
	Outcome            string   `json:"outcome"`
	POV                string   `json:"pov,omitempty"`
	Desire             string   `json:"desire,omitempty"`
	Obstacle           string   `json:"obstacle,omitempty"`
	InformationRelease []string `json:"information_release,omitempty"`
	StateChange        string   `json:"state_change,omitempty"`
}

type ChapterPlanOutput struct {
	ChapterGoal         string      `json:"chapter_goal"`
	OpeningHook         string      `json:"opening_hook"`
	EndingHook          string      `json:"ending_hook"`
	Scenes              []SceneBeat `json:"scenes"`
	RequiredAnchors     []string    `json:"required_anchors,omitempty"`
	ForbiddenReveals    []string    `json:"forbidden_reveals,omitempty"`
	ImprovisationBounds []string    `json:"improvisation_bounds,omitempty"`
	CanonicalSourceRefs []string    `json:"canonical_source_refs,omitempty"`
	CreativeSuggestions []string    `json:"creative_suggestions,omitempty"`
}

type ConstraintNotesOutput struct {
	Notes []ConstraintNote `json:"notes"`
}

func (output ConstraintNotesOutput) Validate() error {
	if len(output.Notes) == 0 || len(output.Notes) > MaxReviewFindings {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	for _, note := range output.Notes {
		if note.Validate() != nil || seen[note.ID] {
			return ErrInvalidTaskOutput
		}
		seen[note.ID] = true
	}
	return nil
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
	enhanced := len(output.RequiredAnchors)+len(output.ForbiddenReveals)+len(output.ImprovisationBounds)+len(output.CanonicalSourceRefs)+len(output.CreativeSuggestions) > 0
	if enhanced {
		if len(output.ImprovisationBounds) == 0 || len(output.CanonicalSourceRefs) == 0 ||
			hasBlankOrDuplicate(output.RequiredAnchors) || hasBlankOrDuplicate(output.ForbiddenReveals) ||
			hasBlankOrDuplicate(output.ImprovisationBounds) || hasBlankOrDuplicate(output.CanonicalSourceRefs) || hasBlankOrDuplicate(output.CreativeSuggestions) {
			return ErrInvalidTaskOutput
		}
		for _, scene := range output.Scenes {
			if blank(scene.POV) || blank(scene.Desire) || blank(scene.Obstacle) || blank(scene.StateChange) || hasBlankOrDuplicate(scene.InformationRelease) {
				return ErrInvalidTaskOutput
			}
		}
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
	Changes     []RevisionChange    `json:"changes"`
}

func (output ReviseDraftOutput) Validate() error {
	if blank(output.Title) || blank(output.Body) || validateRevisionTrace(output.Resolutions, output.Changes) != nil {
		return ErrInvalidTaskOutput
	}
	return nil
}

// ResolutionVerdict 是独立验证者对某条问题在修订稿中实际状态的裁定。
// 判定“已解决”必须附带修订稿中的证据，判定仍未解决不需要举证；
// 这一不对称让默认方向从“通过”变为“证明后才通过”。
type ResolutionVerdict struct {
	FindingID      string        `json:"finding_id"`
	VerifiedStatus FindingStatus `json:"verified_status"`
	Rationale      string        `json:"rationale"`
	Quote          string        `json:"quote,omitempty"`
	ScopeStart     *int          `json:"scope_start,omitempty"`
	ScopeEnd       *int          `json:"scope_end,omitempty"`
}

func (verdict ResolutionVerdict) Validate() error {
	if blank(verdict.FindingID) || blank(verdict.Rationale) || !validFindingStatus(verdict.VerifiedStatus) {
		return ErrInvalidTaskOutput
	}
	if (verdict.ScopeStart == nil) != (verdict.ScopeEnd == nil) {
		return ErrInvalidTaskOutput
	}
	if verdict.ScopeStart != nil {
		if *verdict.ScopeStart < 0 || *verdict.ScopeEnd <= *verdict.ScopeStart || blank(verdict.Quote) {
			return ErrInvalidTaskOutput
		}
	}
	if verdict.VerifiedStatus == FindingResolved && verdict.ScopeStart == nil {
		return ErrInvalidTaskOutput
	}
	return nil
}

// RevisionVerdictOutput 是修订复核节点的输出：逐条裁定原问题，并登记修订引入的新问题。
type RevisionVerdictOutput struct {
	Summary     string              `json:"summary"`
	Verdicts    []ResolutionVerdict `json:"verdicts"`
	Regressions []ReviewFinding     `json:"regressions"`
}

func (output RevisionVerdictOutput) Validate() error {
	if blank(output.Summary) || len(output.Verdicts) == 0 || len(output.Verdicts) > MaxResolutions ||
		len(output.Regressions) > MaxReviewFindings {
		return ErrInvalidTaskOutput
	}
	seen := make(map[string]bool, len(output.Verdicts)+len(output.Regressions))
	for _, verdict := range output.Verdicts {
		if err := verdict.Validate(); err != nil {
			return err
		}
		if seen[verdict.FindingID] {
			return ErrInvalidTaskOutput
		}
		seen[verdict.FindingID] = true
	}
	for _, regression := range output.Regressions {
		if err := regression.Validate(); err != nil {
			return err
		}
		if seen[regression.ID] {
			return ErrInvalidTaskOutput
		}
		seen[regression.ID] = true
	}
	return nil
}

type RevisionChange struct {
	FindingID    string `json:"finding_id"`
	TargetRef    string `json:"target_ref,omitempty"`
	OriginalText string `json:"original_text"`
	RevisedText  string `json:"revised_text"`
}

type ReviseBlueprintOutput struct {
	Blueprint   StoryBlueprintOutput `json:"blueprint"`
	Resolutions []FindingResolution  `json:"resolutions"`
	Changes     []RevisionChange     `json:"changes"`
}

func (output ReviseBlueprintOutput) Validate() error {
	if output.Blueprint.Validate() != nil || validateRevisionTrace(output.Resolutions, output.Changes) != nil {
		return ErrInvalidTaskOutput
	}
	return nil
}

func validateRevisionTrace(resolutions []FindingResolution, changes []RevisionChange) error {
	if len(resolutions) == 0 {
		if len(changes) == 0 || len(changes) > MaxRevisionChanges {
			return ErrInvalidTaskOutput
		}
		seenTargets := map[string]bool{}
		for _, change := range changes {
			if !blank(change.FindingID) || blank(change.TargetRef) || blank(change.OriginalText) || blank(change.RevisedText) ||
				change.OriginalText == change.RevisedText || seenTargets[change.TargetRef] {
				return ErrInvalidTaskOutput
			}
			seenTargets[change.TargetRef] = true
		}
		return nil
	}
	if len(resolutions) > MaxResolutions || len(changes) > MaxRevisionChanges {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	resolved := map[string]bool{}
	for _, resolution := range resolutions {
		if blank(resolution.FindingID) || blank(resolution.Note) || !validFindingStatus(resolution.Status) ||
			resolution.Status == FindingOpen || seen[resolution.FindingID] {
			return ErrInvalidTaskOutput
		}
		seen[resolution.FindingID] = true
		resolved[resolution.FindingID] = resolution.Status == FindingResolved
	}
	changed := map[string]bool{}
	for _, change := range changes {
		if blank(change.FindingID) || !blank(change.TargetRef) || blank(change.OriginalText) || blank(change.RevisedText) ||
			change.OriginalText == change.RevisedText || !resolved[change.FindingID] || changed[change.FindingID] {
			return ErrInvalidTaskOutput
		}
		changed[change.FindingID] = true
	}
	for findingID, isResolved := range resolved {
		if isResolved && !changed[findingID] {
			return ErrInvalidTaskOutput
		}
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
	case TaskReviseBlueprint:
		output = &ReviseBlueprintOutput{}
	case TaskVerifyRevision:
		output = &RevisionVerdictOutput{}
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
