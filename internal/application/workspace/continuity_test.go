package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
)

func TestContinuityLifecycleLinksEventsFactsAndEvidence(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter.Body = "信纸上写着父亲惯用的蓝黑墨水。"
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{ids: []string{"event-1", "fact-1", "evidence-1", "scope-1", "fact-2", "conflict-1"}})
	ctx := context.Background()

	event, err := service.CreateStoryEvent(ctx, StoryEventInput{Title: "收到匿名信", StoryTime: "雾历七月初十", StoryOrder: 100, NarrativeOrder: 2, Status: "occurred", Summary: "主线启动"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LinkStoryEvent(ctx, StoryEventLinkInput{EventID: event.ID, TargetType: "chapter", TargetID: chapter.ID, Role: "叙述章节"}); err != nil {
		t.Fatal(err)
	}
	fact, err := service.CreateFact(ctx, FactInput{Text: "匿名信使用蓝黑墨水", Category: "evidence", Origin: "text", Status: "text_evidenced", Confidence: .95, ValidFromEventID: event.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddFactEvidence(ctx, FactEvidenceInput{FactID: fact.ID, ChapterID: chapter.ID, Quote: "父亲惯用的蓝黑墨水", StartOffset: 5, EndOffset: 14, ChapterRevision: chapter.Revision}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddFactScope(ctx, FactScopeInput{FactID: fact.ID, TargetType: "event", TargetID: event.ID}); err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateFact(ctx, FactInput{Text: "匿名信使用红墨水", Category: "evidence", Origin: "author", Status: "author_confirmed", Confidence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateFactConflict(ctx, FactConflictInput{FactID: fact.ID, OtherFactID: other.ID, Reason: "墨水颜色冲突"}); err != nil {
		t.Fatal(err)
	}

	view, err := service.Continuity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Events) != 1 || len(view.EventLinks) != 1 || len(view.Facts) != 2 || len(view.Evidence) != 1 || len(view.Scopes) != 1 || len(view.Conflicts) != 1 {
		t.Fatalf("continuity = %#v", view)
	}
	confirmed, err := service.ConfirmFact(ctx, fact.ID)
	if err != nil || confirmed.Status != "author_confirmed" || confirmed.ConfirmedBy != "author" {
		t.Fatalf("confirmed = %#v, error = %v", confirmed, err)
	}
}

func TestContinuityRejectsInvalidModelPromotionAndEvidenceAnchor(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter.Body = "正文没有这段话。"
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{ids: []string{"fact-1", "fact-2", "evidence-1"}})
	ctx := context.Background()

	if _, err := service.CreateFact(ctx, FactInput{Text: "模型结论", Category: "history", Origin: "model", Status: "author_confirmed", Confidence: .8}); !errors.Is(err, canondomain.ErrInvalidStatus) {
		t.Fatalf("model promotion error = %v", err)
	}
	fact, err := service.CreateFact(ctx, FactInput{Text: "正文事实", Category: "history", Origin: "text", Status: "text_evidenced", Confidence: .8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddFactEvidence(ctx, FactEvidenceInput{FactID: fact.ID, ChapterID: chapter.ID, Quote: "不存在的引文", StartOffset: 0, EndOffset: 6, ChapterRevision: 1}); !errors.Is(err, canondomain.ErrInvalidEvidence) {
		t.Fatalf("evidence error = %v", err)
	}
}

func TestContinuityReportsStaleEvidenceAndInvalidIntervals(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter.Body = "证据原文"
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{ids: []string{"event-late", "event-early", "fact-1", "fact-2", "evidence-1"}})
	ctx := context.Background()
	late, _ := service.CreateStoryEvent(ctx, StoryEventInput{Title: "后发生", StoryOrder: 20, NarrativeOrder: 1, Status: "occurred"})
	early, _ := service.CreateStoryEvent(ctx, StoryEventInput{Title: "先发生", StoryOrder: 10, NarrativeOrder: 1, Status: "occurred"})
	if _, err := service.CreateFact(ctx, FactInput{Text: "倒置事实", Category: "history", Origin: "author", Status: "author_confirmed", Confidence: 1, ValidFromEventID: late.ID, ValidUntilEventID: early.ID}); !errors.Is(err, canondomain.ErrInvalidReference) {
		t.Fatalf("interval error = %v", err)
	}
	fact, _ := service.CreateFact(ctx, FactInput{Text: "证据事实", Category: "history", Origin: "text", Status: "text_evidenced", Confidence: 1})
	_, _ = service.AddFactEvidence(ctx, FactEvidenceInput{FactID: fact.ID, ChapterID: chapter.ID, Quote: "证据原文", StartOffset: 0, EndOffset: 4, ChapterRevision: 1})
	chapter.Revision = 2
	chapter.Body = "正文已修改"
	store.chapters[chapter.ID] = chapter
	view, err := service.Continuity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range view.ConsistencyIssues {
		if issue.Code == "stale_fact_evidence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues = %#v", view.ConsistencyIssues)
	}
}

func TestContinuityUsesLicenseGate(t *testing.T) {
	denied := errors.New("license denied")
	service := NewService(&fakePackageManager{}, &fakeStoreFactory{}, &fakeLicense{err: denied}, fixedClock{now: time.Unix(100, 0)}, &sequenceIDs{})
	if _, err := service.CreateStoryEvent(context.Background(), StoryEventInput{}); !errors.Is(err, denied) {
		t.Fatalf("error = %v", err)
	}
}
