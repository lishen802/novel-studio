package creation

import (
	"testing"
	"time"
)

func mustReview(t *testing.T) CandidateReview {
	t.Helper()
	review, err := NewCandidateReview(mustProposal(t), time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	return review
}

// 逐项决定互斥：接受会清除排除，排除会清除接受。
func TestDecideKeepsAcceptAndRejectExclusive(t *testing.T) {
	character, err := NewReviewRef(ReviewScopeEntity, "character:hero")
	if err != nil {
		t.Fatal(err)
	}
	review := mustReview(t)
	review, err = review.Decide(character, DecisionReject, time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if review.State(character) != DecisionReject {
		t.Fatalf("state = %q", review.State(character))
	}
	review, err = review.Decide(character, DecisionAccept, time.Unix(302, 0))
	if err != nil {
		t.Fatal(err)
	}
	if review.State(character) != DecisionAccept || len(review.Rejected) != 0 {
		t.Fatalf("review = %#v", review)
	}
	review, err = review.Decide(character, DecisionUnreviewed, time.Unix(303, 0))
	if err != nil {
		t.Fatal(err)
	}
	if review.State(character) != DecisionUnreviewed || len(review.Accepted) != 0 {
		t.Fatalf("review = %#v", review)
	}
}

// 重复决定幂等，不因重复点击推进版本时间戳。
func TestDecideIsIdempotent(t *testing.T) {
	section, err := NewReviewRef(ReviewScopeSection, "position")
	if err != nil {
		t.Fatal(err)
	}
	review := mustReview(t)
	once, err := review.Decide(section, DecisionAccept, time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := once.Decide(section, DecisionAccept, time.Unix(302, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !sameRefs(once.Accepted, twice.Accepted) || !twice.UpdatedAt.Equal(once.UpdatedAt) {
		t.Fatalf("repeated decide changed review: %#v", twice)
	}
}

// 显式全接受只覆盖给定单位，未列出的单位仍为未审阅。
func TestAcceptAllLeavesMissingRefsUnreviewed(t *testing.T) {
	first, _ := NewReviewRef(ReviewScopeEntity, "character:hero")
	second, _ := NewReviewRef(ReviewScopeEntity, "location:port")
	review := mustReview(t)
	review, err := review.Decide(first, DecisionReject, time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	review, err = review.AcceptAll([]ReviewRef{first}, time.Unix(302, 0))
	if err != nil {
		t.Fatal(err)
	}
	if review.State(first) != DecisionAccept {
		t.Fatalf("accepted cleared: %#v", review)
	}
	open := review.Unreviewed([]ReviewRef{first, second})
	if len(open) != 1 || open[0].ID != second.ID {
		t.Fatalf("open refs = %#v", open)
	}
}

func TestNewReviewRefRejectsUnknownScope(t *testing.T) {
	if _, err := NewReviewRef(ReviewScope("category"), "character"); err == nil {
		t.Fatal("unknown scope must be rejected")
	}
	if _, err := NewReviewRef(ReviewScopeEntity, "  "); err == nil {
		t.Fatal("blank id must be rejected")
	}
}

// 审阅位置使用独立 view_version，移动不改变会话内容版本。
func TestReviewCursorMovedKeepsViewVersion(t *testing.T) {
	cursor := NewReviewCursor("session-1", time.Unix(300, 0))
	if cursor.ViewVersion != 1 {
		t.Fatalf("view version = %d", cursor.ViewVersion)
	}
	moved, err := cursor.Moved(StagePeopleWorld, "cand-2", "character:hero", "", "", time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if moved.ViewVersion != cursor.ViewVersion || moved.EntityID != "character:hero" || moved.Stage != StagePeopleWorld {
		t.Fatalf("moved = %#v", moved)
	}
	if _, err := cursor.Moved(Stage(""), "", "", "", "", time.Unix(301, 0)); err == nil {
		t.Fatal("invalid stage must be rejected")
	}
}
