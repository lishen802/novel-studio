package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// 首次入档的叶节点停用只改变未来读取，不删除正式资产和历史规划。
func TestOriginalFormalOutlineRetirementKeepsPastAndRejectsReferences(t *testing.T) {
	ctx := context.Background()
	s, project := createTestStore(t)
	defer s.Close()
	now := time.Unix(100, 0)
	for i, id := range []string{"old", "later"} {
		if err := s.CreateChapter(ctx, mustChapter(t, id, project.ID, id, i+1)); err != nil {
			t.Fatal(err)
		}
	}
	book, _ := story.NewOutlineNode("original-book", project.ID, nil, story.LevelBook, "全书", 1, now)
	volume, _ := story.NewOutlineNode("original-volume", project.ID, &book, story.LevelVolume, "第一卷", 1, now)
	arc, _ := story.NewOutlineNode("original-arc", project.ID, &volume, story.LevelArc, "守关", 1, now)
	chapter, _ := story.NewOutlineNode("original-chapter", project.ID, &arc, story.LevelChapter, "抵达城门", 1, now)
	for _, node := range []story.OutlineNode{book, volume, arc, chapter} {
		if err := s.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	scene, _ := story.NewOutlineNode("child-scene", project.ID, &chapter, story.LevelScene, "城门场景", 1, now)
	if err := s.CreateOutlineNode(ctx, scene); err != nil {
		t.Fatal(err)
	}
	session, _ := creation.NewSession("original-session", project.ID, "main", now)
	if _, err := s.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	candidate, err := creation.NewCandidate("original-roadmap", session.ID, creation.StageRoadmap, creation.KindRoadmap, 1, "", "", "", `{"chapters":[]}`, "story_roadmap.schema", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	formal, _ := asset.New("original-chapter-asset", project.ID, asset.KindOutlineNode, asset.OriginModel, candidate.ID, "", "", "outline_nodes", chapter.ID, now)
	payload, _ := json.Marshal(map[string]any{"parentId": arc.ID, "level": "chapter", "title": chapter.Title, "summary": "", "goal": "", "conflict": "", "outcome": "", "position": 1, "chapterId": ""})
	tx, _ := s.db.BeginTx(ctx, nil)
	if err = insertStoryAsset(ctx, tx, formal, string(payload), now); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	request := asset.FormalOutlineRetireRequest{ProjectID: project.ID, BranchID: "main", AssetID: formal.ID, ChapterID: "later", Reason: "后续目标调整"}
	if _, err = s.db.ExecContext(ctx, `UPDATE story_assets SET source_candidate_id='unknown' WHERE id=?`, formal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PreviewFormalOutlineRetire(ctx, request); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("unverified archive source: %v", err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE story_assets SET source_candidate_id=? WHERE id=?`, candidate.ID, formal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO story_anchors(id,asset_id,project_id,strength,scope_kind,scope_id,statement,created_at) VALUES(?,?,?,?,?,?,?,?)`, "original-anchor", formal.ID, project.ID, "hard", "roadmap_node", chapter.ID, "保持目标", formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO story_lines(id,project_id,name,status,created_at,updated_at) VALUES(?,?,'主线','planned',?,?)`, "linked-line", project.ID, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO story_line_links(project_id,story_line_id,outline_node_id,position,created_at) VALUES(?,?,?,1,?)`, project.ID, "linked-line", chapter.ID, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO scene_cards(id,project_id,chapter_id,outline_node_id,title,position,status,created_at,updated_at) VALUES(?,?,?,?,?,1,'planned',?,?)`, "linked-card", project.ID, "old", chapter.ID, "抵达场景", formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	referencing, _ := asset.New("linked-asset", project.ID, asset.KindWorldRule, asset.OriginAuthor, "", "", "", "world_rules", "untyped", now)
	refTx, _ := s.db.BeginTx(ctx, nil)
	if err = insertStoryAsset(ctx, refTx, referencing, `{"roadmapNode":"original-chapter"}`, now); err != nil {
		t.Fatal(err)
	}
	if err = refTx.Commit(); err != nil {
		t.Fatal(err)
	}
	refs, err := s.FormalOutlineReferences(ctx, project.ID, "main", formal.ID)
	if err != nil || len(refs.References) != 5 {
		t.Fatalf("references: %+v %v", refs, err)
	}
	wantKinds := []string{"child", "story_line", "scene_card", "anchor", "formal_asset"}
	for i, kind := range wantKinds {
		if refs.References[i].Kind != kind || refs.References[i].ID == "" || refs.References[i].Label == "" {
			t.Fatalf("reference %d: %+v", i, refs.References[i])
		}
	}
	if _, err = s.PreviewFormalOutlineRetire(ctx, request); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("referenced original node: %v", err)
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM story_anchors WHERE id='original-anchor'`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`DELETE FROM scene_cards WHERE id='linked-card'`, `DELETE FROM story_line_links WHERE story_line_id='linked-line'`, `DELETE FROM story_lines WHERE id='linked-line'`, `DELETE FROM outline_nodes WHERE id='child-scene'`, `DELETE FROM story_assets WHERE id='linked-asset'`} {
		if _, err = s.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	refs, err = s.FormalOutlineReferences(ctx, project.ID, "main", formal.ID)
	if err != nil || len(refs.References) != 0 {
		t.Fatalf("cleared references: %+v %v", refs, err)
	}
	preview, err := s.PreviewFormalOutlineRetire(ctx, request)
	if err != nil || preview.IntroducedAt != 0 || preview.Position != 2 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	command := asset.FormalOutlineRetireCommand{Request: request, Digest: preview.Digest, IdempotencyKey: "original-retirement"}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO story_anchors(id,asset_id,project_id,strength,scope_kind,scope_id,statement,created_at) VALUES(?,?,?,?,?,?,?,?)`, "late-anchor", formal.ID, project.ID, "hard", "roadmap_node", chapter.ID, "新引用", formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmFormalOutlineRetire(ctx, command, time.Unix(150, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("stale preview must fail: %v", err)
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM story_anchors WHERE id='late-anchor'`); err != nil {
		t.Fatal(err)
	}
	result, err := s.ConfirmFormalOutlineRetire(ctx, command, time.Unix(200, 0))
	if err != nil || result.NodeID != chapter.ID {
		t.Fatalf("confirm: %+v %v", result, err)
	}
	replay, err := s.ConfirmFormalOutlineRetire(ctx, command, time.Unix(300, 0))
	if err != nil || replay.Digest != result.Digest {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	boundaries, err := s.FormalOutlineRetirements(ctx, project.ID)
	if err != nil || boundaries[chapter.ID] != 2 {
		t.Fatalf("boundaries: %v %v", boundaries, err)
	}
	for position, want := range map[int]bool{1: true, 2: false} {
		values, e := s.EffectiveConstraints(ctx, position)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, value := range values {
			if value.AssetID == formal.ID {
				found = true
			}
		}
		if found != want {
			t.Fatalf("visibility at %d: %v", position, found)
		}
	}
	planning, err := s.PlanningArchiveAssets(ctx, project.ID)
	if err != nil || len(planning) != 1 || planning[0].AssetID != formal.ID {
		t.Fatalf("archive still present: %+v %v", planning, err)
	}
	views, err := s.PlanningRevisions(ctx)
	if err != nil || len(views) != 1 {
		t.Fatalf("planning views: %+v %v", views, err)
	}
	change, err := s.PreviewConstraintChange(ctx, asset.ConstraintRequest{ProjectID: project.ID, BranchID: "main", AssetID: formal.ID, Intent: "future_only", ChapterID: "later", Text: strings.Replace(views[0].Current, "抵达城门", "改变目标", 1), Reason: "修订已停用节点"})
	if err != nil || change.CanApply || change.BlockedReason != "该节点已在未来章节停用，不能继续修订" {
		t.Fatalf("retired node revision: %+v %v", change, err)
	}
	command.Request.Reason = "changed"
	if _, err = s.ConfirmFormalOutlineRetire(ctx, command, time.Unix(400, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("key reuse: %v", err)
	}
}
