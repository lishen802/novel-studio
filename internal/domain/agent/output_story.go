package agent

import (
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
	SourceName string `json:"source_name"`
	TargetName string `json:"target_name"`
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	Strength   int    `json:"strength"`
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
	names := map[string]bool{}
	hasProtagonist := false
	for _, character := range output.Characters {
		name := strings.TrimSpace(character.Name)
		if name == "" || names[name] || !validCastRole(character.Role) || blank(character.Summary) ||
			blank(character.Motivation) || blank(character.Arc) {
			return ErrInvalidTaskOutput
		}
		names[name] = true
		if character.Role == "protagonist" {
			hasProtagonist = true
		}
	}
	if !hasProtagonist {
		return ErrInvalidTaskOutput
	}
	for _, relation := range output.Relationships {
		source := strings.TrimSpace(relation.SourceName)
		target := strings.TrimSpace(relation.TargetName)
		if !names[source] || !names[target] || source == target || !validCastRelationKind(relation.Kind) ||
			relation.Strength < -100 || relation.Strength > 100 {
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

type RoadmapBook struct {
	Title          string   `json:"title"`
	Proposition    string   `json:"proposition"`
	Ending         string   `json:"ending"`
	ProtagonistArc string   `json:"protagonist_arc"`
	Inviolable     []string `json:"inviolable"`
}

type RoadmapVolume struct {
	Title    string `json:"title"`
	Goal     string `json:"goal"`
	Conflict string `json:"conflict"`
	Payoff   string `json:"payoff"`
	Position int    `json:"position"`
}

type RoadmapArc struct {
	VolumeRef       string   `json:"volume_ref"`
	Title           string   `json:"title"`
	CausalChain     string   `json:"causal_chain"`
	CharacterChange string   `json:"character_change"`
	Foreshadowing   []string `json:"foreshadowing"`
	Position        int      `json:"position"`
}

type RoadmapChapter struct {
	Title    string   `json:"title"`
	Goal     string   `json:"goal"`
	Conflict string   `json:"conflict"`
	Outcome  string   `json:"outcome"`
	Anchors  []string `json:"anchors"`
	Position int      `json:"position"`
}

type StoryRoadmapOutput struct {
	Book             RoadmapBook      `json:"book"`
	Volumes          []RoadmapVolume  `json:"volumes"`
	Arcs             []RoadmapArc     `json:"arcs"`
	NearTermChapters []RoadmapChapter `json:"near_term_chapters"`
}

// Validate 要求全书层完整、卷/弧齐全，且各层 position 从 1 连续，近期章 3–10 个。
func (output StoryRoadmapOutput) Validate() error {
	if blank(output.Book.Title) || blank(output.Book.Proposition) || blank(output.Book.Ending) ||
		blank(output.Book.ProtagonistArc) || hasBlank(output.Book.Inviolable) ||
		len(output.Volumes) == 0 || len(output.Volumes) > MaxBlueprintItems ||
		len(output.Arcs) == 0 || len(output.Arcs) > MaxBlueprintItems ||
		len(output.NearTermChapters) < MinRoadmapNearTerm || len(output.NearTermChapters) > MaxRoadmapNearTerm {
		return ErrInvalidTaskOutput
	}
	volumeTitles := map[string]bool{}
	volumePositions := make([]int, 0, len(output.Volumes))
	for _, volume := range output.Volumes {
		title := strings.TrimSpace(volume.Title)
		if title == "" || volumeTitles[title] || blank(volume.Goal) || blank(volume.Conflict) || blank(volume.Payoff) {
			return ErrInvalidTaskOutput
		}
		volumeTitles[title] = true
		volumePositions = append(volumePositions, volume.Position)
	}
	if !consecutivePositions(volumePositions) {
		return ErrInvalidTaskOutput
	}
	arcsByVolume := map[string][]int{}
	for _, arc := range output.Arcs {
		if !volumeTitles[strings.TrimSpace(arc.VolumeRef)] || blank(arc.Title) || blank(arc.CausalChain) ||
			blank(arc.CharacterChange) || hasBlank(arc.Foreshadowing) {
			return ErrInvalidTaskOutput
		}
		arcsByVolume[strings.TrimSpace(arc.VolumeRef)] = append(arcsByVolume[strings.TrimSpace(arc.VolumeRef)], arc.Position)
	}
	for _, positions := range arcsByVolume {
		if !consecutivePositions(positions) {
			return ErrInvalidTaskOutput
		}
	}
	chapterPositions := make([]int, 0, len(output.NearTermChapters))
	for _, chapter := range output.NearTermChapters {
		if blank(chapter.Title) || blank(chapter.Goal) || blank(chapter.Conflict) || blank(chapter.Outcome) ||
			hasBlank(chapter.Anchors) {
			return ErrInvalidTaskOutput
		}
		chapterPositions = append(chapterPositions, chapter.Position)
	}
	if !consecutivePositions(chapterPositions) {
		return ErrInvalidTaskOutput
	}
	return nil
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
