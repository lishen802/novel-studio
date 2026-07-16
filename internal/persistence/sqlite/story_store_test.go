package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

func TestStorePersistsStoryStructureAndReturnsStableOrder(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}

	book := mustOutlineNode(t, "book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1)
	volume := mustOutlineNode(t, "volume", project.ID, &book, storydomain.LevelVolume, "第一卷", 1)
	arc := mustOutlineNode(t, "arc", project.ID, &volume, storydomain.LevelArc, "失踪案", 1)
	chapterNode := mustOutlineNode(t, "chapter-node", project.ID, &arc, storydomain.LevelChapter, "雨夜来客", 1)
	chapterNode.ChapterID = chapter.ID
	sceneNode := mustOutlineNode(t, "scene-node", project.ID, &chapterNode, storydomain.LevelScene, "码头交锋", 1)
	for _, node := range []storydomain.OutlineNode{book, volume, arc, chapterNode, sceneNode} {
		if err := store.CreateOutlineNode(ctx, node); err != nil {
			t.Fatalf("create %s: %v", node.ID, err)
		}
	}

	line, err := storydomain.NewStoryLine("main-line", project.ID, "主线", "寻找导师", time.Unix(102, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateStoryLine(ctx, line); err != nil {
		t.Fatal(err)
	}
	link, err := storydomain.NewStoryLineLink(line, chapterNode.ID, 1, time.Unix(103, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkStoryLine(ctx, project.ID, link); err != nil {
		t.Fatal(err)
	}
	card, err := storydomain.NewSceneCard("card", project.ID, chapter.ID, sceneNode.ID, "码头交锋", 1, time.Unix(104, 0))
	if err != nil {
		t.Fatal(err)
	}
	card.Goal, card.Conflict, card.Outcome = "问出线索", "遭到伏击", "发现徽记"
	if err := store.CreateSceneCard(ctx, card); err != nil {
		t.Fatal(err)
	}

	nodes, err := store.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 5 || nodes[0].ID != book.ID || nodes[4].ID != sceneNode.ID {
		t.Fatalf("nodes = %#v", nodes)
	}
	lines, err := store.ListStoryLines(ctx)
	if err != nil || len(lines) != 1 || lines[0].ID != line.ID {
		t.Fatalf("lines = %#v, error = %v", lines, err)
	}
	links, err := store.ListStoryLineLinks(ctx)
	if err != nil || len(links) != 1 || links[0].OutlineNodeID != chapterNode.ID {
		t.Fatalf("links = %#v, error = %v", links, err)
	}
	cards, err := store.ListSceneCards(ctx, chapter.ID)
	if err != nil || len(cards) != 1 || cards[0].Outcome != "发现徽记" {
		t.Fatalf("cards = %#v, error = %v", cards, err)
	}
}

func TestArchiveOutlineNodeRejectsActiveReferences(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	book := mustOutlineNode(t, "book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1)
	volume := mustOutlineNode(t, "volume", project.ID, &book, storydomain.LevelVolume, "第一卷", 1)
	if err := store.CreateOutlineNode(ctx, book); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOutlineNode(ctx, volume); err != nil {
		t.Fatal(err)
	}

	err := store.ArchiveOutlineNode(ctx, book.ID, time.Unix(200, 0))
	if !errors.Is(err, storydomain.ErrReferenced) {
		t.Fatalf("error = %v, want ErrReferenced", err)
	}
	if err := store.ArchiveOutlineNode(ctx, volume.ID, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != book.ID {
		t.Fatalf("active nodes = %#v", nodes)
	}
}

func TestSwapOutlineNodePositionsMovesOnlyTheSelectedSiblings(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	book := mustOutlineNode(t, "book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1)
	first := mustOutlineNode(t, "volume-1", project.ID, &book, storydomain.LevelVolume, "第一卷", 1)
	second := mustOutlineNode(t, "volume-2", project.ID, &book, storydomain.LevelVolume, "第二卷", 2)
	for _, node := range []storydomain.OutlineNode{book, first, second} {
		if err := store.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.SwapOutlineNodePositions(ctx, first.ID, second.ID, time.Unix(300, 0)); err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	positions := map[string]int{}
	for _, node := range nodes {
		positions[node.ID] = node.Position
	}
	if positions[first.ID] != 2 || positions[second.ID] != 1 || positions[book.ID] != 1 {
		t.Fatalf("positions = %#v", positions)
	}
}

func mustOutlineNode(t *testing.T, id, projectID string, parent *storydomain.OutlineNode, level storydomain.OutlineLevel, title string, position int) storydomain.OutlineNode {
	t.Helper()
	node, err := storydomain.NewOutlineNode(id, projectID, parent, level, title, position, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	return node
}
