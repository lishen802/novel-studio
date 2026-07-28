package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

func TestCreateOutlineNodeChecksStoryCapabilityAndAssignsSiblingPosition(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	book, err := storydomain.NewOutlineNode("book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	first, err := storydomain.NewOutlineNode("volume-1", project.ID, &book, storydomain.LevelVolume, "第一卷", 1, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	store.outlineNodes[book.ID] = book
	store.outlineNodes[first.ID] = first
	licenses := &fakeLicense{}
	service := openTestServiceWithLicense(t, store, &sequenceIDs{ids: []string{"volume-2"}}, licenses)

	view, err := service.CreateOutlineNode(context.Background(), CreateOutlineNodeInput{
		ParentID: book.ID,
		Level:    string(storydomain.LevelVolume),
		Title:    "  第二卷  ",
		Summary:  "新的征程",
	})
	if err != nil {
		t.Fatal(err)
	}
	if licenses.lastCapability != license.EditStory {
		t.Fatalf("capability = %q, want %q", licenses.lastCapability, license.EditStory)
	}
	if view.ID != "volume-2" || view.ParentID != book.ID || view.Position != 2 || view.Title != "第二卷" {
		t.Fatalf("view = %#v", view)
	}
	if stored := store.outlineNodes[view.ID]; stored.Summary != "新的征程" {
		t.Fatalf("stored node = %#v", stored)
	}
}

func TestCreateSceneCardRejectsNonSceneOutlineNode(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	book, err := storydomain.NewOutlineNode("book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	store.outlineNodes[book.ID] = book
	service := openTestService(t, store, &sequenceIDs{ids: []string{"card-1"}})

	_, err = service.CreateSceneCard(context.Background(), CreateSceneCardInput{
		ChapterID: chapter.ID, OutlineNodeID: book.ID, Title: "错误场景",
	})
	if !errors.Is(err, storydomain.ErrInvalidReference) {
		t.Fatalf("error = %v, want ErrInvalidReference", err)
	}
	if len(store.sceneCards) != 0 {
		t.Fatalf("scene cards = %#v", store.sceneCards)
	}
}

func TestStoryStructureReturnsNodesLinesLinksAndCards(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	book, _ := storydomain.NewOutlineNode("book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1, time.Unix(100, 0))
	store.outlineNodes[book.ID] = book
	line, _ := storydomain.NewStoryLine("line", project.ID, "主线", "寻找导师", time.Unix(101, 0))
	store.storyLines[line.ID] = line
	store.storyLinks = []storydomain.StoryLineLink{{StoryLineID: line.ID, OutlineNodeID: book.ID, Position: 1}}
	store.sceneCards["card"] = storydomain.SceneCard{ID: "card", ProjectID: project.ID, ChapterID: chapter.ID, OutlineNodeID: book.ID, Title: "场景"}
	service := openTestService(t, store, &sequenceIDs{})

	view, err := service.StoryStructure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.OutlineNodes) != 1 || len(view.StoryLines) != 1 || len(view.StoryLineLinks) != 1 || len(view.SceneCards) != 1 {
		t.Fatalf("view = %#v", view)
	}
}

func TestMoveOutlineNodeSwapsWithTheAdjacentSibling(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	book, _ := storydomain.NewOutlineNode("book", project.ID, nil, storydomain.LevelBook, "全书大纲", 1, time.Unix(100, 0))
	first, _ := storydomain.NewOutlineNode("volume-1", project.ID, &book, storydomain.LevelVolume, "第一卷", 1, time.Unix(101, 0))
	second, _ := storydomain.NewOutlineNode("volume-2", project.ID, &book, storydomain.LevelVolume, "第二卷", 2, time.Unix(102, 0))
	store.outlineNodes[book.ID], store.outlineNodes[first.ID], store.outlineNodes[second.ID] = book, first, second
	service := openTestService(t, store, &sequenceIDs{})

	if err := service.MoveOutlineNode(context.Background(), second.ID, "up"); err != nil {
		t.Fatal(err)
	}
	if store.outlineNodes[first.ID].Position != 2 || store.outlineNodes[second.ID].Position != 1 {
		t.Fatalf("nodes = %#v", store.outlineNodes)
	}
	if err := service.MoveOutlineNode(context.Background(), second.ID, "up"); err != nil {
		t.Fatal(err)
	}
	if store.outlineNodes[second.ID].Position != 1 {
		t.Fatalf("top node moved beyond boundary: %#v", store.outlineNodes[second.ID])
	}
}

func openTestServiceWithLicense(t *testing.T, store *fakeStore, ids *sequenceIDs, licenses *fakeLicense) *Service {
	t.Helper()
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		&fakeStoreFactory{openStore: store},
		licenses,
		fixedClock{now: time.Unix(200, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}
	return service
}
