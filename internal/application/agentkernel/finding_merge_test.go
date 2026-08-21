package agentkernel

import "testing"

func TestMergeReviewContentsIsDeterministicAcrossSplitReviewOrder(t *testing.T) {
	first := `{"quality_status":"needs_review","summary":"连续性","findings":[{"id":"b","category":"continuity","severity":"major","status":"open","source":"model_review","message":"状态冲突","constraint_ref":"canon:1"}]}`
	second := `{"quality_status":"needs_review","summary":"内容","findings":[{"id":"a","category":"motivation","severity":"minor","status":"open","source":"model_review","message":"动机不足","constraint_ref":"goal:1"}]}`
	one, err := mergeReviewContents([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	two, err := mergeReviewContents([]string{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if one != two {
		t.Fatalf("merged review changed with input order:\n%s\n%s", one, two)
	}
}
