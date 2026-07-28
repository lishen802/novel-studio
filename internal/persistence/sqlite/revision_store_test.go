package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
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
	if updated.Body != "雨夜里，警钟提前响起。" || updated.Revision != 2 {
		t.Fatalf("updated chapter = %#v", updated)
	}
	reapplied, err := store.ApplyChangeSet(ctx, changeSet.ID, time.Unix(202, 0))
	if err != nil {
		t.Fatalf("reapply change set: %v", err)
	}
	if reapplied.Revision != updated.Revision {
		t.Fatalf("reapplied revision = %d, want %d", reapplied.Revision, updated.Revision)
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
