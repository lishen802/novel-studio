package agent

import (
	"encoding/json"
	"testing"
)

func TestMergeFindingsIsDeterministicAndDoesNotInventFindings(t *testing.T) {
	first := mergeFinding("b", "人物此时知道门禁密码")
	second := mergeFinding("a", "人物此时不知道门禁密码")
	duplicate := second
	duplicate.ID = "c"
	one, err := MergeFindings([]ReviewFinding{first}, []ReviewFinding{second, duplicate})
	if err != nil {
		t.Fatal(err)
	}
	two, err := MergeFindings([]ReviewFinding{duplicate, second}, []ReviewFinding{first})
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(one)
	right, _ := json.Marshal(two)
	if string(left) != string(right) {
		t.Fatalf("merge changed with input order:\n%s\n%s", left, right)
	}
	if len(one.Findings) != 2 || len(one.Conflicts) != 1 {
		t.Fatalf("merge result = %+v", one)
	}
	for _, finding := range one.Findings {
		if finding.ID != "a" && finding.ID != "b" {
			t.Fatalf("merger invented finding %q", finding.ID)
		}
	}
}

func TestConstraintNoteRequiresCanonicalSourceReferences(t *testing.T) {
	note := ConstraintNote{ID: "note-1", Category: FindingKnowledgeBoundary, Constraint: "角色尚不知道门禁密码", SourceRefs: []string{"manifest:item-1"}}
	if err := note.Validate(); err != nil {
		t.Fatal(err)
	}
	note.SourceRefs = nil
	if err := note.Validate(); err == nil {
		t.Fatal("constraint note without source refs was accepted")
	}
}

func mergeFinding(id, message string) ReviewFinding {
	start, end := 0, 2
	return ReviewFinding{ID: id, Category: FindingKnowledgeBoundary, Severity: SeverityMajor, Status: FindingOpen, Source: FindingModelReview, Message: message, Quote: "门禁", ScopeStart: &start, ScopeEnd: &end}
}
