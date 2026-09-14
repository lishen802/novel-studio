package agent

import (
	"encoding/json"
	"sort"
	"strings"
)

const (
	MaxStoryDirections   = 3
	MinRoadmapNearTerm   = 3
	MaxRoadmapNearTerm   = 10
	MaxCastCharacters    = 50
	MaxCastRelationships = 100
	MaxCastFactions      = 30
	MaxCastLocations     = 50
)

type DirectionCard struct {
	ID           string `json:"id"`
	Hook         string `json:"hook"`
	Genre        string `json:"genre"`
	Protagonist  string `json:"protagonist"`
	CoreConflict string `json:"core_conflict"`
	GrowthCost   string `json:"growth_cost"`
	Pacing       string `json:"pacing"`
	Ending       string `json:"ending"`
}

type StoryDirectionsOutput struct {
	Directions []DirectionCard `json:"directions"`
}

// Validate 强制三张方向卡可比较：七字段非空、核心四元组不整组重复，且至少两组字段取值不同。
func (output StoryDirectionsOutput) Validate() error {
	if len(output.Directions) != MaxStoryDirections {
		return ErrInvalidTaskOutput
	}
	seenID := map[string]bool{}
	signatures := map[string]bool{}
	fieldValues := make([]map[string]struct{}, 7)
	for i := range fieldValues {
		fieldValues[i] = map[string]struct{}{}
	}
	for _, card := range output.Directions {
		id := strings.TrimSpace(card.ID)
		if id == "" || seenID[id] || blank(card.Hook) || blank(card.Genre) || blank(card.Protagonist) ||
			blank(card.CoreConflict) || blank(card.GrowthCost) || blank(card.Pacing) || blank(card.Ending) {
			return ErrInvalidTaskOutput
		}
		seenID[id] = true
		signature := strings.Join([]string{
			strings.TrimSpace(card.Hook), strings.TrimSpace(card.Protagonist),
			strings.TrimSpace(card.CoreConflict), strings.TrimSpace(card.Ending),
		}, "\x00")
		if signatures[signature] {
			return ErrInvalidTaskOutput
		}
		signatures[signature] = true
		values := []string{card.Hook, card.Genre, card.Protagonist, card.CoreConflict, card.GrowthCost, card.Pacing, card.Ending}
		for index, value := range values {
			fieldValues[index][strings.TrimSpace(value)] = struct{}{}
		}
	}
	varying := 0
	for _, values := range fieldValues {
		if len(values) > 1 {
			varying++
		}
	}
	if varying < 2 {
		return ErrInvalidTaskOutput
	}
	return nil
}

type BlueprintAnchor struct {
	Strength  string `json:"strength"`
	Statement string `json:"statement"`
	ScopeHint string `json:"scope_hint,omitempty"`
}

// ValidateProduct 校验作品方案产品 Schema v2 的附加字段；v1 Harness 不调用。
func (output StoryBlueprintOutput) ValidateProduct() error {
	if output.Validate() != nil || blank(output.SellPoint) || blank(output.Genre) || blank(output.GrowthCost) ||
		blank(output.Pacing) || blank(output.Ending) {
		return ErrInvalidTaskOutput
	}
	if len(output.Subplots) > MaxBlueprintItems || len(output.Forbidden) > MaxBlueprintItems ||
		len(output.Anchors) > MaxBlueprintItems || hasBlank(output.Subplots) || hasBlank(output.Forbidden) {
		return ErrInvalidTaskOutput
	}
	for _, anchor := range output.Anchors {
		if !validAnchorStrength(anchor.Strength) || blank(anchor.Statement) {
			return ErrInvalidTaskOutput
		}
	}
	return nil
}

type CastCharacter struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Aliases    []string `json:"aliases"`
	Summary    string   `json:"summary"`
	Motivation string   `json:"motivation"`
	Fear       string   `json:"fear"`
	Secret     string   `json:"secret"`
	Arc        string   `json:"arc"`
}

type CastRelationship struct {
	ID                 string `json:"id,omitempty"`
	SourceCharacterRef string `json:"source_character_ref,omitempty"`
	TargetCharacterRef string `json:"target_character_ref,omitempty"`
	SourceName         string `json:"source_name,omitempty"`
	TargetName         string `json:"target_name,omitempty"`
	Kind               string `json:"kind"`
	Label              string `json:"label"`
	Strength           int    `json:"strength"`
}

type CastFaction struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Goal    string `json:"goal"`
}

type CastLocation struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

type CastWorldRule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ScopeType   string `json:"scope_type"`
}

type StoryCastOutput struct {
	Characters      []CastCharacter    `json:"characters"`
	Relationships   []CastRelationship `json:"relationships"`
	Factions        []CastFaction      `json:"factions"`
	Locations       []CastLocation     `json:"locations"`
	WorldRules      []CastWorldRule    `json:"world_rules"`
	Forbidden       []string           `json:"forbidden"`
	OmissionReasons []string           `json:"omission_reasons"`
}

// Validate 要求至少一名主角与一条世界规则；禁写为空时必须给出省略原因。
func (output StoryCastOutput) Validate() error {
	if len(output.Characters) == 0 || len(output.Characters) > MaxCastCharacters ||
		len(output.Relationships) > MaxCastRelationships || len(output.Factions) > MaxCastFactions ||
		len(output.Locations) > MaxCastLocations || len(output.WorldRules) == 0 ||
		len(output.WorldRules) > MaxBlueprintItems || hasBlank(output.Forbidden) || hasBlank(output.OmissionReasons) ||
		(len(output.Forbidden) == 0 && len(output.OmissionReasons) == 0) {
		return ErrInvalidTaskOutput
	}
	stableReferences := false
	for _, character := range output.Characters {
		stableReferences = stableReferences || strings.TrimSpace(character.ID) != ""
	}
	for _, relation := range output.Relationships {
		stableReferences = stableReferences || strings.TrimSpace(relation.ID) != "" ||
			strings.TrimSpace(relation.SourceCharacterRef) != "" || strings.TrimSpace(relation.TargetCharacterRef) != ""
	}
	names := map[string]bool{}
	characterIDs := map[string]bool{}
	hasProtagonist := false
	for _, character := range output.Characters {
		name := strings.TrimSpace(character.Name)
		id := strings.TrimSpace(character.ID)
		if name == "" || (!stableReferences && names[name]) || (stableReferences && (id == "" || characterIDs[id])) ||
			!validCastRole(character.Role) || blank(character.Summary) ||
			blank(character.Motivation) || blank(character.Arc) {
			return ErrInvalidTaskOutput
		}
		names[name] = true
		characterIDs[id] = true
		if character.Role == "protagonist" {
			hasProtagonist = true
		}
	}
	if !hasProtagonist {
		return ErrInvalidTaskOutput
	}
	relationIDs := map[string]bool{}
	for _, relation := range output.Relationships {
		validReference := false
		if stableReferences {
			id := strings.TrimSpace(relation.ID)
			source := strings.TrimSpace(relation.SourceCharacterRef)
			target := strings.TrimSpace(relation.TargetCharacterRef)
			validReference = id != "" && !relationIDs[id] && characterIDs[source] && characterIDs[target] && source != target &&
				strings.TrimSpace(relation.SourceName) == "" && strings.TrimSpace(relation.TargetName) == ""
			relationIDs[id] = true
		} else {
			source := strings.TrimSpace(relation.SourceName)
			target := strings.TrimSpace(relation.TargetName)
			validReference = names[source] && names[target] && source != target
		}
		if !validReference || !validCastRelationKind(relation.Kind) || relation.Strength < -100 || relation.Strength > 100 {
			return ErrInvalidTaskOutput
		}
	}
	factionNames := map[string]bool{}
	for _, faction := range output.Factions {
		name := strings.TrimSpace(faction.Name)
		if name == "" || factionNames[name] || blank(faction.Summary) || blank(faction.Goal) {
			return ErrInvalidTaskOutput
		}
		factionNames[name] = true
	}
	locationNames := map[string]bool{}
	for _, location := range output.Locations {
		name := strings.TrimSpace(location.Name)
		if name == "" || locationNames[name] || !validCastLocationKind(location.Kind) || blank(location.Summary) {
			return ErrInvalidTaskOutput
		}
		locationNames[name] = true
	}
	ruleNames := map[string]bool{}
	for _, rule := range output.WorldRules {
		name := strings.TrimSpace(rule.Name)
		if name == "" || ruleNames[name] || blank(rule.Description) || !validCastRuleScope(rule.ScopeType) {
			return ErrInvalidTaskOutput
		}
		ruleNames[name] = true
	}
	return nil
}

// ValidateStableReferences 要求人物与关系完整使用 v2 稳定 ID，禁止退回姓名猜测。
func (output StoryCastOutput) ValidateStableReferences() error {
	for _, character := range output.Characters {
		if strings.TrimSpace(character.ID) == "" {
			return ErrInvalidTaskOutput
		}
	}
	for _, relation := range output.Relationships {
		if strings.TrimSpace(relation.ID) == "" || strings.TrimSpace(relation.SourceCharacterRef) == "" ||
			strings.TrimSpace(relation.TargetCharacterRef) == "" {
			return ErrInvalidTaskOutput
		}
	}
	return output.Validate()
}

type RoadmapBook struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Proposition    string   `json:"proposition"`
	Ending         string   `json:"ending"`
	ProtagonistArc string   `json:"protagonist_arc"`
	Inviolable     []string `json:"inviolable"`
}

type RoadmapVolume struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Goal     string `json:"goal"`
	Conflict string `json:"conflict"`
	Payoff   string `json:"payoff"`
	Position int    `json:"position"`
}

type RoadmapCommitment struct {
	ID               string   `json:"id"`
	Statement        string   `json:"statement"`
	FulfillmentRange string   `json:"fulfillment_range"`
	RelatedRefs      []string `json:"related_refs"`
}

type RoadmapRetainedQuestion struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Reason    string `json:"reason"`
}

type RoadmapArc struct {
	ID                        string                    `json:"id"`
	VolumeRef                 string                    `json:"volume_ref"`
	Title                     string                    `json:"title"`
	CausalChain               string                    `json:"causal_chain"`
	CharacterChange           string                    `json:"character_change"`
	ReaderExpectation         string                    `json:"reader_expectation,omitempty"`
	StagePayoff               string                    `json:"stage_payoff,omitempty"`
	IrreversibleChange        string                    `json:"irreversible_change,omitempty"`
	EndingContribution        string                    `json:"ending_contribution,omitempty"`
	Commitments               []RoadmapCommitment       `json:"commitments,omitempty"`
	CommitmentsOmission       string                    `json:"commitments_omission,omitempty"`
	RetainedQuestions         []RoadmapRetainedQuestion `json:"retained_questions,omitempty"`
	RetainedQuestionsOmission string                    `json:"retained_questions_omission,omitempty"`
	Foreshadowing             []string                  `json:"foreshadowing"`
	Position                  int                       `json:"position"`
}

// RoadmapAnchor 是带强度与语句的结构化锚点；字符串锚点属于旧载荷，强度为空。
type RoadmapAnchor struct {
	Strength  string `json:"strength"`
	Statement string `json:"statement"`
}

type RoadmapChapter struct {
	ID       string          `json:"id"`
	ArcRef   string          `json:"arc_ref"`
	Title    string          `json:"title"`
	Goal     string          `json:"goal"`
	Conflict string          `json:"conflict"`
	Outcome  string          `json:"outcome"`
	Anchors  []RoadmapAnchor `json:"anchors"`
	Position int             `json:"position"`
}

// UnmarshalJSON 兼容旧版字符串锚点：旧载荷解析为强度为空的锚点，保持可读但不可入档。
func (chapter *RoadmapChapter) UnmarshalJSON(data []byte) error {
	type plain RoadmapChapter
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err == nil {
		*chapter = RoadmapChapter(decoded)
		return nil
	}
	// 旧载荷 anchors 为字符串数组；逐一映射为强度为空的锚点。
	var legacy struct {
		ID       string   `json:"id"`
		ArcRef   string   `json:"arc_ref"`
		Title    string   `json:"title"`
		Goal     string   `json:"goal"`
		Conflict string   `json:"conflict"`
		Outcome  string   `json:"outcome"`
		Anchors  []string `json:"anchors"`
		Position int      `json:"position"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	anchors := make([]RoadmapAnchor, 0, len(legacy.Anchors))
	for _, statement := range legacy.Anchors {
		anchors = append(anchors, RoadmapAnchor{Statement: statement})
	}
	*chapter = RoadmapChapter{
		ID: legacy.ID, ArcRef: legacy.ArcRef, Title: legacy.Title, Goal: legacy.Goal,
		Conflict: legacy.Conflict, Outcome: legacy.Outcome, Anchors: anchors, Position: legacy.Position,
	}
	return nil
}

type StoryRoadmapOutput struct {
	Book             RoadmapBook      `json:"book"`
	Volumes          []RoadmapVolume  `json:"volumes"`
	Arcs             []RoadmapArc     `json:"arcs"`
	NearTermChapters []RoadmapChapter `json:"near_term_chapters"`
}

// Validate 要求全书层完整、卷/弧齐全，各层 position 从 1 连续，近期章 3–10 个；
// 卷/弧/章必须有稳定 ID，弧按卷 ID、近期章按弧 ID 引用，锚点必须带合法强度。
func (output StoryRoadmapOutput) Validate() error {
	if blank(output.Book.ID) || blank(output.Book.Title) || blank(output.Book.Proposition) || blank(output.Book.Ending) ||
		blank(output.Book.ProtagonistArc) || hasBlank(output.Book.Inviolable) ||
		len(output.Volumes) == 0 || len(output.Volumes) > MaxBlueprintItems ||
		len(output.Arcs) == 0 || len(output.Arcs) > MaxBlueprintItems ||
		len(output.NearTermChapters) < MinRoadmapNearTerm || len(output.NearTermChapters) > MaxRoadmapNearTerm {
		return ErrInvalidTaskOutput
	}
	volumeIDs := map[string]bool{}
	volumePositions := make([]int, 0, len(output.Volumes))
	for _, volume := range output.Volumes {
		id := strings.TrimSpace(volume.ID)
		if id == "" || volumeIDs[id] || blank(volume.Title) || blank(volume.Goal) || blank(volume.Conflict) || blank(volume.Payoff) {
			return ErrInvalidTaskOutput
		}
		volumeIDs[id] = true
		volumePositions = append(volumePositions, volume.Position)
	}
	if !consecutivePositions(volumePositions) {
		return ErrInvalidTaskOutput
	}
	arcIDs := map[string]bool{}
	arcsByVolume := map[string][]int{}
	for _, arc := range output.Arcs {
		id := strings.TrimSpace(arc.ID)
		if id == "" || arcIDs[id] || !volumeIDs[strings.TrimSpace(arc.VolumeRef)] || blank(arc.Title) ||
			blank(arc.CausalChain) || blank(arc.CharacterChange) || hasBlank(arc.Foreshadowing) {
			return ErrInvalidTaskOutput
		}
		arcIDs[id] = true
		arcsByVolume[strings.TrimSpace(arc.VolumeRef)] = append(arcsByVolume[strings.TrimSpace(arc.VolumeRef)], arc.Position)
	}
	for _, positions := range arcsByVolume {
		if !consecutivePositions(positions) {
			return ErrInvalidTaskOutput
		}
	}
	chapterIDs := map[string]bool{}
	chapterPositions := make([]int, 0, len(output.NearTermChapters))
	for _, chapter := range output.NearTermChapters {
		id := strings.TrimSpace(chapter.ID)
		if id == "" || chapterIDs[id] || !arcIDs[strings.TrimSpace(chapter.ArcRef)] || blank(chapter.Title) ||
			blank(chapter.Goal) || blank(chapter.Conflict) || blank(chapter.Outcome) {
			return ErrInvalidTaskOutput
		}
		chapterIDs[id] = true
		for _, anchor := range chapter.Anchors {
			if !validAnchorStrength(anchor.Strength) || blank(anchor.Statement) {
				return ErrInvalidTaskOutput
			}
		}
		chapterPositions = append(chapterPositions, chapter.Position)
	}
	if !consecutivePositions(chapterPositions) {
		return ErrInvalidTaskOutput
	}
	return nil
}

// ValidatePlanningGoals 校验故事弧规划目标。这些是待实现目标，不能把空值解释成已兑现。
// 旧 v2 载荷可解码且通过 Validate，但不得因此被当成 v3 已验证。
func (output StoryRoadmapOutput) ValidatePlanningGoals() error {
	if err := output.Validate(); err != nil {
		return err
	}
	for _, arc := range output.Arcs {
		if blank(arc.ReaderExpectation) || blank(arc.StagePayoff) || blank(arc.IrreversibleChange) || blank(arc.EndingContribution) {
			return ErrInvalidTaskOutput
		}
		if err := validatePlanningList(len(arc.Commitments), arc.CommitmentsOmission); err != nil {
			return err
		}
		seenCommitments := map[string]bool{}
		for _, item := range arc.Commitments {
			id := strings.TrimSpace(item.ID)
			if id == "" || seenCommitments[id] || blank(item.Statement) || blank(item.FulfillmentRange) || hasBlank(item.RelatedRefs) {
				return ErrInvalidTaskOutput
			}
			seenCommitments[id] = true
		}
		if err := validatePlanningList(len(arc.RetainedQuestions), arc.RetainedQuestionsOmission); err != nil {
			return err
		}
		seenQuestions := map[string]bool{}
		for _, item := range arc.RetainedQuestions {
			id := strings.TrimSpace(item.ID)
			if id == "" || seenQuestions[id] || blank(item.Statement) || blank(item.Reason) {
				return ErrInvalidTaskOutput
			}
			seenQuestions[id] = true
		}
	}
	return nil
}

func validatePlanningList(count int, omission string) error {
	if count == 0 && blank(omission) {
		return ErrInvalidTaskOutput
	}
	if count > 0 && strings.TrimSpace(omission) != "" {
		return ErrInvalidTaskOutput
	}
	return nil
}

// PlanningObject 返回可入档的故事弧目标载荷；旧 v2 没有目标时返回 nil，不伪造空目标。
func (arc RoadmapArc) PlanningObject() map[string]any {
	if blank(arc.ReaderExpectation) && blank(arc.StagePayoff) && blank(arc.IrreversibleChange) &&
		blank(arc.EndingContribution) && len(arc.Commitments) == 0 && len(arc.RetainedQuestions) == 0 {
		return nil
	}
	object := map[string]any{
		"reader_expectation":  arc.ReaderExpectation,
		"stage_payoff":        arc.StagePayoff,
		"irreversible_change": arc.IrreversibleChange,
		"ending_contribution": arc.EndingContribution,
		"commitments":         arc.Commitments,
		"retained_questions":  arc.RetainedQuestions,
	}
	if strings.TrimSpace(arc.CommitmentsOmission) != "" {
		object["commitments_omission"] = arc.CommitmentsOmission
	}
	if strings.TrimSpace(arc.RetainedQuestionsOmission) != "" {
		object["retained_questions_omission"] = arc.RetainedQuestionsOmission
	}
	return object
}

func hasBlank(values []string) bool {
	for _, value := range values {
		if blank(value) {
			return true
		}
	}
	return false
}

func consecutivePositions(positions []int) bool {
	if len(positions) == 0 {
		return false
	}
	sorted := append([]int(nil), positions...)
	sort.Ints(sorted)
	for index, value := range sorted {
		if value != index+1 {
			return false
		}
	}
	return true
}

func validAnchorStrength(value string) bool {
	switch strings.TrimSpace(value) {
	case "hard", "soft", "forbidden":
		return true
	default:
		return false
	}
}

func validCastRole(value string) bool {
	switch strings.TrimSpace(value) {
	case "protagonist", "supporting", "antagonist", "minor":
		return true
	default:
		return false
	}
}

func validCastRelationKind(value string) bool {
	switch strings.TrimSpace(value) {
	case "ally", "family", "romantic", "rival", "enemy", "mentor", "other":
		return true
	default:
		return false
	}
}

func validCastLocationKind(value string) bool {
	switch strings.TrimSpace(value) {
	case "city", "settlement", "building", "landmark", "wilderness", "other":
		return true
	default:
		return false
	}
}

func validCastRuleScope(value string) bool {
	switch strings.TrimSpace(value) {
	case "project", "region", "location":
		return true
	default:
		return false
	}
}
