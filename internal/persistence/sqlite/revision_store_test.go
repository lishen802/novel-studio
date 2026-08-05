package sqlite

import (
	"context"
	"database/sql"
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

func TestStoreAppliesExtractionCreateOperationsWithEvidence(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	// The foreshadowing hook's planted_chapter_id must reference an existing
	// chapter, and the relationship's source/target must reference characters
	// created earlier in the same change set.
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter.Body = "林默走入钟楼。"
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}

	evidence := []revisiondomain.Evidence{{
		ChapterID: chapter.ID, ChapterTitle: "第一章",
		StartLine: 1, EndLine: 2, Excerpt: "林默走入钟楼。",
	}}
	changeSet, err := revisiondomain.NewChangeSet(
		"change-set-extraction", project.ID, "抽取候选", "分章抽取后合并",
		"extraction-task:task-1",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "operation-outline", Kind: revisiondomain.OperationCreateOutlineNode, TargetID: "outline-1",
				AfterValue: `{"level":"book","title":"雾城钟声","summary":"追查钟楼异响","goal":"找到失踪者","conflict":"城卫阻挠","outcome":"发现密道","position":1}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-character-1", Kind: revisiondomain.OperationCreateCharacter, TargetID: "character-1",
				AfterValue: `{"name":"林默","aliases":["阿默"],"role":"protagonist","summary":"失忆的巡夜人","motivation":"找回记忆","fear":"再次失去同伴","secret":"曾进入钟楼","arc":"从逃避到承担","status":"active"}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-character-2", Kind: revisiondomain.OperationCreateCharacter, TargetID: "character-2",
				AfterValue: `{"name":"苏雨","aliases":[],"role":"supporting","summary":"钟楼守门人","motivation":"守护秘密","fear":"真相曝光","secret":"知道密道","arc":"从隐瞒到坦白","status":"active"}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-relationship", Kind: revisiondomain.OperationCreateRelationship, TargetID: "relationship-1",
				AfterValue: `{"sourceCharacterId":"character-1","targetCharacterId":"character-2","kind":"mentor","label":"师徒","strength":60,"publicState":"尊敬","trueState":"猜疑"}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-world-rule", Kind: revisiondomain.OperationCreateWorldRule, TargetID: "world-rule-1",
				AfterValue: `{"name":"钟楼密道","description":"钟楼地下通往城外的密道","scopeType":"project"}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-story-event", Kind: revisiondomain.OperationCreateStoryEvent, TargetID: "story-event-1",
				AfterValue: `{"title":"钟楼初探","summary":"林默首次进入钟楼","cause":"追寻线索","result":"发现密道入口","storyTime":"第一夜","storyOrder":1,"narrativeOrder":1,"status":"occurred"}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-fact", Kind: revisiondomain.OperationCreateFact, TargetID: "fact-1",
				AfterValue: `{"text":"钟楼在城西","category":"location","status":"ai_inferred","confidence":0.8,"validFromEventId":"","validUntilEventId":""}`,
				Evidence:  evidence,
			},
			{
				ID: "operation-foreshadowing", Kind: revisiondomain.OperationCreateForeshadowingHook, TargetID: "foreshadowing-1",
				AfterValue: `{"title":"密道尽头","setup":"钟楼地下有未被探索的通道","plantedChapterId":"` + chapter.ID + `"}`,
				Evidence:  evidence,
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
		t.Fatalf("apply extraction change set: %v", err)
	}
	if result.ChangeSet.Status != revisiondomain.ChangeSetApplied {
		t.Fatalf("change set status = %q", result.ChangeSet.Status)
	}

	outlines, _ := store.ListOutlineNodes(ctx)
	if len(outlines) != 1 || outlines[0].Title != "雾城钟声" || outlines[0].Outcome != "发现密道" {
		t.Fatalf("outline = %#v", outlines)
	}
	character1, _ := store.Character(ctx, "character-1")
	if character1.Name != "林默" || len(character1.Aliases) != 1 || character1.Aliases[0] != "阿默" {
		t.Fatalf("character-1 = %#v", character1)
	}
	character2, _ := store.Character(ctx, "character-2")
	if character2.Name != "苏雨" || character2.Role != worlddomain.RoleSupporting {
		t.Fatalf("character-2 = %#v", character2)
	}
	relationships, _ := store.ListRelationships(ctx)
	if len(relationships) != 1 ||
		relationships[0].SourceCharacterID != "character-1" ||
		relationships[0].TargetCharacterID != "character-2" ||
		relationships[0].Kind != worlddomain.RelationshipMentor ||
		relationships[0].Strength != 60 {
		t.Fatalf("relationships = %#v", relationships)
	}
	rules, _ := store.ListWorldRules(ctx)
	if len(rules) != 1 || rules[0].Name != "钟楼密道" {
		t.Fatalf("world rules = %#v", rules)
	}
	events, _ := store.ListStoryEvents(ctx)
	if len(events) != 1 || events[0].Title != "钟楼初探" || events[0].StoryOrder != 1 {
		t.Fatalf("story events = %#v", events)
	}
	fact, _ := store.Fact(ctx, "fact-1")
	if fact.Text != "钟楼在城西" || fact.Origin != canondomain.OriginModel ||
		fact.Status != canondomain.StatusAIInferred || fact.Confidence != 0.8 {
		t.Fatalf("fact = %#v", fact)
	}
	hooks, _ := store.ListForeshadowingHooks(ctx)
	if len(hooks) != 1 || hooks[0].Title != "密道尽头" ||
		hooks[0].PlantedChapterID != chapter.ID {
		t.Fatalf("foreshadowing hooks = %#v", hooks)
	}

	stored, _ := store.ChangeSet(ctx, changeSet.ID)
	if len(stored.Operations) != 8 {
		t.Fatalf("stored operations = %d, want 8", len(stored.Operations))
	}
	for _, operation := range stored.Operations {
		if len(operation.Evidence) != 1 ||
			operation.Evidence[0].ChapterID != chapter.ID ||
			operation.Evidence[0].Excerpt != "林默走入钟楼。" {
			t.Fatalf("operation %s evidence = %#v", operation.ID, operation.Evidence)
		}
		if operation.BeforeValue != "{}" {
			t.Fatalf("operation %s before_value = %q, want {}", operation.ID, operation.BeforeValue)
		}
	}
}

func TestStoreDecidesExtractionCreateOperationsIndividually(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	changeSet, err := revisiondomain.NewChangeSet(
		"change-set-extraction-review", project.ID, "逐项确认", "",
		"extraction-task:task-1",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "operation-character", Kind: revisiondomain.OperationCreateCharacter, TargetID: "character-1",
				AfterValue: `{"name":"林默","aliases":[],"role":"protagonist","summary":"模型版本","motivation":"","fear":"","secret":"","arc":"","status":"active"}`,
			},
			{
				ID: "operation-fact", Kind: revisiondomain.OperationCreateFact, TargetID: "fact-1",
				AfterValue: `{"text":"钟楼在城西","category":"location","status":"ai_inferred","confidence":0.5,"validFromEventId":"","validUntilEventId":""}`,
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
		t.Fatal("create operation was not edited")
	}
	if len(edited.Operations[0].Evidence) != 0 {
		t.Fatalf("editing should not drop evidence, got %#v", edited.Operations[0].Evidence)
	}

	if _, err := store.ApplyChangeOperation(ctx, changeSet.ID, "operation-character", time.Unix(202, 0)); err != nil {
		t.Fatal(err)
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
	created, _ := store.Character(ctx, "character-1")
	if created.Summary != "作者修订版" || len(created.Aliases) != 1 {
		t.Fatalf("created character = %#v", created)
	}
	if _, err := store.Fact(ctx, "fact-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected fact should not exist, got err = %v", err)
	}
}
