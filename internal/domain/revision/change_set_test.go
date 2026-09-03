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

func TestCreateOperationsCanonicalizeBeforeValueAndRejectEmptyAfter(t *testing.T) {
	now := time.Unix(100, 0)
	tests := []struct {
		name      string
		kind      OperationKind
		after     string
		wantError error
	}{
		{
			name:  "create outline node",
			kind:  OperationCreateOutlineNode,
			after: `{"level":"volume","title":"第一卷","position":1}`,
		},
		{
			name:  "create character with reordered keys",
			kind:  OperationCreateCharacter,
			after: `{"motivation":"守护","name":"墨老","role":"supporting"}`,
		},
		{
			name:  "create relationship",
			kind:  OperationCreateRelationship,
			after: `{"sourceCharacterId":"c1","targetCharacterId":"c2","kind":"ally","strength":50}`,
		},
		{
			name:      "create character with empty after json rejects noop",
			kind:      OperationCreateCharacter,
			after:     `{}`,
			wantError: ErrNoopOperation,
		},
		{
			name:      "create fact with invalid json rejects",
			kind:      OperationCreateFact,
			after:     `{not-json`,
			wantError: ErrInvalidOperation,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changeSet, err := NewChangeSet(
				"change-set",
				"project-1",
				"抽取候选",
				"",
				"extraction-task:1",
				[]PatchOperationSpec{{
					ID:         "operation-1",
					Kind:       test.kind,
					TargetID:   "target-1",
					AfterValue: test.after,
				}},
				now,
			)
			if test.wantError != nil {
				if !errors.Is(err, test.wantError) {
					t.Fatalf("error = %v, want %v", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewChangeSet: %v", err)
			}
			operation := changeSet.Operations[0]
			if operation.BeforeValue != "{}" {
				t.Fatalf("before_value = %q, want {}", operation.BeforeValue)
			}
			if operation.BaseRevision != 0 {
				t.Fatalf("base_revision = %d, want 0", operation.BaseRevision)
			}
			if operation.BaseFingerprint != EmptyObjectFingerprint() {
				t.Fatalf("base_fingerprint = %q, want %q", operation.BaseFingerprint, EmptyObjectFingerprint())
			}
			if operation.AfterValue == "{}" {
				t.Fatalf("after_value should not be canonical-noop")
			}
		})
	}
}

func TestCreateOperationDistinguishesFromUpdateKinds(t *testing.T) {
	createKinds := []OperationKind{
		OperationCreateOutlineNode,
		OperationCreateCharacter,
		OperationCreateRelationship,
		OperationCreateWorldRule,
		OperationCreateStoryEvent,
		OperationCreateFact,
		OperationCreateForeshadowingHook,
		OperationCreateFaction,
		OperationCreateLocation,
		OperationCreateAnchor,
		OperationCreateStoryAsset,
	}
	for _, kind := range createKinds {
		if !IsCreateOperation(kind) {
			t.Fatalf("IsCreateOperation(%q) = false, want true", kind)
		}
	}
	for _, kind := range []OperationKind{
		OperationReplaceChapterBody,
		OperationUpdateOutlineNode,
		OperationUpdateCharacter,
		OperationUpdateFact,
		OperationUpdateProject,
	} {
		if IsCreateOperation(kind) {
			t.Fatalf("IsCreateOperation(%q) = true, want false", kind)
		}
	}
}

func TestPatchOperationSpecEvidenceIsCopiedDefensively(t *testing.T) {
	now := time.Unix(100, 0)
	evidence := []Evidence{{
		ChapterID: "chapter-1", ChapterTitle: "第一章", StartLine: 10, EndLine: 20, Excerpt: "引文",
	}}
	changeSet, err := NewChangeSet(
		"change-set",
		"project-1",
		"抽取候选",
		"",
		"extraction-task:1",
		[]PatchOperationSpec{{
			ID:         "operation-1",
			Kind:       OperationCreateCharacter,
			TargetID:   "character-1",
			AfterValue: `{"name":"墨老","role":"supporting"}`,
			Evidence:   evidence,
		}},
		now,
	)
	if err != nil {
		t.Fatalf("NewChangeSet: %v", err)
	}
	operation := changeSet.Operations[0]
	if len(operation.Evidence) != 1 || operation.Evidence[0].ChapterID != "chapter-1" {
		t.Fatalf("evidence = %#v", operation.Evidence)
	}
	// Mutating the input slice after construction must not affect the operation.
	evidence[0].Excerpt = "tampered"
	if operation.Evidence[0].Excerpt == "tampered" {
		t.Fatal("evidence slice was not copied defensively")
	}
}
