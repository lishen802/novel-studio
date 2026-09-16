package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// seedReviewCandidate 建立会话与一个候选行，供审阅持久化测试使用。
func seedReviewCandidate(t *testing.T, store *Store, projectID string) (creationdomain.Session, creationdomain.Candidate) {
	t.Helper()
	ctx := context.Background()
	now := time.Unix(700, 0).UTC()
	session, err := creationdomain.NewSession("session-review", projectID, "main", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = store.CreateCreationSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := creationdomain.NewCandidate(
		"candidate-review", session.ID, creationdomain.StagePeopleWorld, creationdomain.KindPeopleWorld,
		1, "", "run-review", "artifact-review", `{"characters":[],"world_rules":[]}`, "story_cast.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	return session, candidate
}

// 审阅关系按候选版本持久化，并以 review_version 拒绝过期写入。
func TestSaveCreationReviewGuardsReviewVersion(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	_, candidate := seedReviewCandidate(t, store, project.ID)

	empty, err := store.CreationReview(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if empty.CandidateID != "" {
		t.Fatalf("missing review must read as empty: %#v", empty)
	}
	ref, err := creationdomain.NewReviewRef(creationdomain.ReviewScopeEntity, "character:char-1")
	if err != nil {
		t.Fatal(err)
	}
	created, err := creationdomain.NewCandidateReview(candidate, time.Unix(701, 0))
	if err != nil {
		t.Fatal(err)
	}
	created, err = created.Decide(ref, creationdomain.DecisionAccept, time.Unix(701, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationReview(ctx, creationdomain.CandidateReview{}, created); err != nil {
		t.Fatal(err)
	}
	stored, err := store.CreationReview(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ReviewVersion != 1 || len(stored.Accepted) != 1 || stored.Accepted[0].ID != "character:char-1" {
		t.Fatalf("stored review = %#v", stored)
	}

	rejected := stored
	rejected, err = rejected.Decide(ref, creationdomain.DecisionReject, time.Unix(702, 0))
	if err != nil {
		t.Fatal(err)
	}
	// 过期版本写入必须冲突，不能覆盖已提交的审阅关系。
	stale := creationdomain.CandidateReview{CandidateID: candidate.ID, SessionID: candidate.SessionID, ReviewVersion: 0}
	if err := store.SaveCreationReview(ctx, stale, rejected); !errors.Is(err, creationdomain.ErrReviewVersionConflict) {
		t.Fatalf("stale write error = %v", err)
	}
	if err := store.SaveCreationReview(ctx, stored, rejected); err != nil {
		t.Fatal(err)
	}
	updated, err := store.CreationReview(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ReviewVersion != 2 || len(updated.Accepted) != 0 || len(updated.Rejected) != 1 {
		t.Fatalf("updated review = %#v", updated)
	}
	if !updated.UpdatedAt.Equal(time.Unix(702, 0).UTC()) {
		t.Fatalf("updated at = %v", updated.UpdatedAt)
	}
}

// 审阅位置按独立 view_version 保存，缺少会话时读取返回 sql.ErrNoRows。
func TestSaveCreationReviewCursorGuardsViewVersion(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	session, candidate := seedReviewCandidate(t, store, project.ID)

	if _, err := store.CreationReviewCursor(ctx, session.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing cursor error = %v", err)
	}
	cursor := creationdomain.NewReviewCursor(session.ID, time.Unix(701, 0))
	cursor.Stage = creationdomain.StagePeopleWorld
	cursor.EntityID = "character:char-1"
	cursor.CandidateID = candidate.ID
	if err := store.SaveCreationReviewCursor(ctx, cursor); err != nil {
		t.Fatal(err)
	}
	stored, err := store.CreationReviewCursor(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ViewVersion != 1 || stored.EntityID != "character:char-1" || stored.Stage != creationdomain.StagePeopleWorld {
		t.Fatalf("stored cursor = %#v", stored)
	}

	moved := cursor
	moved.EntityID = "rule:潮汐律"
	moved.ViewVersion = 0
	if err := store.SaveCreationReviewCursor(ctx, moved); !errors.Is(err, creationdomain.ErrReviewVersionConflict) {
		t.Fatalf("stale cursor error = %v", err)
	}
	moved.ViewVersion = stored.ViewVersion
	if err := store.SaveCreationReviewCursor(ctx, moved); err != nil {
		t.Fatal(err)
	}
	after, err := store.CreationReviewCursor(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ViewVersion != 2 || after.EntityID != "rule:潮汐律" {
		t.Fatalf("after cursor = %#v", after)
	}
}

// 审阅行受外键约束：不存在的候选不能写入审阅记录。
func TestSaveCreationReviewRequiresCandidate(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	session, candidate := seedReviewCandidate(t, store, project.ID)
	orphan := creationdomain.CandidateReview{
		CandidateID: "candidate-missing", SessionID: session.ID, Stage: creationdomain.StagePeopleWorld,
		UpdatedAt: time.Unix(701, 0),
	}
	if err := store.SaveCreationReview(ctx, creationdomain.CandidateReview{}, orphan); err == nil {
		t.Fatal("review of a missing candidate must fail")
	}
	if _, err := store.CreationReview(ctx, candidate.ID); err != nil {
		t.Fatal(err)
	}
}

// 删除候选行时审阅关系同步级联删除，避免悬空审阅。
func TestCreationReviewCascadesWithCandidate(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	_, candidate := seedReviewCandidate(t, store, project.ID)
	review, err := creationdomain.NewCandidateReview(candidate, time.Unix(701, 0))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := creationdomain.NewReviewRef(creationdomain.ReviewScopeEntity, "character:char-1")
	if err != nil {
		t.Fatal(err)
	}
	review, err = review.Decide(ref, creationdomain.DecisionAccept, time.Unix(701, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationReview(ctx, creationdomain.CandidateReview{}, review); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM creation_candidates WHERE id = ?`, candidate.ID); err != nil {
		t.Fatal(err)
	}
	after, err := store.CreationReview(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CandidateID != "" {
		t.Fatalf("cascaded review = %#v", after)
	}
}
