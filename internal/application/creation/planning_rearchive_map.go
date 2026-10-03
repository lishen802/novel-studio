package creation

import (
	"encoding/json"
	"fmt"
	"strings"

	agent "github.com/lishen802/novel-studio/internal/domain/agent"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type rearchiveNode struct {
	ID       string
	Level    string
	Title    string
	Summary  string
	Position int
	Fields   map[string]any
}

func (node rearchiveNode) signature() string {
	return hashPlanningPreview([]any{node.Level, node.Title, node.Summary, node.Position})
}

// roadmapRearchiveNodes 从稳定来源 ID 生成正式节点文字，不重排或重建结构。
func roadmapRearchiveNodes(road agent.StoryRoadmapOutput) []rearchiveNode {
	out := []rearchiveNode{{ID: road.Book.ID, Level: "book", Title: road.Book.Title, Summary: road.Book.Proposition, Position: 1, Fields: map[string]any{
		"title": road.Book.Title, "summary": road.Book.Proposition, "goal": road.Book.ProtagonistArc, "conflict": strings.Join(road.Book.Inviolable, "；"), "outcome": road.Book.Ending,
	}}}
	for _, v := range road.Volumes {
		out = append(out, rearchiveNode{ID: v.ID, Level: "volume", Title: v.Title, Summary: v.Goal, Position: v.Position, Fields: map[string]any{"title": v.Title, "summary": v.Goal, "goal": v.Goal, "conflict": v.Conflict, "outcome": v.Payoff}})
	}
	for _, a := range road.Arcs {
		fields := map[string]any{"title": a.Title, "summary": a.CausalChain, "goal": a.CausalChain, "conflict": a.CharacterChange, "outcome": strings.Join(a.Foreshadowing, "；")}
		if planning := a.PlanningObject(); planning != nil {
			fields["planning"] = planning
		}
		out = append(out, rearchiveNode{ID: a.ID, Level: "arc", Title: a.Title, Summary: a.CausalChain, Position: a.Position, Fields: fields})
	}
	for _, c := range road.NearTermChapters {
		out = append(out, rearchiveNode{ID: c.ID, Level: "chapter", Title: c.Title, Summary: c.Goal, Position: c.Position, Fields: map[string]any{"title": c.Title, "summary": c.Goal, "goal": c.Goal, "conflict": c.Conflict, "outcome": c.Outcome}})
	}
	return out
}

// mapRearchiveRoadmap 用原始完整字段唯一匹配正式 revision 1，同名仍靠层级与内容校验。
func mapRearchiveRoadmap(oldRoad, newRoad agent.StoryRoadmapOutput, assets []asset.PlanningArchiveAsset) (map[string]map[string]any, error) {
	oldNodes, newNodes := roadmapRearchiveNodes(oldRoad), roadmapRearchiveNodes(newRoad)
	if len(oldNodes) != len(newNodes) || len(oldNodes) != len(assets) {
		return nil, errRearchiveIdentity
	}
	byID := map[string]rearchiveNode{}
	for _, n := range newNodes {
		if n.ID == "" || byID[n.ID].ID != "" {
			return nil, errRearchiveIdentity
		}
		byID[n.ID] = n
	}
	bySignature := map[string]string{}
	for _, n := range oldNodes {
		if bySignature[n.signature()] != "" {
			return nil, errRearchiveIdentity
		}
		bySignature[n.signature()] = n.ID
	}
	matched := map[string]bool{}
	result := map[string]map[string]any{}
	for _, row := range assets {
		var original struct {
			Level, Title, Summary string
			Position              int
		}
		if row.Kind != "outline_node" || json.Unmarshal([]byte(row.Original), &original) != nil {
			return nil, errRearchiveIdentity
		}
		id := bySignature[(rearchiveNode{Level: original.Level, Title: original.Title, Summary: original.Summary, Position: original.Position}).signature()]
		if id == "" || matched[id] {
			return nil, errRearchiveIdentity
		}
		matched[id] = true
		next, ok := byID[id]
		if !ok || next.Level != original.Level || next.Position != original.Position {
			return nil, errRearchiveIdentity
		}
		var current map[string]json.RawMessage
		if json.Unmarshal([]byte(row.Current), &current) != nil {
			return nil, asset.ErrChangeUnsupported
		}
		if planning, has := current["planning"]; has {
			var existing map[string]json.RawMessage
			if json.Unmarshal(planning, &existing) != nil {
				return nil, asset.ErrChangeUnsupported
			}
			planned, ok := next.Fields["planning"].(map[string]any)
			if !ok {
				return nil, asset.ErrChangeUnsupported
			}
			for _, key := range []string{"reference_bindings", "schema_version"} {
				if raw, has := existing[key]; has {
					planned[key] = raw
				}
			}
		} else if _, has := next.Fields["planning"]; has {
			return nil, asset.ErrChangeUnsupported
		}
		result[row.AssetID] = next.Fields
	}
	if len(matched) != len(oldNodes) {
		return nil, fmt.Errorf("%w: 节点集合不完整", errRearchiveIdentity)
	}
	return result, nil
}
