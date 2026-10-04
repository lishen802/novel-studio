package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// 多条故事线必须全部明确选择，确认后一起解除并保留幂等回执。
func TestFormalOutlineRetirementUnlinksEverySelectedStoryLine(t *testing.T) {
	ctx := context.Background()
	s, project := createTestStore(t)
	defer s.Close()
	now := time.Unix(100, 0)
	for i, id := range []string{"old", "later"} {
		if err := s.CreateChapter(ctx, mustChapter(t, id, project.ID, id, i+1)); err != nil {
			t.Fatal(err)
		}
	}
	book, _ := story.NewOutlineNode("book-multi", project.ID, nil, story.LevelBook, "全书", 1, now)
	volume, _ := story.NewOutlineNode("volume-multi", project.ID, &book, story.LevelVolume, "第一卷", 1, now)
	arc, _ := story.NewOutlineNode("arc-multi", project.ID, &volume, story.LevelArc, "守关", 1, now)
	chapter, _ := story.NewOutlineNode("chapter-multi", project.ID, &arc, story.LevelChapter, "抵达城门", 1, now)
	for _, node := range []story.OutlineNode{book, volume, arc, chapter} {
		if err := s.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	session, _ := creation.NewSession("session-multi", project.ID, "main", now)
	if _, err := s.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	candidate, err := creation.NewCandidate("roadmap-multi", session.ID, creation.StageRoadmap, creation.KindRoadmap, 1, "", "", "", `{"chapters":[]}`, "story_roadmap.schema", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	formal, _ := asset.New("asset-multi", project.ID, asset.KindOutlineNode, asset.OriginModel, candidate.ID, "", "", "outline_nodes", chapter.ID, now)
	payload, _ := json.Marshal(map[string]any{"parentId": arc.ID, "level": "chapter", "title": chapter.Title, "summary": "", "goal": "", "conflict": "", "outcome": "", "position": 1, "chapterId": ""})
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertStoryAsset(ctx, tx, formal, string(payload), now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, line := range []struct{ id, name string }{{"line-a", "主线"}, {"line-b", "支线"}} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO story_lines(id,project_id,name,status,created_at,updated_at) VALUES(?,?,?,'planned',?,?)`, line.id, project.ID, line.name, formatTime(now), formatTime(now)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO story_line_links(project_id,story_line_id,outline_node_id,position,created_at) VALUES(?,?,?,1,?)`, project.ID, line.id, chapter.ID, formatTime(now)); err != nil {
			t.Fatal(err)
		}
	}
	r := asset.FormalOutlineRetireRequest{ProjectID: project.ID, BranchID: "main", AssetID: formal.ID, ChapterID: "later", Reason: "调整近期目标"}
	r.UnlinkStoryLineIDs = []string{"line-a"}
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("single item array: %v", err)
	}
	r.UnlinkStoryLineIDs = nil
	r.UnlinkStoryLineID = "line-a"
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("partial selection: %v", err)
	}
	r.UnlinkStoryLineID = ""
	r.UnlinkStoryLineIDs = []string{"line-b", "line-a"}
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("unstable order: %v", err)
	}
	r.UnlinkStoryLineIDs = []string{"line-a", "line-b"}
	p, err := s.PreviewFormalOutlineRetire(ctx, r)
	if err != nil || len(p.UnlinkedStoryLines) != 2 || p.UnlinkedStoryLines[0].Name != "主线" || p.UnlinkedStoryLines[1].Name != "支线" {
		t.Fatalf("preview: %+v %v", p, err)
	}
	cmd := asset.FormalOutlineRetireCommand{Request: r, Digest: p.Digest, IdempotencyKey: "multi-unlink"}
	if _, err := s.db.ExecContext(ctx, `UPDATE story_lines SET name='改名' WHERE id='line-b'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(200, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("stale name: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE story_lines SET name='支线' WHERE id='line-b'`); err != nil {
		t.Fatal(err)
	}
	result, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(210, 0))
	if err != nil || len(result.UnlinkedStoryLines) != 2 {
		t.Fatalf("confirm: %+v %v", result, err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM story_line_links WHERE outline_node_id=?`, chapter.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("remaining links: %d %v", count, err)
	}
	replay, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(220, 0))
	if err != nil || replay.Digest != result.Digest || len(replay.UnlinkedStoryLines) != 2 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
}
