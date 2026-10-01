package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestChapterRevisionHistoryIncludesCurrentAndArchivedBodies(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()

	first, err := chapter.Commit("第一版正文", chapter.Revision, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitChapter(ctx, first, chapter); err != nil {
		t.Fatal(err)
	}
	second, err := first.Commit("第二版正文", first.Revision, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitChapter(ctx, second, first); err != nil {
		t.Fatal(err)
	}

	history, err := store.ListChapterRevisions(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}
	if history[0].Revision != 3 || history[0].Body != "第二版正文" {
		t.Fatalf("current history item = %#v", history[0])
	}
	if history[1].Revision != 2 || history[1].Body != "第一版正文" {
		t.Fatalf("second history item = %#v", history[1])
	}
	if history[2].Revision != 1 || history[2].Source != revisiondomain.RevisionInitial {
		t.Fatalf("initial history item = %#v", history[2])
	}
	for _, item := range history {
		if item.ChapterID != chapter.ID || item.Title != chapter.Title {
			t.Fatalf("history item belongs to wrong chapter: %#v, project = %s", item, project.ID)
		}
	}
}

func TestRestoreChapterRevisionCreatesNewRevisionAndPreservesHistory(t *testing.T) {
	ctx := context.Background()
	store, _, chapter := createStoreWithChapter(t)
	defer store.Close()

	first, _ := chapter.Commit("第一版正文", chapter.Revision, time.Unix(200, 0))
	if err := store.CommitChapter(ctx, first, chapter); err != nil {
		t.Fatal(err)
	}
	second, _ := first.Commit("第二版正文", first.Revision, time.Unix(300, 0))
	if err := store.CommitChapter(ctx, second, first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(ctx, chapter.ID, second.Body, second.Revision); err != nil {
		t.Fatal(err)
	}

	restored, err := store.RestoreChapterRevision(
		ctx,
		chapter.ID,
		1,
		second.Revision,
		time.Unix(400, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 4 || restored.Body != "" {
		t.Fatalf("restored chapter = %#v", restored)
	}
	if _, _, _, found, err := store.Draft(ctx, chapter.ID); err != nil || found {
		t.Fatalf("draft after restore = (found %v, error %v)", found, err)
	}

	history, err := store.ListChapterRevisions(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if history[0].Revision != 4 || history[0].Source != revisiondomain.RevisionRestore {
		t.Fatalf("restored history item = %#v", history[0])
	}
	if history[1].Revision != 3 || history[1].Body != "第二版正文" {
		t.Fatalf("pre-restore history item = %#v", history[1])
	}
}

func TestRestoreChapterRevisionRejectsStaleEditorWithoutWrites(t *testing.T) {
	ctx := context.Background()
	store, _, chapter := createStoreWithChapter(t)
	defer store.Close()

	updated, _ := chapter.Commit("当前正文", chapter.Revision, time.Unix(200, 0))
	if err := store.CommitChapter(ctx, updated, chapter); err != nil {
		t.Fatal(err)
	}

	_, err := store.RestoreChapterRevision(
		ctx,
		chapter.ID,
		1,
		chapter.Revision,
		time.Unix(300, 0),
	)
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("restore error = %v, want revision conflict", err)
	}
	current, err := store.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current != updated {
		t.Fatalf("chapter changed after conflict: %#v", current)
	}
}

func TestBackupCreatesConsistentOpenableDatabase(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	updated, _ := chapter.Commit("快照正文", chapter.Revision, time.Unix(200, 0))
	if err := store.CommitChapter(ctx, updated, chapter); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "snapshot.sqlite")
	if err := store.Backup(ctx, target); err != nil {
		t.Fatal(err)
	}
	backup, err := (Factory{}).Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	gotProject, err := backup.Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotChapter, err := backup.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotProject.ID != project.ID || gotChapter.Body != "快照正文" || gotChapter.Revision != 2 {
		t.Fatalf("backup state = (%#v, %#v)", gotProject, gotChapter)
	}
}

func createStoreWithChapter(t *testing.T) (*Store, projectdomain.Project, projectdomain.Chapter) {
	t.Helper()
	store, project := createTestStore(t)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store, project, chapter
}
