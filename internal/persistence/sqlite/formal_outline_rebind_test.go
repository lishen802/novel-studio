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

// 空正文项目改绑故事线时保留目标正式身份，并与源节点停用一起提交。
func TestFormalOutlineRetirementRebindsSingleStoryLine(t *testing.T) {
	ctx := context.Background()
	s, project := createTestStore(t)
	defer s.Close()
	now := time.Unix(100, 0)
	for i, id := range []string{"old", "later"} {
		if err := s.CreateChapter(ctx, mustChapter(t, id, project.ID, id, i+1)); err != nil {
			t.Fatal(err)
		}
	}
	book, _ := story.NewOutlineNode("rebind-book", project.ID, nil, story.LevelBook, "全书", 1, now)
	volume, _ := story.NewOutlineNode("rebind-volume", project.ID, &book, story.LevelVolume, "第一卷", 1, now)
	arc, _ := story.NewOutlineNode("rebind-arc", project.ID, &volume, story.LevelArc, "守关", 1, now)
	source, _ := story.NewOutlineNode("rebind-source", project.ID, &arc, story.LevelChapter, "旧目标", 1, now)
	target, _ := story.NewOutlineNode("rebind-target", project.ID, &arc, story.LevelChapter, "新目标", 2, now)
	for _, node := range []story.OutlineNode{book, volume, arc, source, target} {
		if err := s.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	session, _ := creation.NewSession("rebind-session", project.ID, "main", now)
	if _, err := s.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	candidate, err := creation.NewCandidate("rebind-roadmap", session.ID, creation.StageRoadmap, creation.KindRoadmap, 1, "", "", "", `{"chapters":[]}`, "story_roadmap.schema", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		node story.OutlineNode
		id   string
	}{{source, "source-asset"}, {target, "target-asset"}} {
		formal, _ := asset.New(entry.id, project.ID, asset.KindOutlineNode, asset.OriginModel, candidate.ID, "", "", "outline_nodes", entry.node.ID, now)
		payload, _ := json.Marshal(map[string]any{"parentId": arc.ID, "level": "chapter", "title": entry.node.Title, "summary": "", "goal": "", "conflict": "", "outcome": "", "position": entry.node.Position, "chapterId": ""})
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
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO story_lines(id,project_id,name,status,created_at,updated_at) VALUES(?,?,'主线','planned',?,?)`, "rebind-line", project.ID, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO story_line_links(project_id,story_line_id,outline_node_id,position,created_at) VALUES(?,?,?,1,?)`, project.ID, "rebind-line", source.ID, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	r := asset.FormalOutlineRetireRequest{ProjectID: project.ID, BranchID: "main", AssetID: "source-asset", ChapterID: "later", Reason: "保留主线目标", RebindStoryLineID: "rebind-line", RebindTargetAssetID: "target-asset"}
	r.RebindTargetAssetID = "source-asset"
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("source cannot be target: %v", err)
	}
	r.RebindTargetAssetID = "target-asset"
	r.UnlinkStoryLineID = "rebind-line"
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("rebind and unlink are exclusive: %v", err)
	}
	r.UnlinkStoryLineID = ""
	if _, err := s.db.ExecContext(ctx, `UPDATE chapters SET body='已写正文' WHERE id='old'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewFormalOutlineRetire(ctx, r); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("written project: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE chapters SET body='' WHERE id='old'`); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewFormalOutlineRetire(ctx, r)
	if err != nil || p.ReboundStoryLine == nil || p.ReboundStoryLine.TargetNodeID != target.ID {
		t.Fatalf("preview: %+v %v", p, err)
	}
	cmd := asset.FormalOutlineRetireCommand{Request: r, Digest: p.Digest, IdempotencyKey: "rebind-once"}
	if _, err := s.db.ExecContext(ctx, `UPDATE outline_nodes SET title='目标已变' WHERE id=?`, target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(200, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("stale target: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE outline_nodes SET title='新目标' WHERE id=?`, target.ID); err != nil {
		t.Fatal(err)
	}
	result, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(210, 0))
	if err != nil || result.ReboundStoryLine == nil {
		t.Fatalf("confirm: %+v %v", result, err)
	}
	var linkedNode string
	if err := s.db.QueryRowContext(ctx, `SELECT outline_node_id FROM story_line_links WHERE story_line_id='rebind-line'`).Scan(&linkedNode); err != nil || linkedNode != target.ID {
		t.Fatalf("target link: %s %v", linkedNode, err)
	}
	boundaries, err := s.FormalOutlineRetirements(ctx, project.ID)
	if err != nil || boundaries[source.ID] != 2 {
		t.Fatalf("retired source boundary: %+v %v", boundaries, err)
	}
	values, err := s.EffectiveConstraints(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	var sourceVisible, targetVisible bool
	for _, value := range values {
		sourceVisible = sourceVisible || value.AssetID == "source-asset"
		targetVisible = targetVisible || value.AssetID == "target-asset"
	}
	if sourceVisible || !targetVisible {
		t.Fatalf("effective goals after rebind: source=%v target=%v", sourceVisible, targetVisible)
	}
	if replay, err := s.ConfirmFormalOutlineRetire(ctx, cmd, time.Unix(220, 0)); err != nil || replay.Digest != result.Digest {
		t.Fatalf("replay: %+v %v", replay, err)
	}
}
