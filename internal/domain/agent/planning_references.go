package agent

import "strings"

// planningNodeIDs 收集候选内可引用的节点，拒绝跨层重名 ID。
func (output StoryRoadmapOutput) planningNodeIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	add := func(id string) bool {
		id = strings.TrimSpace(id)
		if id == "" || ids[id] || externalPlanningRef(id) {
			return false
		}
		ids[id] = true
		return true
	}
	if !add(output.Book.ID) {
		return nil, ErrInvalidTaskOutput
	}
	for _, n := range output.Volumes {
		if !add(n.ID) {
			return nil, ErrInvalidTaskOutput
		}
	}
	for _, n := range output.Arcs {
		if !add(n.ID) {
			return nil, ErrInvalidTaskOutput
		}
	}
	for _, n := range output.NearTermChapters {
		if !add(n.ID) {
			return nil, ErrInvalidTaskOutput
		}
	}
	return ids, nil
}

// ValidatePlanningReferences 校验外部人物/关系引用；没有稳定 ID 的资产不可按名称猜测。
func (output StoryRoadmapOutput) ValidatePlanningReferences(cast StoryCastOutput) error {
	if err := output.ValidatePlanningGoals(); err != nil {
		return err
	}
	nodes, err := output.planningNodeIDs()
	if err != nil {
		return err
	}
	refs := map[string]bool{}
	for _, n := range cast.Characters {
		refs["character:"+strings.TrimSpace(n.ID)] = n.ID != ""
	}
	for _, n := range cast.Relationships {
		refs["relationship:"+strings.TrimSpace(n.ID)] = n.ID != ""
	}
	for _, arc := range output.Arcs {
		for _, c := range arc.Commitments {
			for _, ref := range c.RelatedRefs {
				if !nodes[ref] && !refs[ref] {
					return ErrInvalidTaskOutput
				}
			}
		}
	}
	return nil
}

func externalPlanningRef(ref string) bool {
	kind, id, ok := strings.Cut(ref, ":")
	return ok && strings.TrimSpace(id) != "" && (kind == "character" || kind == "relationship")
}
