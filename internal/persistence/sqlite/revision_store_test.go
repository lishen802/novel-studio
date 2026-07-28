package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func TestStoreAppliesChapterChangeSetAtomicallyAndIdempotently(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		"change-set-1", "operation-1", project.ID, chapter.ID,
		"重写开场", "加强冲突", "agent:writer", chapter.Revision,
		chapter.Body, "雨夜里，警钟提前响起。", "提前引出危机", time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListChangeSets(ctx)
	if err != nil {
		t.Fatalf("list change sets: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != changeSet.ID ||
		len(listed[0].Operations) != 1 {
		t.Fatalf("listed change sets = %#v", listed)
	}

	updated, err := store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(201, 0))
	if err != nil {
		t.Fatalf("apply change set: %v", err)
	}
	if len(updated.Chapters) != 1 ||
		updated.Chapters[0].Body != "雨夜里，警钟提前响起。" ||
		updated.Chapters[0].Revision != 2 {
		t.Fatalf("updated result = %#v", updated)
	}
	reapplied, err := store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(202, 0))
	if err != nil {
		t.Fatalf("reapply change set: %v", err)
	}
	if len(reapplied.Chapters) != 1 ||
		reapplied.Chapters[0].Revision != updated.Chapters[0].Revision {
		t.Fatalf("reapplied result = %#v, want %#v", reapplied, updated)
	}

	stored, err := store.ChangeSet(ctx, changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != revisiondomain.ChangeSetApplied ||
		stored.Operations[0].Status != revisiondomain.ChangeSetApplied {
		t.Fatalf("stored status = (%q, %q)", stored.Status, stored.Operations[0].Status)
	}
	var archivedBody string
	if err := store.db.QueryRowContext(
		ctx,
		"SELECT body FROM chapter_revisions WHERE chapter_id = ? AND revision = 1",
		chapter.ID,
	).Scan(&archivedBody); err != nil {
		t.Fatal(err)
	}
	if archivedBody != chapter.Body {
		t.Fatalf("archived body = %q", archivedBody)
	}
}

func TestStorePreservesChapterAndMarksStaleChangeSetConflicted(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		"change-set-1", "operation-1", project.ID, chapter.ID,
		"候选正文", "", "agent:writer", chapter.Revision,
		chapter.Body, "模型候选正文", "", time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}
	manual, err := chapter.Commit("作者刚刚保存的正文", chapter.Revision, time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitChapter(ctx, manual, chapter); err != nil {
		t.Fatal(err)
	}

	_, err = store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(202, 0))
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("apply error = %v, want revision conflict", err)
	}
	current, err := store.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Body != manual.Body || current.Revision != manual.Revision {
		t.Fatalf("chapter changed after conflict: %#v", current)
	}
	stored, err := store.ChangeSet(ctx, changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != revisiondomain.ChangeSetConflicted {
		t.Fatalf("status = %q, want conflicted", stored.Status)
	}
}

func TestStoreRejectsChangeSetIdempotently(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		"change-set-1", "operation-1", project.ID, chapter.ID,
		"候选正文", "", "agent:writer", chapter.Revision,
		chapter.Body, "不采用的正文", "", time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}
	if err := store.RejectChangeSet(ctx, changeSet.ID, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.RejectChangeSet(ctx, changeSet.ID, time.Unix(202, 0)); err != nil {
		t.Fatalf("repeat reject: %v", err)
	}
	if _, err := store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(203, 0)); !errors.Is(err, revisiondomain.ErrInvalidTransition) {
		t.Fatalf("apply rejected error = %v, want invalid transition", err)
	}
}

func TestStoreAppliesStructuredChangeSetAtomically(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()

	outline := mustOutlineNode(t, "outline-1", project.ID, nil, storydomain.LevelBook, "旧书名", 1)
	character := mustCharacter(t, "character-1", project.ID, "林默", worlddomain.RoleProtagonist)
	fact, err := canondomain.NewFact(
		"fact-1", project.ID, "钟楼在城西", "location", canondomain.OriginText,
		canondomain.StatusTextEvidenced, .8, "", "", time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOutlineNode(ctx, outline); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFact(ctx, fact); err != nil {
		t.Fatal(err)
	}

	changeSet, err := revisiondomain.NewChangeSet(
		"change-set-structured", project.ID, "统一调整设定", "让大纲、人物和事实保持一致",
		"agent:planner", []revisiondomain.PatchOperationSpec{
			{
				ID: "operation-outline", Kind: revisiondomain.OperationUpdateOutlineNode, TargetID: outline.ID,
				BeforeValue: `{"title":"旧书名","summary":"","goal":"","conflict":"","outcome":""}`,
				AfterValue:  `{"title":"雾城钟声","summary":"追查钟楼异响","goal":"找到失踪者","conflict":"城卫阻挠","outcome":"发现密道"}`,
			},
			{
				ID: "operation-character", Kind: revisiondomain.OperationUpdateCharacter, TargetID: character.ID,
				BeforeValue: `{"name":"林默","aliases":[],"role":"protagonist","summary":"","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
				AfterValue:  `{"name":"林默","aliases":["阿默"],"role":"protagonist","summary":"失忆的巡夜人","motivation":"找回记忆","fear":"再次失去同伴","secret":"曾进入钟楼","arc":"从逃避到承担","status":"active"}`,
			},
			{
				ID: "operation-fact", Kind: revisiondomain.OperationUpdateFact, TargetID: fact.ID,
				BeforeValue: `{"text":"钟楼在城西","category":"location","status":"text_evidenced","confidence":0.8,"validFromEventId":"","validUntilEventId":""}`,
				AfterValue:  `{"text":"钟楼在城北","category":"location","status":"text_evidenced","confidence":0.95,"validFromEventId":"","validUntilEventId":""}`,
			},
		}, time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}

	result, err := store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(201, 0))
	if err != nil {
		t.Fatalf("apply structured change set: %v", err)
	}
	if result.ChangeSet.Status != revisiondomain.ChangeSetApplied || len(result.Chapters) != 0 {
		t.Fatalf("apply result = %#v", result)
	}
	outlines, _ := store.ListOutlineNodes(ctx)
	if outlines[0].Title != "雾城钟声" || outlines[0].Outcome != "发现密道" {
		t.Fatalf("outline = %#v", outlines[0])
	}
	updatedCharacter, _ := store.Character(ctx, character.ID)
	if updatedCharacter.Summary != "失忆的巡夜人" || len(updatedCharacter.Aliases) != 1 {
		t.Fatalf("character = %#v", updatedCharacter)
	}
	updatedFact, _ := store.Fact(ctx, fact.ID)
	if updatedFact.Text != "钟楼在城北" || updatedFact.Confidence != .95 {
		t.Fatalf("fact = %#v", updatedFact)
	}
}

func TestStoreRollsBackStructuredGroupWhenOneBaselineIsStale(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	outline := mustOutlineNode(t, "outline-1", project.ID, nil, storydomain.LevelBook, "旧书名", 1)
	character := mustCharacter(t, "character-1", project.ID, "林默", worlddomain.RoleProtagonist)
	if err := store.CreateOutlineNode(ctx, outline); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChangeSet(
		"change-set-stale", project.ID, "整组候选", "", "agent:planner",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "operation-outline", Kind: revisiondomain.OperationUpdateOutlineNode, TargetID: outline.ID,
				BeforeValue: `{"title":"旧书名","summary":"","goal":"","conflict":"","outcome":""}`,
				AfterValue:  `{"title":"新书名","summary":"","goal":"","conflict":"","outcome":""}`,
			},
			{
				ID: "operation-character", Kind: revisiondomain.OperationUpdateCharacter, TargetID: character.ID,
				BeforeValue: `{"name":"林默","aliases":[],"role":"protagonist","summary":"","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
				AfterValue:  `{"name":"林默","aliases":[],"role":"protagonist","summary":"模型摘要","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
			},
		}, time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}
	manuallyUpdated, err := character.UpdateProfile(
		character.Name, character.Aliases, character.Role, "作者刚刚修改",
		character.Motivation, character.Fear, character.Secret, character.Arc,
		character.Status, time.Unix(201, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCharacter(ctx, manuallyUpdated); err != nil {
		t.Fatal(err)
	}

	_, err = store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(202, 0))
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("apply error = %v, want revision conflict", err)
	}
	outlines, _ := store.ListOutlineNodes(ctx)
	if outlines[0].Title != "旧书名" {
		t.Fatalf("outline partially applied: %#v", outlines[0])
	}
	stored, _ := store.ChangeSet(ctx, changeSet.ID)
	conflicted := false
	for _, operation := range stored.Operations {
		if operation.ID == "operation-character" &&
			operation.Status == revisiondomain.ChangeSetConflicted {
			conflicted = true
		}
	}
	if stored.Status != revisiondomain.ChangeSetConflicted || !conflicted {
		t.Fatalf("stored conflict = %#v", stored)
	}
}

func TestStoreEditsAndDecidesStructuredOperationsIndividually(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	character := mustCharacter(t, "character-1", project.ID, "林默", worlddomain.RoleProtagonist)
	fact, err := canondomain.NewFact(
		"fact-1", project.ID, "旧事实", "plot", canondomain.OriginModel,
		canondomain.StatusAIInferred, .5, "", "", time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFact(ctx, fact); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChangeSet(
		"change-set-review", project.ID, "逐项审阅", "", "agent:reviewer",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "operation-character", Kind: revisiondomain.OperationUpdateCharacter, TargetID: character.ID,
				BeforeValue: `{"name":"林默","aliases":[],"role":"protagonist","summary":"","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
				AfterValue:  `{"name":"林默","aliases":[],"role":"protagonist","summary":"模型版本","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
			},
			{
				ID: "operation-fact", Kind: revisiondomain.OperationUpdateFact, TargetID: fact.ID,
				BeforeValue: `{"text":"旧事实","category":"plot","status":"ai_inferred","confidence":0.5,"validFromEventId":"","validUntilEventId":""}`,
				AfterValue:  `{"text":"不采用","category":"plot","status":"ai_inferred","confidence":0.5,"validFromEventId":"","validUntilEventId":""}`,
			},
		}, time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChangeSet(ctx, changeSet); err != nil {
		t.Fatal(err)
	}
	edited, err := store.EditChangeOperation(
		ctx, changeSet.ID, "operation-character",
		`{"name":"林默","aliases":["阿默"],"role":"protagonist","summary":"作者修订版","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
		time.Unix(201, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Operations[0].AfterValue == changeSet.Operations[0].AfterValue {
		t.Fatal("operation candidate was not edited")
	}
	result, err := store.ApplyChangeOperation(ctx, changeSet.ID, "operation-character", time.Unix(202, 0))
	if err != nil {
		t.Fatal(err)
	}
	if result.ChangeSet.Status != revisiondomain.ChangeSetPartiallyApplied {
		t.Fatalf("status after apply = %q", result.ChangeSet.Status)
	}
	if err := store.RejectChangeOperation(ctx, changeSet.ID, "operation-fact", time.Unix(203, 0)); err != nil {
		t.Fatal(err)
	}
	stored, _ := store.ChangeSet(ctx, changeSet.ID)
	if stored.Status != revisiondomain.ChangeSetApplied ||
		stored.Operations[0].Status != revisiondomain.ChangeSetApplied ||
		stored.Operations[1].Status != revisiondomain.ChangeSetRejected {
		t.Fatalf("final decisions = %#v", stored)
	}
	updatedCharacter, _ := store.Character(ctx, character.ID)
	if updatedCharacter.Summary != "作者修订版" {
		t.Fatalf("character = %#v", updatedCharacter)
	}
	updatedFact, _ := store.Fact(ctx, fact.ID)
	if updatedFact.Text != fact.Text {
		t.Fatalf("rejected fact changed: %#v", updatedFact)
	}
}
