package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
)

func TestStorePersistsTimelineFactsEvidenceScopesAndConflicts(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)
	chapter, _ := projectdomain.NewChapter("chapter-1", project.ID, "第一章", 1, now)
	chapter.Body = "信纸上写着父亲惯用的蓝黑墨水。"
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	event, _ := timelinedomain.NewStoryEvent("event-1", project.ID, "收到匿名信", "雾历七月初十", 100, 2, timelinedomain.EventOccurred, now)
	if err := store.CreateStoryEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	link, _ := timelinedomain.NewEventLink(event, timelinedomain.TargetChapter, chapter.ID, "叙述章节", now)
	if err := store.UpsertStoryEventLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	fact, _ := canondomain.NewFact("fact-1", project.ID, "匿名信使用蓝黑墨水", "evidence", canondomain.OriginText, canondomain.StatusTextEvidenced, .95, event.ID, "", now)
	other, _ := canondomain.NewFact("fact-2", project.ID, "匿名信使用红墨水", "evidence", canondomain.OriginAuthor, canondomain.StatusConflicted, .5, "", "", now)
	if err := store.CreateFact(ctx, fact); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFact(ctx, other); err != nil {
		t.Fatal(err)
	}
	evidence, _ := canondomain.NewEvidence("evidence-1", project.ID, fact.ID, chapter.ID, "父亲惯用的蓝黑墨水", 5, 15, chapter.Revision, now)
	if err := store.CreateFactEvidence(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	scope, _ := canondomain.NewScope("scope-1", project.ID, fact.ID, canondomain.ScopeEvent, event.ID, now)
	if err := store.CreateFactScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	conflict, _ := canondomain.NewConflict("conflict-1", project.ID, fact.ID, other.ID, "墨水颜色冲突", now)
	if err := store.CreateFactConflict(ctx, conflict); err != nil {
		t.Fatal(err)
	}

	events, err := store.ListStoryEvents(ctx)
	if err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("events = %#v, error = %v", events, err)
	}
	links, _ := store.ListStoryEventLinks(ctx)
	facts, _ := store.ListFacts(ctx)
	evidenceList, _ := store.ListFactEvidence(ctx)
	scopes, _ := store.ListFactScopes(ctx)
	conflicts, _ := store.ListFactConflicts(ctx)
	if len(links) != 1 || len(facts) != 2 || len(evidenceList) != 1 || len(scopes) != 1 || len(conflicts) != 1 {
		t.Fatalf("links=%#v facts=%#v evidence=%#v scopes=%#v conflicts=%#v", links, facts, evidenceList, scopes, conflicts)
	}
}

func TestStoreBlocksArchivingReferencedTimelineAndCanonRecords(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)
	event, _ := timelinedomain.NewStoryEvent("event-1", project.ID, "事件", "", 1, 1, timelinedomain.EventOccurred, now)
	_ = store.CreateStoryEvent(ctx, event)
	fact, _ := canondomain.NewFact("fact-1", project.ID, "事件发生", "history", canondomain.OriginAuthor, canondomain.StatusAuthorConfirmed, 1, event.ID, "", now)
	_ = store.CreateFact(ctx, fact)
	if err := store.ArchiveStoryEvent(ctx, event.ID, now); !errors.Is(err, timelinedomain.ErrReferenced) {
		t.Fatalf("archive event error = %v", err)
	}
	chapter, _ := projectdomain.NewChapter("chapter-1", project.ID, "第一章", 1, now)
	_ = store.CreateChapter(ctx, chapter)
	evidence, _ := canondomain.NewEvidence("evidence-1", project.ID, fact.ID, chapter.ID, "事件发生", 0, 4, chapter.Revision, now)
	_ = store.CreateFactEvidence(ctx, evidence)
	if err := store.ArchiveFact(ctx, fact.ID, now); !errors.Is(err, canondomain.ErrReferenced) {
		t.Fatalf("archive fact error = %v", err)
	}
}
