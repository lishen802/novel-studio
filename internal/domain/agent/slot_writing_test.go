package agent

import "testing"

func TestChapterSkeletonRejectsInvalidSlots(t *testing.T) {
	valid := testSkeleton()
	cases := []func(*ChapterSkeletonOutput){
		func(value *ChapterSkeletonOutput) { value.Beats[0].Slots[1].ID = value.Beats[0].Slots[0].ID },
		func(value *ChapterSkeletonOutput) { value.Beats[0].Slots[0].Type = "monologue" },
		func(value *ChapterSkeletonOutput) { value.Beats[0].Slots = nil },
	}
	for _, mutate := range cases {
		value := valid
		value.Beats = append([]SkeletonBeat(nil), valid.Beats...)
		value.Beats[0].Slots = append([]SkeletonSlot(nil), valid.Beats[0].Slots...)
		mutate(&value)
		if value.Validate() == nil {
			t.Fatalf("invalid skeleton accepted: %+v", value)
		}
	}
}

func TestMechanicalJoinNormalizesLineEndingsAndUsesRuneOffsets(t *testing.T) {
	skeleton := testSkeleton()
	joined, err := MechanicalJoin(skeleton,
		FillSlotOutput{FilledSlots: []FilledSlot{{ID: "s2", Content: "e\u0301起身。"}}},
		FillSlotOutput{FilledSlots: []FilledSlot{{ID: "s1", Content: "“走。”\r\n她说。"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if joined.Body != "“走。”\n她说。\n\ne\u0301起身。" || joined.Spans[0].Start != 0 || joined.Spans[0].End != 8 || joined.Spans[1].Start != 10 || joined.Spans[1].End != 15 {
		t.Fatalf("join = %+v", joined)
	}
}

func TestMechanicalJoinRejectsMissingDuplicateAndInventedSlots(t *testing.T) {
	skeleton := testSkeleton()
	cases := [][]FillSlotOutput{
		{{FilledSlots: []FilledSlot{{ID: "s1", Content: "甲"}}}},
		{{FilledSlots: []FilledSlot{{ID: "s1", Content: "甲"}, {ID: "s1", Content: "乙"}, {ID: "s2", Content: "丙"}}}},
		{{FilledSlots: []FilledSlot{{ID: "s1", Content: "甲"}, {ID: "s2", Content: "乙"}, {ID: "s3", Content: "丙"}}}},
	}
	for _, groups := range cases {
		if _, err := MechanicalJoin(skeleton, groups...); err == nil {
			t.Fatalf("invalid fill accepted: %+v", groups)
		}
	}
}

func testSkeleton() ChapterSkeletonOutput {
	return ChapterSkeletonOutput{ChapterGoal: "逃离", Beats: []SkeletonBeat{{ID: "b1", Summary: "冲出房间", POV: "林岚", Slots: []SkeletonSlot{
		{ID: "s1", Type: SlotDialogue, Intent: "催促", TargetRunes: 20, SourceRefs: []string{"anchor@1"}},
		{ID: "s2", Type: SlotAction, Intent: "起身", TargetRunes: 20, SourceRefs: []string{"anchor@1"}},
	}}}}
}
