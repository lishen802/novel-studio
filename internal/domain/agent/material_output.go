package agent

import "strings"

// MaterialAnalysisOutput 只提取作者已给出的完整阶段内容；缺失内容通过缺口返回，不替作者编造。
type MaterialAnalysisOutput struct {
	EntryID     string                `json:"entry_id"`
	SourceHash  string                `json:"source_hash"`
	Proposal    *StoryBlueprintOutput `json:"proposal"`
	PeopleWorld *StoryCastOutput      `json:"people_world"`
	Roadmap     *StoryRoadmapOutput   `json:"roadmap"`
	Gaps        []MaterialGap         `json:"gaps"`
	Sources     []MaterialFieldSource `json:"field_sources"`
}

type MaterialGap struct {
	ID              string `json:"id"`
	Stage           string `json:"stage"`
	FieldPath       string `json:"field_path"`
	Kind            string `json:"kind"`
	Reason          string `json:"reason"`
	Quote           string `json:"quote"`
	SuggestedAction string `json:"suggested_action"`
}

type MaterialFieldSource struct {
	Stage     string `json:"stage"`
	FieldPath string `json:"field_path"`
	Quote     string `json:"quote"`
}

// Validate 校验提取结果和缺口结构；引文归属及精确范围由应用层核实。
func (output MaterialAnalysisOutput) Validate() error {
	if blank(output.EntryID) || len(output.SourceHash) != 64 || len(output.Gaps) > 500 || len(output.Sources) > 1000 {
		return ErrInvalidTaskOutput
	}
	if output.Proposal != nil && output.Proposal.ValidateProduct() != nil {
		return ErrInvalidTaskOutput
	}
	if output.PeopleWorld != nil && output.PeopleWorld.Validate() != nil {
		return ErrInvalidTaskOutput
	}
	if output.Roadmap != nil && output.Roadmap.Validate() != nil {
		return ErrInvalidTaskOutput
	}
	seen := map[string]bool{}
	for _, gap := range output.Gaps {
		if blank(gap.ID) || seen[gap.ID] || !materialStage(gap.Stage) || !strings.HasPrefix(gap.FieldPath, "/") || blank(gap.Reason) || blank(gap.SuggestedAction) ||
			(gap.Kind != "required" && gap.Kind != "optional" && gap.Kind != "conflict") || (gap.Kind == "conflict" && blank(gap.Quote)) {
			return ErrInvalidTaskOutput
		}
		seen[gap.ID] = true
	}
	for _, source := range output.Sources {
		if !materialStage(source.Stage) || !strings.HasPrefix(source.FieldPath, "/") || blank(source.Quote) {
			return ErrInvalidTaskOutput
		}
	}
	for stage, missing := range map[string]bool{"proposal": output.Proposal == nil, "people_world": output.PeopleWorld == nil, "roadmap": output.Roadmap == nil} {
		if !missing {
			continue
		}
		found := false
		for _, gap := range output.Gaps {
			if gap.Stage == stage && gap.Kind != "optional" {
				found = true
			}
		}
		if !found {
			return ErrInvalidTaskOutput
		}
	}
	return nil
}

func materialStage(stage string) bool {
	return stage == "proposal" || stage == "people_world" || stage == "roadmap"
}
