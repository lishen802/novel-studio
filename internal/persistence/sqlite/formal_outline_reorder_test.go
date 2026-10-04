package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

func seedFormalReorder(t *testing.T) (*Store, asset.FormalOutlineReorderRequest) {
	t.Helper()
	ctx := context.Background()
	s, project := createTestStore(t)
	now := time.Unix(100, 0)
	for i, id := range []string{"written", "intro", "future", "later"} {
		if err := s.CreateChapter(ctx, mustChapter(t, id, project.ID, id, i+1)); err != nil {
			t.Fatal(err)
		}
	}
	book, _ := story.NewOutlineNode("book", project.ID, nil, story.LevelBook, "全书", 1, now)
	volume, _ := story.NewOutlineNode("volume", project.ID, &book, story.LevelVolume, "第一卷", 1, now)
	arc, _ := story.NewOutlineNode("arc", project.ID, &volume, story.LevelArc, "守关", 1, now)
	for _, node := range []story.OutlineNode{book, volume, arc} {
		if err := s.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	formal, _ := asset.New("arc-asset", project.ID, asset.KindOutlineNode, asset.OriginAuthor, "", "", "", "outline_nodes", arc.ID, now)
	payload, _ := json.Marshal(map[string]any{"parentId": volume.ID, "level": "arc", "title": arc.Title, "summary": "", "goal": "", "conflict": "", "outcome": "", "position": 1})
	tx, _ := s.db.BeginTx(ctx, nil)
	if err := insertStoryAsset(ctx, tx, formal, string(payload), now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"先守城", "后破局"} {
		r := asset.FormalOutlineAppendRequest{ProjectID: project.ID, BranchID: "main", ParentAssetID: formal.ID, ChapterID: "intro", Title: title, Reason: "补足近期目标"}
		preview, err := s.PreviewFormalOutlineAppend(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		cmd := asset.FormalOutlineAppendCommand{Request: r, Digest: preview.Digest, IdempotencyKey: title}
		if _, err := s.ConfirmFormalOutlineAppend(ctx, cmd, []string{"first-node", "second-node"}[i], []string{"first-asset", "second-asset"}[i], now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE chapters SET body='已写旧章' WHERE id='written'`); err != nil {
		t.Fatal(err)
	}
	return s, asset.FormalOutlineReorderRequest{ProjectID: project.ID, BranchID: "main", FirstAssetID: "first-asset", SecondAssetID: "second-asset", ChapterID: "future", Reason: "先破局再守城"}
}

// TestFormalOutlineReorderPreservesEarlierChapters 验证未来顺序及幂等回执不改变旧章。
func TestFormalOutlineReorderPreservesEarlierChapters(t *testing.T) {
	s, request := seedFormalReorder(t)
	defer s.Close()
	ctx := context.Background()
	p, err := s.PreviewFormalOutlineReorder(ctx, request)
	if err != nil || p.FirstPosition != 1 || p.SecondPosition != 2 || p.Position != 3 {
		t.Fatalf("preview: %+v %v", p, err)
	}
	cmd := asset.FormalOutlineReorderCommand{Request: request, Digest: p.Digest, IdempotencyKey: "swap-once"}
	if _, err := s.ConfirmFormalOutlineReorder(ctx, cmd, "swap-id", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	if again, err := s.ConfirmFormalOutlineReorder(ctx, cmd, "unused", time.Unix(300, 0)); err != nil || again.Digest != p.Digest {
		t.Fatalf("replay: %+v %v", again, err)
	}
	for chapterID, want := range map[string]int{"intro": 1, "future": 2, "later": 2} {
		positions, err := s.FormalOutlinePositionsAtChapter(ctx, request.ProjectID, chapterID)
		if err != nil || positions["first-node"] != want || positions["second-node"] != 3-want {
			t.Fatalf("%s positions: %#v %v", chapterID, positions, err)
		}
		effective, err := s.EffectiveConstraints(ctx, map[string]int{"intro": 2, "future": 3, "later": 4}[chapterID])
		if err != nil {
			t.Fatal(err)
		}
		for _, node := range effective {
			if node.TypedID == "first-node" && node.Position != want {
				t.Fatalf("%s effective first node: %+v", chapterID, node)
			}
		}
	}
	var original int
	if err := s.db.QueryRowContext(ctx, `SELECT position FROM outline_nodes WHERE id='first-node'`).Scan(&original); err != nil || original != 1 {
		t.Fatalf("typed original changed: %d %v", original, err)
	}
	if _, err := s.PreviewFormalOutlineReorder(ctx, request); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("same boundary should require a later chapter: %v", err)
	}
}

// TestFormalOutlineReorderRequiresForwardAdjacentTargets 验证选择顺序必须与当前顺序一致。
func TestFormalOutlineReorderRequiresForwardAdjacentTargets(t *testing.T) {
	s, request := seedFormalReorder(t)
	defer s.Close()
	request.FirstAssetID, request.SecondAssetID = request.SecondAssetID, request.FirstAssetID
	if _, err := s.PreviewFormalOutlineReorder(context.Background(), request); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("reverse order should be rejected: %v", err)
	}
}

// TestFormalOutlineReorderRejectsNewFutureWriting 验证预览失效不产生部分修订。
func TestFormalOutlineReorderRejectsNewFutureWriting(t *testing.T) {
	s, request := seedFormalReorder(t)
	defer s.Close()
	ctx := context.Background()
	p, err := s.PreviewFormalOutlineReorder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE chapters SET body='作者新正文' WHERE id='later'`); err != nil {
		t.Fatal(err)
	}
	cmd := asset.FormalOutlineReorderCommand{Request: request, Digest: p.Digest, IdempotencyKey: "stale"}
	if _, err := s.ConfirmFormalOutlineReorder(ctx, cmd, "failed", time.Unix(200, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("confirm after writing: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM formal_outline_reorders`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial reorder: %d %v", count, err)
	}
}

// TestFormalOutlineReorderRollsBackBothAssets 验证第二节点写入失败时无部分版本。
func TestFormalOutlineReorderRollsBackBothAssets(t *testing.T) {
	s, request := seedFormalReorder(t)
	defer s.Close()
	ctx := context.Background()
	p, err := s.PreviewFormalOutlineReorder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER fail_second_reorder BEFORE INSERT ON story_asset_revisions WHEN NEW.asset_id='second-asset' AND NEW.revision=2 BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	cmd := asset.FormalOutlineReorderCommand{Request: request, Digest: p.Digest, IdempotencyKey: "failed-write"}
	if _, err := s.ConfirmFormalOutlineReorder(ctx, cmd, "failed", time.Unix(200, 0)); err == nil {
		t.Fatal("second revision failure should roll back the operation")
	}
	for _, id := range []string{"first-asset", "second-asset"} {
		var revision, changes int
		if err := s.db.QueryRowContext(ctx, `SELECT current_revision FROM story_assets WHERE id=?`, id).Scan(&revision); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM story_constraint_changes WHERE asset_id=?`, id).Scan(&changes); err != nil {
			t.Fatal(err)
		}
		if revision != 1 || changes != 0 {
			t.Fatalf("partial write for %s: revision=%d changes=%d", id, revision, changes)
		}
	}
}
