package workspace

import (
	"testing"

	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// TestFormalOutlinePlanningUsesChapterEffectivePositions 验证上下文采用未来版本顺序。
func TestFormalOutlinePlanningUsesChapterEffectivePositions(t *testing.T) {
	nodes := []story.OutlineNode{
		{ID: "first", ParentID: "arc", Position: 1},
		{ID: "second", ParentID: "arc", Position: 2},
		{ID: "other", ParentID: "another-arc", Position: 1},
	}
	versions := []asset.EffectiveConstraint{
		{Kind: "outline_node", TypedID: "first", Position: 2},
		{Kind: "outline_node", TypedID: "second", Position: 1},
		{Kind: "outline_node", TypedID: "other", Position: 1},
	}
	result := formalOutlinePlanning(nodes, versions)
	if len(result) != 3 || result[0].ID != "second" || result[1].ID != "first" || result[2].ID != "other" {
		t.Fatalf("effective order: %#v", result)
	}
	if nodes[0].Position != 1 || nodes[1].Position != 2 {
		t.Fatalf("original nodes changed: %#v", nodes)
	}
}
