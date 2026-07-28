package story

import (
	"errors"
	"testing"
	"time"
)

func TestNewOutlineNodeBuildsTheFiveLevelHierarchy(t *testing.T) {
	now := time.Date(2026, 7, 16, 6, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	book, err := NewOutlineNode("book", "project", nil, LevelBook, "全书大纲", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	volume, err := NewOutlineNode("volume", "project", &book, LevelVolume, "第一卷", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	arc, err := NewOutlineNode("arc", "project", &volume, LevelArc, "调查失踪案", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	chapter, err := NewOutlineNode("chapter", "project", &arc, LevelChapter, "雨夜来客", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	chapter.ChapterID = "chapter-1"
	scene, err := NewOutlineNode("scene", "project", &chapter, LevelScene, "码头交锋", 1, now)
	if err != nil {
		t.Fatal(err)
	}

	if scene.ParentID != chapter.ID || scene.Level != LevelScene {
		t.Fatalf("scene = %#v", scene)
	}
	if book.CreatedAt.Location() != time.UTC || book.UpdatedAt.Location() != time.UTC {
		t.Fatalf("book times must be UTC: %#v", book)
	}
}

func TestNewOutlineNodeRejectsSkippedAndCrossProjectParents(t *testing.T) {
	now := time.Now()
	book, err := NewOutlineNode("book", "project", nil, LevelBook, "全书大纲", 1, now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewOutlineNode("chapter", "project", &book, LevelChapter, "第一章", 1, now)
	if !errors.Is(err, ErrInvalidHierarchy) {
		t.Fatalf("skipped level error = %v", err)
	}
	_, err = NewOutlineNode("volume", "other-project", &book, LevelVolume, "第一卷", 1, now)
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("cross-project error = %v", err)
	}
}

func TestOutlineNodeUpdatesAuthorFacingContentWithoutChangingStructure(t *testing.T) {
	now := time.Unix(100, 0)
	node, err := NewOutlineNode(
		"book",
		"project",
		nil,
		LevelBook,
		"旧标题",
		1,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	node.Summary = "旧摘要"

	updated, err := node.UpdateContent(
		"新标题",
		"新摘要",
		"新目标",
		"新冲突",
		"新结果",
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "新标题" ||
		updated.Summary != "新摘要" ||
		updated.Goal != "新目标" ||
		updated.Conflict != "新冲突" ||
		updated.Outcome != "新结果" {
		t.Fatalf("updated content = %#v", updated)
	}
	if updated.ID != node.ID ||
		updated.ParentID != node.ParentID ||
		updated.Level != node.Level ||
		updated.Position != node.Position {
		t.Fatalf("structural identity changed: %#v", updated)
	}
	if _, err := node.UpdateContent(
		" ",
		"",
		"",
		"",
		"",
		now,
	); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank title error = %v", err)
	}
}

func TestStoryLineLinksKeepAnExplicitOrder(t *testing.T) {
	now := time.Now()
	line, err := NewStoryLine("line", "project", "主线", "寻找失踪的导师", now)
	if err != nil {
		t.Fatal(err)
	}
	link, err := NewStoryLineLink(line, "node", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if link.StoryLineID != line.ID || link.OutlineNodeID != "node" || link.Position != 2 {
		t.Fatalf("link = %#v", link)
	}
}

func TestSceneCardRequiresChapterAndOutlineReferences(t *testing.T) {
	now := time.Now()
	card, err := NewSceneCard("scene-card", "project", "chapter", "scene-node", "码头交锋", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if card.Status != ScenePlanned || card.Position != 1 {
		t.Fatalf("card = %#v", card)
	}
	_, err = NewSceneCard("scene-card", "project", "", "scene-node", "码头交锋", 1, now)
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("missing chapter error = %v", err)
	}
}
