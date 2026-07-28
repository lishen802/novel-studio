package revision

import (
	"errors"
	"testing"
	"time"
)

func TestChapterBodyChangeSetRequiresReviewBeforeItCanBeApplied(t *testing.T) {
	now := time.Date(2026, 7, 28, 4, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	changeSet, err := NewChapterBodyChangeSet(
		"change-set-1",
		"operation-1",
		"project-1",
		"chapter-1",
		"重写第一章开场",
		"加强冲突",
		"agent:writer",
		3,
		"旧正文",
		"新正文",
		"更早引出危机",
		now,
	)
	if err != nil {
		t.Fatalf("new change set: %v", err)
	}
	if changeSet.Status != ChangeSetPending {
		t.Fatalf("status = %q, want pending", changeSet.Status)
	}
	if len(changeSet.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(changeSet.Operations))
	}
	operation := changeSet.Operations[0]
	if operation.Kind != OperationReplaceChapterBody || operation.BaseRevision != 3 {
		t.Fatalf("operation = %#v", operation)
	}
	if operation.BeforeValue != "旧正文" || operation.AfterValue != "新正文" {
		t.Fatalf("body patch = (%q, %q)", operation.BeforeValue, operation.AfterValue)
	}

	applied, err := changeSet.Apply(now.Add(time.Minute))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	reapplied, err := applied.Apply(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if !reapplied.UpdatedAt.Equal(applied.UpdatedAt) {
		t.Fatal("idempotent reapply changed updated_at")
	}
	if _, err := applied.Reject(now.Add(3 * time.Minute)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("reject applied error = %v, want invalid transition", err)
	}
}

func TestChapterBodyChangeSetRejectsEmptyOrNoopPatches(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		id     string
		before string
		after  string
	}{
		{name: "blank id", id: "", before: "旧", after: "新"},
		{name: "same body", id: "change-set", before: "相同", after: "相同"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewChapterBodyChangeSet(
				test.id, "operation", "project", "chapter", "标题", "", "agent",
				1, test.before, test.after, "", now,
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestStructuredChangeSetSupportsMultipleTypedOperations(t *testing.T) {
	now := time.Unix(100, 0)
	changeSet, err := NewChangeSet(
		"change-set-1",
		"project-1",
		"调整第一幕设定",
		"同步大纲和人物动机",
		"agent:planner",
		[]PatchOperationSpec{
			{
				ID:          "operation-outline",
				Kind:        OperationUpdateOutlineNode,
				TargetID:    "outline-1",
				BeforeValue: `{"title":"旧标题","summary":"旧摘要"}`,
				AfterValue:  `{"summary":"新摘要","title":"新标题"}`,
				Reason:      "强化故事承诺",
			},
			{
				ID:          "operation-character",
				Kind:        OperationUpdateCharacter,
				TargetID:    "character-1",
				BeforeValue: `{"name":"墨老","motivation":"守护"}`,
				AfterValue:  `{"name":"墨老","motivation":"赎罪"}`,
				Reason:      "补强人物动机",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(changeSet.Operations) != 2 {
		t.Fatalf("operations = %d, want 2", len(changeSet.Operations))
	}
	for _, operation := range changeSet.Operations {
		if operation.BaseRevision != 0 || operation.BaseFingerprint == "" {
			t.Fatalf("structured operation version = %#v", operation)
		}
		if operation.Status != ChangeSetPending {
			t.Fatalf("operation status = %q", operation.Status)
		}
	}
	if changeSet.Operations[0].AfterValue != `{"summary":"新摘要","title":"新标题"}` {
		t.Fatalf("canonical after value = %q", changeSet.Operations[0].AfterValue)
	}
}

func TestChangeSetSupportsEditingAndDecidingIndividualOperations(t *testing.T) {
	now := time.Unix(100, 0)
	changeSet, err := NewChangeSet(
		"change-set-1",
		"project-1",
		"结构化候选",
		"",
		"agent:planner",
		[]PatchOperationSpec{
			{
				ID:          "operation-outline",
				Kind:        OperationUpdateOutlineNode,
				TargetID:    "outline-1",
				BeforeValue: `{"title":"旧标题"}`,
				AfterValue:  `{"title":"候选标题"}`,
			},
			{
				ID:           "operation-chapter",
				Kind:         OperationReplaceChapterBody,
				TargetID:     "chapter-1",
				BaseRevision: 3,
				BeforeValue:  "旧正文",
				AfterValue:   "候选正文",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	edited, err := changeSet.EditOperation(
		"operation-outline",
		`{"title":"作者调整后的标题"}`,
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Operations[0].AfterValue != `{"title":"作者调整后的标题"}` {
		t.Fatalf("edited operation = %#v", edited.Operations[0])
	}
	partiallyApplied, err := edited.ApplyOperation(
		"operation-outline",
		now.Add(2*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if partiallyApplied.Status != ChangeSetPartiallyApplied {
		t.Fatalf("change set status = %q, want partially_applied", partiallyApplied.Status)
	}
	completed, err := partiallyApplied.RejectOperation(
		"operation-chapter",
		now.Add(3*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != ChangeSetApplied {
		t.Fatalf("completed mixed change set status = %q, want applied", completed.Status)
	}
	if _, err := completed.EditOperation(
		"operation-outline",
		`{"title":"不能再次编辑"}`,
		now.Add(4*time.Minute),
	); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("edit applied operation error = %v", err)
	}
}

func TestStructuredChangeSetRejectsInvalidJSONAndNoopOperations(t *testing.T) {
	now := time.Unix(100, 0)
	tests := []struct {
		name   string
		before string
		after  string
	}{
		{name: "invalid json", before: `{"title":"旧"}`, after: `{invalid`},
		{name: "canonical noop", before: `{"title":"相同","summary":""}`, after: `{"summary":"","title":"相同"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewChangeSet(
				"change-set",
				"project",
				"标题",
				"",
				"agent",
				[]PatchOperationSpec{{
					ID:          "operation",
					Kind:        OperationUpdateFact,
					TargetID:    "fact-1",
					BeforeValue: test.before,
					AfterValue:  test.after,
				}},
				now,
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
