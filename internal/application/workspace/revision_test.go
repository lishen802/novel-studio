package workspace

import (
	"context"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/license"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestChapterChangeSetStaysPendingUntilAuthorAppliesIt(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	licenses := &fakeLicense{}
	service := openTestServiceWithLicense(
		t,
		store,
		&sequenceIDs{ids: []string{"change-set-1", "operation-1"}},
		licenses,
	)
	ctx := context.Background()

	pending, err := service.ProposeChapterChange(ctx, ChapterChangeInput{
		ChapterID:    chapter.ID,
		Title:        "重写第一章开场",
		Summary:      "让冲突更早出现",
		ProposedBody: "警钟在雨幕中骤然响起。",
		Reason:       "提升开场张力",
		CreatedBy:    "agent:writer",
	})
	if err != nil {
		t.Fatalf("propose chapter change: %v", err)
	}
	if licenses.lastCapability != license.RunAI {
		t.Fatalf("capability = %q, want run_ai", licenses.lastCapability)
	}
	if pending.Status != "pending" || pending.Operations[0].BaseRevision != 1 {
		t.Fatalf("pending view = %#v", pending)
	}
	if got := store.chapters[chapter.ID]; got != chapter {
		t.Fatalf("proposal changed formal chapter: %#v", got)
	}

	listed, err := service.ChangeSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != pending.ID {
		t.Fatalf("listed change sets = %#v", listed)
	}

	applied, err := service.ApplyChangeSet(ctx, pending.ID)
	if err != nil {
		t.Fatalf("apply change set: %v", err)
	}
	if licenses.lastCapability != license.EditManuscript {
		t.Fatalf("apply capability = %q, want edit_manuscript", licenses.lastCapability)
	}
	if applied.Body != "警钟在雨幕中骤然响起。" || applied.Revision != 2 {
		t.Fatalf("applied chapter = %#v", applied)
	}
	if store.changeSets[pending.ID].Status != revisiondomain.ChangeSetApplied {
		t.Fatalf("stored status = %q", store.changeSets[pending.ID].Status)
	}
}

func TestAuthorCanRejectChapterChangeWithoutChangingManuscript(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	service := openTestService(
		t,
		store,
		&sequenceIDs{ids: []string{"change-set-1", "operation-1"}},
	)
	ctx := context.Background()
	pending, err := service.ProposeChapterChange(ctx, ChapterChangeInput{
		ChapterID:    chapter.ID,
		Title:        "候选正文",
		ProposedBody: "不采用的内容",
		CreatedBy:    "agent:writer",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.RejectChangeSet(ctx, pending.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.RejectChangeSet(ctx, pending.ID); err != nil {
		t.Fatalf("repeat reject: %v", err)
	}
	if got := store.chapters[chapter.ID]; got != chapter {
		t.Fatalf("rejection changed formal chapter: %#v", got)
	}
	if store.changeSets[pending.ID].Status != revisiondomain.ChangeSetRejected {
		t.Fatalf("status = %q", store.changeSets[pending.ID].Status)
	}
}
