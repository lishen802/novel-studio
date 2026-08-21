package agent

import "testing"

func TestEnhancedChapterDirectorSeparatesCanonicalAndSuggestions(t *testing.T) {
	output := ChapterPlanOutput{
		ChapterGoal: "取得名册", OpeningHook: "暗门开启", EndingHook: "发现内应",
		RequiredAnchors: []string{"名册"}, ForbiddenReveals: []string{"幕后身份"},
		ImprovisationBounds: []string{"不得离开宗门"}, CanonicalSourceRefs: []string{"manifest:item-1"}, CreativeSuggestions: []string{"可以安排巡逻换岗"},
		Scenes: []SceneBeat{{ID: "scene-1", Goal: "潜入", Conflict: "巡逻", Outcome: "抵达暗门", POV: "墨老", Desire: "无声通过", Obstacle: "守卫", InformationRelease: []string{"暗门位置"}, StateChange: "名册线索出现"}},
	}
	if err := output.Validate(); err != nil {
		t.Fatal(err)
	}
	output.Scenes[0].POV = ""
	if err := output.Validate(); err == nil {
		t.Fatal("enhanced director output without POV was accepted")
	}
}
