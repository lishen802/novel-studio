package creation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEntryRevisionPreservesSourceAndRejectsInvalidSwitch(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	source := "  作者原文\n主角不知道真相。  "
	first, err := NewEntryRevision("entry-1", "session", "", 1, EntryAuthorPlan, source, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Material != source || first.MaterialSHA256 == "" {
		t.Fatal("作者原文必须逐字保留")
	}
	next, err := first.Revise("entry-2", EntryIdea, source, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if next.ParentID != first.ID || next.Version != 2 || next.Material != source || first.Kind != EntryAuthorPlan {
		t.Fatal("切换必须追加版本并保留来源")
	}
	if _, err := first.Revise("entry-1", EntryIdea, source, now); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("重复身份: %v", err)
	}
	if _, err := NewEntryRevision("id", "s", "", 1, EntryAuthorPlan, "  ", now); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("空方案: %v", err)
	}
	if _, err := NewEntryRevision("id", "s", "", 1, EntryKind("import"), source, now); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("导入不能伪装为原创起点: %v", err)
	}
	if _, err := first.Revise("large", EntryIdea, strings.Repeat("字", MaxEntryMaterialRunes+1), now); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("材料应有界: %v", err)
	}
}

func TestSourceSpanUsesRunesAndExactRevision(t *testing.T) {
	revision, err := NewEntryRevision("e", "s", "", 1, EntryAuthorPlan, "甲😀乙\n丙", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	span := MaterialSpan{EntryID: revision.ID, MaterialSHA256: revision.MaterialSHA256, Start: 1, End: 3}
	got, err := span.Text(revision)
	if err != nil || got != "😀乙" {
		t.Fatalf("Unicode 范围: %q %v", got, err)
	}
	for _, bad := range []MaterialSpan{
		{EntryID: "other", MaterialSHA256: revision.MaterialSHA256, Start: 1, End: 3},
		{EntryID: revision.ID, MaterialSHA256: "bad", Start: 1, End: 3},
		{EntryID: revision.ID, MaterialSHA256: revision.MaterialSHA256, Start: -1, End: 3},
		{EntryID: revision.ID, MaterialSHA256: revision.MaterialSHA256, Start: 1, End: 99},
	} {
		if _, err := bad.Text(revision); !errors.Is(err, ErrInvalidSourceSpan) {
			t.Fatalf("非法来源未阻断: %v", err)
		}
	}
}

func TestGapReportCannotClearRequiredGapWithoutBoundResolution(t *testing.T) {
	revision, _ := NewEntryRevision("e", "s", "", 1, EntryAuthorPlan, "主角寻找失踪父亲", time.Now())
	report := PlanGapReport{ID: "g", EntryID: revision.ID, MaterialSHA256: revision.MaterialSHA256, SchemaVersion: 1,
		Items: []PlanGap{{ID: "ending", FieldPath: "/ending", Kind: GapRequired, Reason: "缺少结局方向", SuggestedAction: "补充结局", Resolution: GapOpen}}}
	if err := report.Validate(revision); err != nil {
		t.Fatal(err)
	}
	if report.Ready(revision) {
		t.Fatal("必填缺口未解决")
	}
	report.Items[0].Resolution = GapResolved
	if err := report.Validate(revision); !errors.Is(err, ErrInvalidGapReport) {
		t.Fatalf("不能无依据标记解决: %v", err)
	}
	report.Items[0].ResolutionCandidateID = "candidate"
	report.Items[0].ResolutionCandidateSHA256 = strings.Repeat("a", 64)
	if err := report.Validate(revision); err != nil {
		t.Fatal(err)
	}
	if !report.Ready(revision) {
		t.Fatal("有效解决引用应可进入后续验证")
	}
	report.Items = append(report.Items, report.Items[0])
	if err := report.Validate(revision); !errors.Is(err, ErrInvalidGapReport) {
		t.Fatalf("重复项: %v", err)
	}
}

func TestOpeningSampleHandoffIsBoundToDirectionRevision(t *testing.T) {
	now := time.Now()
	direction, err := NewCandidate("d", "s", StageDirections, KindDirectionSet, 1, "", "r", "a", `{"directions":[{"id":"second"}]}`, "story_directions.schema", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	handoff := SampleHandoff{ID: "h", SessionID: "s", DirectionCandidateID: "d", DirectionContentSHA256: direction.ContentSHA256,
		DirectionCardID: "second", PointOfView: "第三人称限知", Tone: "轻松", TargetRunes: 1200, CreatedAt: now}
	if err := handoff.Validate(direction); err != nil {
		t.Fatal(err)
	}
	changed := direction
	changed.Status = StatusStale
	if err := handoff.Validate(changed); !errors.Is(err, ErrInvalidSampleHandoff) {
		t.Fatalf("过期来源: %v", err)
	}
	handoff.DirectionCardID = "first"
	if err := handoff.Validate(direction); !errors.Is(err, ErrInvalidSampleHandoff) {
		t.Fatalf("错误方向卡: %v", err)
	}
}
