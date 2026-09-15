package workspace

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
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
	if len(applied.Chapters) != 1 ||
		applied.Chapters[0].Body != "警钟在雨幕中骤然响起。" ||
		applied.Chapters[0].Revision != 2 {
		t.Fatalf("applied result = %#v", applied)
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

func TestStructuredChangeProposalKeepsAssetsUntouchedAndCreatesRecoveryPointBeforeGroupApply(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	outline, err := storydomain.NewOutlineNode(
		"outline-1", project.ID, nil, storydomain.LevelBook, "旧书名", 1, time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	character := mustWorldCharacter(t, "character-1", project.ID, "林默", worlddomain.RoleProtagonist)
	fact, err := canondomain.NewFact(
		"fact-1", project.ID, "钟楼在城西", "location", canondomain.OriginText,
		canondomain.StatusTextEvidenced, .8, "", "", time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	outline.PlanningJSON = `{"reader_expectation":"保留规划"}`
	store.outlineNodes[outline.ID] = outline
	store.characters[character.ID] = character
	store.facts[fact.ID] = fact
	archive := &fakeSnapshotArchive{}
	ids := &sequenceIDs{ids: []string{"operation-outline", "operation-character", "change-set-1", "snapshot-1"}}
	service := openVersionTestService(t, store, archive, ids)
	ctx := context.Background()

	pending, err := service.ProposeStructuredChange(ctx, StructuredChangeInput{
		Title:     "统一人物与大纲",
		Summary:   "候选设定仍需作者确认",
		CreatedBy: "agent:planner",
		Operations: []StructuredChangeOperationInput{
			{
				Kind: string(revisiondomain.OperationUpdateOutlineNode), TargetID: outline.ID,
				Outline: &OutlineContentInput{Title: "雾城钟声", Summary: "追查钟楼异响"},
			},
			{
				Kind: string(revisiondomain.OperationUpdateCharacter), TargetID: character.ID,
				Character: &CharacterInput{
					Name: "林默", Aliases: []string{"阿默"}, Role: "protagonist",
					Summary: "失忆的巡夜人", Status: "active",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("propose structured change: %v", err)
	}
	if pending.Status != "pending" || len(pending.Operations) != 2 ||
		pending.Operations[0].BaseFingerprint == "" {
		t.Fatalf("pending = %#v", pending)
	}
	for _, value := range []string{store.changeSets[pending.ID].Operations[0].BeforeValue, store.changeSets[pending.ID].Operations[0].AfterValue} {
		if !strings.Contains(value, "保留规划") {
			t.Fatal("普通修改未冻结原规划")
		}
	}
	if store.outlineNodes[outline.ID].Title != outline.Title ||
		store.characters[character.ID].Summary != character.Summary {
		t.Fatal("proposal changed formal assets")
	}

	applied, err := service.ApplyChangeSet(ctx, pending.ID)
	if err != nil {
		t.Fatalf("apply structured group: %v", err)
	}
	if applied.ChangeSet.Status != "applied" || len(archive.recorded) != 1 ||
		archive.recorded[0].Kind != revisiondomain.SnapshotRecovery {
		t.Fatalf("apply/recovery = (%#v, %#v)", applied, archive.recorded)
	}
}
