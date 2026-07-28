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
