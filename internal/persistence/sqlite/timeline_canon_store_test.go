package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
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

func TestStoreBlocksArchivingPolymorphicContinuityTargets(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)

	character := mustCharacter(t, "character", project.ID, "林默", worlddomain.RoleProtagonist)
	faction, _ := worlddomain.NewFaction("faction", project.ID, "守夜司", "", "守城", now)
	locationRegion, _ := worlddomain.NewWorldRegion("location-region", project.ID, nil, worlddomain.RegionWorld, "雾海", worlddomain.DirectionCenter, now)
	region, _ := worlddomain.NewWorldRegion("region", project.ID, &locationRegion, worlddomain.RegionTerritory, "镜海", worlddomain.DirectionEast, now)
	location, _ := worlddomain.NewLocation("location", project.ID, locationRegion, worlddomain.LocationCity, "雾港", "", now)
	item, _ := worlddomain.NewItem("item", project.ID, "问天剑", "", now)
	rule, _ := worlddomain.NewWorldRule("rule", project.ID, "禁飞", "不可御剑", worlddomain.RuleScopeProject, "", now)
	event, _ := timelinedomain.NewStoryEvent("event", project.ID, "入城", "", 1, 1, timelinedomain.EventOccurred, now)
	linkSourceEvent, _ := timelinedomain.NewStoryEvent("link-source-event", project.ID, "相遇", "", 2, 2, timelinedomain.EventOccurred, now)
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFaction(ctx, faction); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorldRegion(ctx, locationRegion); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorldRegion(ctx, region); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateLocation(ctx, location); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorldRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateStoryEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateStoryEvent(ctx, linkSourceEvent); err != nil {
		t.Fatal(err)
	}
	fact, _ := canondomain.NewFact("fact", project.ID, "连续性约束", "rule", canondomain.OriginAuthor, canondomain.StatusAuthorConfirmed, 1, "", "", now)
	if err := store.CreateFact(ctx, fact); err != nil {
		t.Fatal(err)
	}

	targets := []struct {
		kind canondomain.ScopeType
		id   string
	}{
		{canondomain.ScopeCharacter, character.ID}, {canondomain.ScopeFaction, faction.ID},
		{canondomain.ScopeRegion, region.ID}, {canondomain.ScopeLocation, location.ID},
		{canondomain.ScopeItem, item.ID}, {canondomain.ScopeRule, rule.ID}, {canondomain.ScopeEvent, event.ID},
	}
	for index, target := range targets {
		scope, _ := canondomain.NewScope(fmt.Sprintf("scope-%d", index), project.ID, fact.ID, target.kind, target.id, now)
		if err := store.CreateFactScope(ctx, scope); err != nil {
			t.Fatal(err)
		}
	}
	linkedCharacter := mustCharacter(t, "linked-character", project.ID, "顾青", worlddomain.RoleSupporting)
	if err := store.CreateCharacter(ctx, linkedCharacter); err != nil {
		t.Fatal(err)
	}
	link, _ := timelinedomain.NewEventLink(linkSourceEvent, timelinedomain.TargetCharacter, linkedCharacter.ID, "参与者", now)
	if err := store.UpsertStoryEventLink(ctx, link); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name    string
		archive func() error
	}{
		{"character scope", func() error { return store.ArchiveCharacter(ctx, character.ID, now) }},
		{"character event link", func() error { return store.ArchiveCharacter(ctx, linkedCharacter.ID, now) }},
		{"faction", func() error { return store.ArchiveFaction(ctx, faction.ID, now) }},
		{"region", func() error { return store.ArchiveWorldRegion(ctx, region.ID, now) }},
		{"location", func() error { return store.ArchiveLocation(ctx, location.ID, now) }},
		{"item", func() error { return store.ArchiveItem(ctx, item.ID, now) }},
		{"rule", func() error { return store.ArchiveWorldRule(ctx, rule.ID, now) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.archive(); !errors.Is(err, worlddomain.ErrReferenced) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if err := store.ArchiveStoryEvent(ctx, event.ID, now); !errors.Is(err, timelinedomain.ErrReferenced) {
		t.Fatalf("archive event error = %v", err)
	}
}

func TestStoreRejectsReverseDuplicateFactConflict(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)
	one, _ := canondomain.NewFact("fact-a", project.ID, "A", "test", canondomain.OriginAuthor, canondomain.StatusAuthorConfirmed, 1, "", "", now)
	two, _ := canondomain.NewFact("fact-b", project.ID, "B", "test", canondomain.OriginAuthor, canondomain.StatusAuthorConfirmed, 1, "", "", now)
	_ = store.CreateFact(ctx, one)
	_ = store.CreateFact(ctx, two)
	first, _ := canondomain.NewConflict("conflict-1", project.ID, one.ID, two.ID, "冲突", now)
	second := first
	second.ID, second.FactID, second.OtherFactID = "conflict-2", first.OtherFactID, first.FactID
	if err := store.CreateFactConflict(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFactConflict(ctx, second); err == nil {
		t.Fatal("reverse duplicate conflict was accepted")
	}
}
