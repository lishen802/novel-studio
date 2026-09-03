package agent

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestStoryDirectionsRejectsIdenticalCoreAndLowVariance(t *testing.T) {
	valid := StoryDirectionsOutput{Directions: []DirectionCard{
		{ID: "d1", Hook: "守关人放走仇敌", Genre: "奇幻悬疑", Protagonist: "阿绫", CoreConflict: "守城与救人", GrowthCost: "军职", Pacing: "中速", Ending: "开城"},
		{ID: "d2", Hook: "潮汐神谕错位", Genre: "历史传奇", Protagonist: "祭司", CoreConflict: "神谕与民生", GrowthCost: "信仰", Pacing: "慢热", Ending: "改写神谕"},
		{ID: "d3", Hook: "假身份潜入盐帮", Genre: "江湖复仇", Protagonist: "少主", CoreConflict: "复仇与认亲", GrowthCost: "姓氏", Pacing: "快节奏", Ending: "揭穿盐帮"},
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid directions: %v", err)
	}
	clone := valid
	clone.Directions[1].Hook = clone.Directions[0].Hook
	clone.Directions[1].Protagonist = clone.Directions[0].Protagonist
	clone.Directions[1].CoreConflict = clone.Directions[0].CoreConflict
	clone.Directions[1].Ending = clone.Directions[0].Ending
	if !errors.Is(clone.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected identical core quartet to fail")
	}
	sameExceptEnding := valid
	sameExceptEnding.Directions[1] = DirectionCard{
		ID: "d2", Hook: valid.Directions[0].Hook, Genre: valid.Directions[0].Genre,
		Protagonist: valid.Directions[0].Protagonist, CoreConflict: valid.Directions[0].CoreConflict,
		GrowthCost: valid.Directions[0].GrowthCost, Pacing: valid.Directions[0].Pacing, Ending: "另一结局",
	}
	sameExceptEnding.Directions[2] = DirectionCard{
		ID: "d3", Hook: valid.Directions[0].Hook, Genre: valid.Directions[0].Genre,
		Protagonist: valid.Directions[0].Protagonist, CoreConflict: valid.Directions[0].CoreConflict,
		GrowthCost: valid.Directions[0].GrowthCost, Pacing: valid.Directions[0].Pacing, Ending: "第三结局",
	}
	if !errors.Is(sameExceptEnding.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected a single varying field to fail")
	}
}

func TestStoryBlueprintProductIsSupersetOfV1(t *testing.T) {
	v1 := []byte(`{"title":"雾港","positioning":"悬疑成长","core_conflict":"真相与亲情冲突","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`)
	decoded, err := DecodeTaskOutput(TaskStoryBlueprint, v1)
	if err != nil {
		t.Fatalf("v1 decode: %v", err)
	}
	blueprint := decoded.(*StoryBlueprintOutput)
	if err := blueprint.Validate(); err != nil {
		t.Fatalf("v1 validate: %v", err)
	}
	if err := blueprint.ValidateProduct(); !errors.Is(err, ErrInvalidTaskOutput) {
		t.Fatal("v1 payload must fail product schema")
	}
	product := *blueprint
	product.SellPoint = "守关人必须放走仇敌"
	product.Genre = "奇幻悬疑"
	product.GrowthCost = "失去军职"
	product.Pacing = "中速"
	product.Ending = "开城救人"
	product.Subplots = []string{"旧令牌"}
	product.Forbidden = []string{"现代武器"}
	product.Anchors = []BlueprintAnchor{{Strength: "hard", Statement: "钟楼必须三响"}}
	if err := product.ValidateProduct(); err != nil {
		t.Fatalf("product validate: %v", err)
	}
	encoded, err := json.Marshal(product)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeTaskOutput(TaskStoryBlueprint, encoded)
	if err != nil {
		t.Fatalf("product decode: %v", err)
	}
	if again.(*StoryBlueprintOutput).Validate() != nil {
		t.Fatal("product payload must still satisfy v1 required fields")
	}
}

func TestStoryCastRequiresProtagonistRuleAndOmission(t *testing.T) {
	valid := StoryCastOutput{
		Characters: []CastCharacter{{
			Name: "阿绫", Role: "protagonist", Summary: "守关人", Motivation: "赎罪", Arc: "学会取舍",
		}},
		WorldRules:      []CastWorldRule{{Name: "潮汐律", Description: "潮位决定航路", ScopeType: "project"}},
		OmissionReasons: []string{"本方向不需要势力"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid cast: %v", err)
	}
	valid.Characters[0].Role = "narrator"
	if !errors.Is(valid.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected unknown role to fail")
	}
	valid.Characters[0].Role = "supporting"
	if !errors.Is(valid.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected missing protagonist to fail")
	}
	empty := StoryCastOutput{
		Characters: []CastCharacter{{Name: "阿绫", Role: "protagonist", Summary: "守关人", Motivation: "赎罪", Arc: "学会取舍"}},
		WorldRules: []CastWorldRule{{Name: "潮汐律", Description: "潮位决定航路", ScopeType: "project"}},
	}
	if !errors.Is(empty.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected empty forbidden without omission reason to fail")
	}
}

func TestStoryRoadmapRequiresConsecutivePositionsAndNearTermBounds(t *testing.T) {
	valid := StoryRoadmapOutput{
		Book:    RoadmapBook{Title: "潮汐关", Proposition: "守关抉择", Ending: "开城", ProtagonistArc: "承担"},
		Volumes: []RoadmapVolume{{Title: "第一卷", Goal: "建立危机", Conflict: "敌船压境", Payoff: "作出选择", Position: 1}},
		Arcs:    []RoadmapArc{{VolumeRef: "第一卷", Title: "开关", CausalChain: "敌船靠岸", CharacterChange: "接受代价", Position: 1}},
		NearTermChapters: []RoadmapChapter{
			{Title: "空船", Goal: "发现异常", Conflict: "无人登岸", Outcome: "独自登船", Position: 1},
			{Title: "旧识", Goal: "认出仇敌", Conflict: "军令与私情", Outcome: "暂时藏匿", Position: 2},
			{Title: "开关", Goal: "作出选择", Conflict: "守城与救人", Outcome: "开城", Position: 3},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid roadmap: %v", err)
	}
	valid.NearTermChapters[2].Position = 4
	if !errors.Is(valid.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected gap in chapter positions to fail")
	}
	valid.NearTermChapters[2].Position = 3
	valid.NearTermChapters = valid.NearTermChapters[:2]
	if !errors.Is(valid.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected fewer than three near-term chapters to fail")
	}
}
