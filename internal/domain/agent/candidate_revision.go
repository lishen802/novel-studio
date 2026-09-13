package agent

import "encoding/json"

// StoryCandidateRevisionOutput 保存完整候选载荷，显式声明类型与具体 Schema 版本。
// Payload 不重编码为旧类型，避免修订过程中丢失扩展业务字段。
type StoryCandidateRevisionOutput struct {
	CandidateKind    string          `json:"candidate_kind"`
	PayloadSchemaRef string          `json:"payload_schema_ref"`
	Payload          json.RawMessage `json:"payload"`
}

// Validate 分发到已支持的具体载荷合同；未知版本必须显式升级后才能使用。
func (output StoryCandidateRevisionOutput) Validate() error {
	var kind TaskKind
	switch output.CandidateKind {
	case "direction_set":
		if output.PayloadSchemaRef != "story_directions.schema@1" {
			return ErrInvalidTaskOutput
		}
		kind = TaskStoryDirections
	case "proposal":
		if output.PayloadSchemaRef != "story_blueprint.schema@2" {
			return ErrInvalidTaskOutput
		}
		kind = TaskStoryBlueprint
	case "people_world":
		if output.PayloadSchemaRef != "story_cast.schema@2" {
			return ErrInvalidTaskOutput
		}
		kind = TaskStoryCast
	case "roadmap":
		if output.PayloadSchemaRef != "story_roadmap.schema@2" {
			return ErrInvalidTaskOutput
		}
		kind = TaskStoryRoadmap
	default:
		return ErrInvalidTaskOutput
	}
	if !json.Valid(output.Payload) {
		return ErrInvalidTaskOutput
	}
	decoded, err := DecodeTaskOutput(kind, output.Payload)
	if err != nil {
		return err
	}
	if blueprint, ok := decoded.(*StoryBlueprintOutput); ok {
		return blueprint.ValidateProduct()
	}
	if cast, ok := decoded.(*StoryCastOutput); ok {
		return cast.ValidateStableReferences()
	}
	return decoded.Validate()
}
