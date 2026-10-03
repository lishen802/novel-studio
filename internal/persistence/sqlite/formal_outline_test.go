package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// 新的近期章节节点只对边界后的正式读取可见，旧正文与重复确认保持原样。
func TestFormalOutlineAppendHasFutureBoundaryAndReceipt(t *testing.T) {
	ctx := context.Background()
	s, project := createTestStore(t)
	defer s.Close()
	now := time.Unix(100, 0)
	for i, id := range []string{"old", "future"} {
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
	r := asset.FormalOutlineAppendRequest{ProjectID: project.ID, BranchID: "main", ParentAssetID: formal.ID, ChapterID: "future", Title: "逼近城门", Summary: "守关人的抉择", Goal: "查清来船", Reason: "补足后续章节目标"}
	p, err := s.PreviewFormalOutlineAppend(ctx, r)
	if err != nil || p.Position != 2 || p.NodePosition != 1 {
		t.Fatalf("preview: %+v %v", p, err)
	}
	cmd := asset.FormalOutlineAppendCommand{Request: r, Digest: p.Digest, IdempotencyKey: "append-once"}
	first, err := s.ConfirmFormalOutlineAppend(ctx, cmd, "future-node", "future-asset", time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ConfirmFormalOutlineAppend(ctx, cmd, "unused-node", "unused-asset", time.Unix(300, 0))
	if err != nil || first.NodeID != second.NodeID || first.AssetID != second.AssetID {
		t.Fatalf("replay: %+v %v", second, err)
	}
	for position, want := range map[int]bool{1: false, 2: true} {
		versions, e := s.EffectiveConstraints(ctx, position)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, v := range versions {
			if v.AssetID == first.AssetID {
				found = true
			}
		}
		if found != want {
			t.Fatalf("node visibility at %d: %v", position, found)
		}
	}
	boundaries, err := s.FormalOutlineIntroductions(ctx, project.ID)
	if err != nil || boundaries[first.NodeID] != 2 {
		t.Fatalf("boundaries: %v %v", boundaries, err)
	}
	planning, err := s.PlanningRevisions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundPlanning := false
	var addedText string
	for _, row := range planning {
		if row.AssetID == first.AssetID && row.TypedID == first.NodeID {
			foundPlanning = true
			addedText = row.Current
		}
	}
	if !foundPlanning {
		t.Fatalf("new node absent from formal planning: %+v", planning)
	}
	tooEarly, err := s.PreviewConstraintChange(ctx, asset.ConstraintRequest{ProjectID: project.ID, BranchID: "main", AssetID: first.AssetID, Intent: "future_only", ChapterID: "old", Text: strings.Replace(addedText, "逼近城门", "新的章节目标", 1), Reason: "调整目标"})
	if err != nil || tooEarly.CanApply || tooEarly.BlockedReason != "生效章节不能早于该节点的新增边界" {
		t.Fatalf("revision before introduction: %+v %v", tooEarly, err)
	}
	originalArchive, err := s.PlanningArchiveAssets(ctx, project.ID)
	if err != nil || len(originalArchive) != 1 || originalArchive[0].AssetID != formal.ID {
		t.Fatalf("new author node changed original candidate mapping: %+v %v", originalArchive, err)
	}
	cmd.Request.Title = "换参"
	if _, err = s.ConfirmFormalOutlineAppend(ctx, cmd, "bad", "bad-asset", time.Unix(400, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("key reuse: %v", err)
	}
	staleRequest := r
	staleRequest.Title = "下一目标"
	stalePreview, err := s.PreviewFormalOutlineAppend(ctx, staleRequest)
	if err != nil {
		t.Fatal(err)
	}
	future, err := s.Chapter(ctx, "future")
	if err != nil {
		t.Fatal(err)
	}
	written := future
	written.Body = "作者已写的未来正文"
	written.Revision++
	if err = s.CommitChapter(ctx, written, future); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmFormalOutlineAppend(ctx, asset.FormalOutlineAppendCommand{Request: staleRequest, Digest: stalePreview.Digest, IdempotencyKey: "stale"}, "stale-node", "stale-asset", time.Unix(500, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("stale preview must fail: %v", err)
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM formal_outline_introductions WHERE project_id=?`, project.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed confirmation wrote a node: %d %v", count, err)
	}
}
