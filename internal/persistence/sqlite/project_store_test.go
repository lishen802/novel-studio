package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestStoreReturnsProject(t *testing.T) {
	store, want := createTestStore(t)
	defer store.Close()

	got, err := store.Project(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("project = %#v, want %#v", got, want)
	}
}

func TestStoreCreatesAndListsChaptersInPositionOrder(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()

	second := mustChapter(t, "chapter-2", project.ID, "第二章", 2)
	first := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateChapter(context.Background(), first); err != nil {
		t.Fatal(err)
	}

	chapters, err := store.ListChapters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2", len(chapters))
	}
	if chapters[0].ID != first.ID || chapters[1].ID != second.ID {
		t.Fatalf("chapter order = [%q, %q]", chapters[0].ID, chapters[1].ID)
	}
}

func TestSaveDraftUpsertsWithoutChangingCommittedChapter(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveDraft(context.Background(), chapter.ID, "未提交草稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), chapter.ID, "更新后的未提交草稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}

	committed, err := store.Chapter(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Body != "" || committed.Revision != 1 {
		t.Fatalf("committed chapter = %#v", committed)
	}

	body, baseRevision, updatedAt, found, err := store.Draft(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || body != "更新后的未提交草稿" || baseRevision != 1 {
		t.Fatalf("draft = (%q, %d, %v)", body, baseRevision, found)
	}
	if updatedAt.IsZero() || updatedAt.Location() != time.UTC {
		t.Fatalf("draft updated_at = %v, want non-zero UTC time", updatedAt)
	}
}

func TestSaveDraftRejectsStaleRevisionWithoutReplacingRecoveryDraft(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), chapter.ID, "当前草稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}

	err := store.SaveDraft(context.Background(), chapter.ID, "过期窗口草稿", chapter.Revision+1)
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}

	body, baseRevision, _, found, err := store.Draft(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || body != "当前草稿" || baseRevision != chapter.Revision {
		t.Fatalf("draft = (%q, %d, %v)", body, baseRevision, found)
	}
}

func TestDeleteDraftKeepsCommittedChapter(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), chapter.ID, "待丢弃草稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteDraft(context.Background(), chapter.ID, chapter.Revision); err != nil {
		t.Fatal(err)
	}
	_, _, _, found, err := store.Draft(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("draft still exists after explicit deletion")
	}
	committed, err := store.Chapter(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if committed != chapter {
		t.Fatalf("committed chapter changed: %#v", committed)
	}
}

func TestDeleteDraftRejectsStaleRevisionAndKeepsNewerDraft(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), chapter.ID, "新版恢复草稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}

	err := store.DeleteDraft(context.Background(), chapter.ID, chapter.Revision-1)
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}
	body, baseRevision, _, found, err := store.Draft(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || body != "新版恢复草稿" || baseRevision != chapter.Revision {
		t.Fatalf("draft = (%q, %d, %v)", body, baseRevision, found)
	}
}

func TestCommitChapterArchivesPreviousRevisionAndClearsDraft(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	previous := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), previous); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), previous.ID, "第一版正文", previous.Revision); err != nil {
		t.Fatal(err)
	}

	updated, err := previous.Commit("第一版正文", previous.Revision, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitChapter(context.Background(), updated, previous); err != nil {
		t.Fatal(err)
	}

	got, err := store.Chapter(context.Background(), previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "第一版正文" || got.Revision != 2 {
		t.Fatalf("chapter = %#v", got)
	}
	_, _, _, found, err := store.Draft(context.Background(), previous.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("draft still exists after commit")
	}

	var archivedBody string
	var archivedRevision int64
	err = store.db.QueryRow(
		"SELECT body, revision FROM chapter_revisions WHERE chapter_id = ?",
		previous.ID,
	).Scan(&archivedBody, &archivedRevision)
	if err != nil {
		t.Fatal(err)
	}
	if archivedBody != previous.Body || archivedRevision != previous.Revision {
		t.Fatalf("archived revision = (%q, %d)", archivedBody, archivedRevision)
	}
}

func TestCommitChapterRollsBackAllWritesOnRevisionConflict(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	stale := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), stale); err != nil {
		t.Fatal(err)
	}

	firstUpdate, err := stale.Commit("窗口一的正文", stale.Revision, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitChapter(context.Background(), firstUpdate, stale); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDraft(context.Background(), stale.ID, "窗口二的草稿", firstUpdate.Revision); err != nil {
		t.Fatal(err)
	}

	staleUpdate, err := stale.Commit("窗口二的正文", stale.Revision, time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	err = store.CommitChapter(context.Background(), staleUpdate, stale)
	if !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}

	got, err := store.Chapter(context.Background(), stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != firstUpdate.Body || got.Revision != firstUpdate.Revision {
		t.Fatalf("chapter changed after conflict: %#v", got)
	}
	var revisionCount int
	if err := store.db.QueryRow(
		"SELECT COUNT(*) FROM chapter_revisions WHERE chapter_id = ?",
		stale.ID,
	).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if revisionCount != 1 {
		t.Fatalf("revision count = %d, want 1", revisionCount)
	}
	body, baseRevision, _, found, err := store.Draft(context.Background(), stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || body != "窗口二的草稿" || baseRevision != firstUpdate.Revision {
		t.Fatalf("draft = (%q, %d, %v)", body, baseRevision, found)
	}
}

func TestSaveDraftReturnsWhenAnotherWriterHoldsDatabaseLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	project, err := projectdomain.NewProject(
		"project-1",
		"长夜织境",
		"东方奇幻",
		"",
		time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).Create(context.Background(), path, project)
	if err != nil {
		t.Fatal(err)
	}
	store := opened.(*Store)
	defer store.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}

	locker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close()
	if _, err := locker.Exec("PRAGMA busy_timeout = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := locker.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer locker.Exec("ROLLBACK")
	if _, err := store.db.Exec("PRAGMA busy_timeout = 100"); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	err = store.SaveDraft(context.Background(), chapter.ID, "不会写入的草稿", chapter.Revision)
	if err == nil {
		t.Fatal("SaveDraft succeeded while another writer held the lock")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("locked write took %s, want under 1s", elapsed)
	}
	_, _, _, found, err := store.Draft(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("draft was partially written while database was locked")
	}
}

func TestOpenRejectsPackageWithoutExactlyOneProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = (Factory{}).Open(context.Background(), path)
	if !errors.Is(err, ErrProjectCount) {
		t.Fatalf("error = %v, want ErrProjectCount", err)
	}
}

func createTestStore(t *testing.T) (*Store, projectdomain.Project) {
	t.Helper()
	project, err := projectdomain.NewProject(
		"project-1",
		"长夜织境",
		"东方奇幻",
		"简介",
		time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).Create(
		context.Background(),
		filepath.Join(t.TempDir(), "project.db"),
		project,
	)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := opened.(*Store)
	if !ok {
		t.Fatalf("store type = %T, want *Store", opened)
	}
	return store, project
}

func mustChapter(t *testing.T, id, projectID, title string, position int) projectdomain.Chapter {
	t.Helper()
	chapter, err := projectdomain.NewChapter(id, projectID, title, position, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	return chapter
}
