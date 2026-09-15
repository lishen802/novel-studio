package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	story "github.com/lishen802/novel-studio/internal/domain/story"
	"testing"
	"time"
)

func TestOutlineTitleEditPreservesPlanning(t *testing.T) {
	ctx := context.Background()
	s, p := createTestStore(t)
	defer s.Close()
	now := time.Now().UTC()
	for _, parent := range []story.OutlineNode{{ID: "b", ProjectID: p.ID, Level: story.LevelBook, Title: "全书", Position: 1, CreatedAt: now, UpdatedAt: now}, {ID: "v", ParentID: "b", ProjectID: p.ID, Level: story.LevelVolume, Title: "卷", Position: 1, CreatedAt: now, UpdatedAt: now}} {
		if err := s.CreateOutlineNode(ctx, parent); err != nil {
			t.Fatal(err)
		}
	}
	n := story.OutlineNode{ParentID: "v", ID: "review-arc", ProjectID: p.ID, Level: story.LevelArc, Title: "旧标题", Position: 1, PlanningJSON: `{"reader_expectation":"保留这份规划"}`, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateOutlineNode(ctx, n); err != nil {
		t.Fatal(err)
	}
	before, _ := canonicalJSON(outlineNodeValue{Title: n.Title, Planning: []byte(n.PlanningJSON)})
	sum := sha256.Sum256([]byte(before))
	after, _ := canonicalJSON(outlineNodeValue{Title: "新标题"})
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// 仅规划改变时，旧标题修改也必须冲突。
	if _, err = tx.ExecContext(ctx, `UPDATE outline_nodes SET planning_json='{"reader_expectation":"新规划"}' WHERE id=?`, n.ID); err != nil {
		t.Fatal(err)
	}
	_, staleErr := prepareOutlineOperation(ctx, tx, revision.ChangeSet{ProjectID: p.ID}, revision.PatchOperation{TargetID: n.ID, BeforeValue: before, BaseFingerprint: hex.EncodeToString(sum[:]), AfterValue: after}, now)
	if staleErr == nil {
		t.Fatal("规划变化未触发基线冲突")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE outline_nodes SET planning_json=? WHERE id=?", n.PlanningJSON, n.ID); err != nil {
		t.Fatal(err)
	}
	op, err := prepareOutlineOperation(ctx, tx, revision.ChangeSet{ProjectID: p.ID}, revision.PatchOperation{TargetID: n.ID, BeforeValue: before, BaseFingerprint: hex.EncodeToString(sum[:]), AfterValue: after}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = op.apply(); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("title-only edit: planning before=%s after=%s", n.PlanningJSON, nodes[2].PlanningJSON)
	if nodes[2].PlanningJSON != n.PlanningJSON {
		t.Error("标题更新不应清空规划")
	}
}
