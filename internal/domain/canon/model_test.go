package canon

import (
	"errors"
	"testing"
	"time"
)

func TestModelFactCannotBecomeConfirmedOnCreation(t *testing.T) {
	_, err := NewFact("fact-1", "project-1", "潮汐会改写记忆", "world_rule", OriginModel, StatusAuthorConfirmed, .7, "", "", time.Unix(100, 0))
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("error = %v, want invalid status", err)
	}
	fact, err := NewFact("fact-1", "project-1", "潮汐会改写记忆", "world_rule", OriginModel, StatusAIInferred, .7, "event-1", "event-8", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := fact.Confirm("author", time.Unix(200, 0))
	if err != nil || confirmed.Status != StatusAuthorConfirmed || confirmed.ConfirmedBy != "author" {
		t.Fatalf("confirmed = %#v, error = %v", confirmed, err)
	}
}

func TestFactRequiresDedicatedTransitionForAuthorConfirmation(t *testing.T) {
	fact, err := NewFact("fact-1", "project-1", "雾港禁飞", "rule", OriginModel, StatusAIInferred, .8, "", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fact.Update(fact.Text, fact.Category, StatusAuthorConfirmed, 1, "", "", time.Unix(101, 0)); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("update to confirmed error = %v, want ErrInvalidStatus", err)
	}
	confirmed, err := fact.Confirm("author", time.Unix(102, 0))
	if err != nil || confirmed.ConfirmedBy != "author" {
		t.Fatalf("confirmed fact = %#v, error = %v", confirmed, err)
	}
}

func TestFactValidatesConfidenceAndRequiredValues(t *testing.T) {
	now := time.Unix(100, 0)
	if _, err := NewFact("fact-1", "project-1", "事实", "identity", OriginAuthor, StatusAuthorConfirmed, 1.1, "", "", now); !errors.Is(err, ErrInvalidConfidence) {
		t.Fatalf("confidence error = %v", err)
	}
	if _, err := NewFact("fact-1", "project-1", "", "identity", OriginAuthor, StatusAuthorConfirmed, 1, "", "", now); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank error = %v", err)
	}
}

func TestEvidenceKeepsRevisionAndTextPosition(t *testing.T) {
	evidence, err := NewEvidence("evidence-1", "project-1", "fact-1", "chapter-1", "父亲惯用的蓝黑墨水", 18, 28, 4, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.StartOffset != 18 || evidence.EndOffset != 28 || evidence.ChapterRevision != 4 {
		t.Fatalf("evidence = %#v", evidence)
	}
	if _, err := NewEvidence("evidence-2", "project-1", "fact-1", "chapter-1", "证据", 20, 10, 1, time.Now()); !errors.Is(err, ErrInvalidEvidence) {
		t.Fatalf("offset error = %v", err)
	}
}

func TestEvidencePreservesExactWhitespace(t *testing.T) {
	evidence, err := NewEvidence("evidence-1", "project-1", "fact-1", "chapter-1", "\n 原文 ", 2, 7, 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Quote != "\n 原文 " {
		t.Fatalf("quote = %q, want exact whitespace", evidence.Quote)
	}
}

func TestFactScopeAndConflictRequireValidReferences(t *testing.T) {
	scope, err := NewScope("scope-1", "project-1", "fact-1", ScopeCharacter, "character-1", time.Unix(100, 0))
	if err != nil || scope.TargetID != "character-1" {
		t.Fatalf("scope = %#v, error = %v", scope, err)
	}
	if _, err := NewScope("scope-2", "project-1", "fact-1", ScopeType("bad"), "target", time.Now()); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("scope error = %v", err)
	}
	conflict, err := NewConflict("conflict-1", "project-1", "fact-1", "fact-2", "正文记载相反", time.Unix(100, 0))
	if err != nil || conflict.Status != ConflictOpen {
		t.Fatalf("conflict = %#v, error = %v", conflict, err)
	}
	if _, err := NewConflict("conflict-2", "project-1", "fact-1", "fact-1", "", time.Now()); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("self conflict error = %v", err)
	}
}

func TestConflictCanonicalizesTheSymmetricFactPair(t *testing.T) {
	conflict, err := NewConflict("conflict-1", "project-1", "fact-z", "fact-a", "互斥", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if conflict.FactID != "fact-a" || conflict.OtherFactID != "fact-z" {
		t.Fatalf("pair = (%q, %q), want canonical order", conflict.FactID, conflict.OtherFactID)
	}
}
