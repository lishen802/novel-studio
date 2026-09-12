package sqlite

import (
	"context"
	"database/sql"
	"errors"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"testing"
	"time"
)

func TestSampleHandoffTransactionAndRecovery(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Now().UTC()
	s, _ := creationdomain.NewSession("sample-session", project.ID, "main", now)
	s, err := store.CreateCreationSession(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	c, err := creationdomain.NewCandidate("direction", s.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet, 1, "", "", "", `{"directions":[{"id":"d1"}]}`, "story_directions.schema", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationCandidate(ctx, c); err != nil {
		t.Fatal(err)
	}
	s.SelectedDirectionCandidateID = c.ID
	s.SelectedDirectionCardID = "d1"
	if err := store.SaveCreationSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	s, _ = store.CreationSession(ctx, project.ID)
	h := creationdomain.SampleHandoff{ID: "sample", SessionID: s.ID, DirectionCandidateID: c.ID, DirectionCardID: "d1", DirectionContentSHA256: c.ContentSHA256, PointOfView: "第三人称", Tone: "克制", Constraints: []string{"不揭晓谜底"}, TargetRunes: 1200, CreatedAt: now}
	if err := store.SaveCreationSample(ctx, s, c, h); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationSample(ctx, s, c, h); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("重复请求: %v", err)
	}
	var seq int
	var name, path string
	if err := store.db.QueryRowContext(ctx, "PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	restored, err := (&Store{db: db}).LatestCreationSample(ctx, s.ID)
	if err != nil || restored.ID != h.ID || restored.Constraints[0] != "不揭晓谜底" {
		t.Fatalf("恢复: %+v %v", restored, err)
	}
	s, _ = store.CreationSession(ctx, project.ID)
	before := s.ContentVersion
	changed := c
	changed.QualityStatus = creationdomain.QualityBlocked
	if err := store.UpdateCreationCandidate(ctx, changed); err != nil {
		t.Fatal(err)
	}
	h.ID = "should-rollback"
	if err := store.SaveCreationSample(ctx, s, c, h); err == nil {
		t.Fatal("变化后的方向不能保存旧快照")
	}
	s, _ = store.CreationSession(ctx, project.ID)
	if s.ContentVersion != before {
		t.Fatal("失败不能递增会话版本")
	}
	var count int
	store.db.QueryRowContext(ctx, `SELECT count(*) FROM creation_sample_handoffs`).Scan(&count)
	if count != 1 {
		t.Fatalf("出现重复或失败记录: %d", count)
	}
}
