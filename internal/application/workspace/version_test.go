package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestChapterHistoryMarksCurrentRevisionAndRestoresAsNewRevision(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	chapter.Revision = 3
	chapter.Body = "当前正文"
	store.chapters[chapter.ID] = chapter
	store.chapterRevisions[chapter.ID] = []revisiondomain.ChapterRevision{
		{
			ChapterID: chapter.ID,
			Revision:  3,
			Title:     chapter.Title,
			Body:      chapter.Body,
			Source:    revisiondomain.RevisionAuthor,
			CreatedAt: time.Unix(300, 0),
		},
		{
			ChapterID: chapter.ID,
			Revision:  1,
			Title:     chapter.Title,
			Body:      "初稿",
			Source:    revisiondomain.RevisionInitial,
			CreatedAt: time.Unix(100, 0),
		},
	}
	service := openVersionTestService(
		t,
		store,
		&fakeSnapshotArchive{},
		&sequenceIDs{},
	)

	history, err := service.ChapterHistory(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || !history[0].IsCurrent || history[1].IsCurrent {
		t.Fatalf("history = %#v", history)
	}

	restored, err := service.RestoreChapterRevision(
		context.Background(),
		chapter.ID,
		1,
		chapter.Revision,
	)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 4 || restored.Body != "初稿" {
		t.Fatalf("restored chapter = %#v", restored)
	}
}

func TestCreateNamedSnapshotBacksUpBeforeRecordingMetadata(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	archive := &fakeSnapshotArchive{}
	service := openVersionTestService(
		t,
		store,
		archive,
		&sequenceIDs{ids: []string{"snapshot-1"}},
	)

	created, err := service.CreateSnapshot(context.Background(), "  第一幕定稿  ")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := "/books/project.zjproject/snapshots/snapshot-1.sqlite"
	if store.backupPath != wantPath {
		t.Fatalf("backup path = %q, want %q", store.backupPath, wantPath)
	}
	if created.Name != "第一幕定稿" || created.Kind != "named" {
		t.Fatalf("created snapshot = %#v", created)
	}
	if len(archive.recorded) != 1 || archive.recorded[0].ID != created.ID {
		t.Fatalf("recorded snapshots = %#v", archive.recorded)
	}
}

func TestSnapshotsSortByTimeInsteadOfFormattedTimestamp(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	exactSecond, _ := revisiondomain.NewSnapshot(
		"snapshot-exact",
		project.ID,
		"整秒快照",
		revisiondomain.SnapshotNamed,
		time.Unix(100, 0),
	)
	laterFraction, _ := revisiondomain.NewSnapshot(
		"snapshot-later",
		project.ID,
		"稍后快照",
		revisiondomain.SnapshotNamed,
		time.Unix(100, 900_000_000),
	)
	service := openVersionTestService(
		t,
		store,
		&fakeSnapshotArchive{
			snapshots: []revisiondomain.Snapshot{exactSecond, laterFraction},
		},
		&sequenceIDs{},
	)

	snapshots, err := service.Snapshots(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 || snapshots[0].ID != laterFraction.ID {
		t.Fatalf("snapshots = %#v", snapshots)
	}
}

func TestRestoreSnapshotCreatesRecoveryPointAndReopensRestoredDatabase(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	currentStore := newFakeStore(project)
	currentChapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	currentChapter.Body = "当前正文"
	currentChapter.Revision = 3
	currentStore.chapters[currentChapter.ID] = currentChapter

	restoredStore := newFakeStore(project)
	restoredChapter := currentChapter
	restoredChapter.Body = "快照正文"
	restoredChapter.Revision = 2
	restoredStore.chapters[restoredChapter.ID] = restoredChapter

	target, _ := revisiondomain.NewSnapshot(
		"snapshot-target",
		project.ID,
		"第一幕定稿",
		revisiondomain.SnapshotNamed,
		time.Unix(150, 0),
	)
	archive := &fakeSnapshotArchive{
		snapshots: []revisiondomain.Snapshot{target},
	}
	factory := &fakeStoreFactory{
		openStores: []ports.ProjectStore{currentStore, restoredStore},
	}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		factory,
		&fakeLicense{},
		fixedClock{now: time.Unix(400, 0)},
		&sequenceIDs{ids: []string{"snapshot-recovery"}},
		archive,
	)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}

	result, err := service.RestoreSnapshot(context.Background(), target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !archive.beganRestore || !archive.committedRestore || archive.rolledBackRestore {
		t.Fatalf("restore lifecycle = begin:%v commit:%v rollback:%v", archive.beganRestore, archive.committedRestore, archive.rolledBackRestore)
	}
	if currentStore.closeCalls != 1 {
		t.Fatalf("current store close calls = %d", currentStore.closeCalls)
	}
	if len(archive.recorded) != 1 || archive.recorded[0].Kind != revisiondomain.SnapshotRecovery {
		t.Fatalf("recovery snapshots = %#v", archive.recorded)
	}
	if len(result.Chapters) != 1 || result.Chapters[0].Body != "快照正文" {
		t.Fatalf("restore result = %#v", result)
	}
}

func TestRestoreSnapshotRollsBackForeignProjectDatabase(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	currentStore := newFakeStore(project)
	foreignStore := newFakeStore(mustProject(t, "project-2", "其他作品"))
	reopenedOriginal := newFakeStore(project)
	target, _ := revisiondomain.NewSnapshot(
		"snapshot-target",
		project.ID,
		"安全点",
		revisiondomain.SnapshotNamed,
		time.Unix(150, 0),
	)
	archive := &fakeSnapshotArchive{snapshots: []revisiondomain.Snapshot{target}}
	factory := &fakeStoreFactory{
		openStores: []ports.ProjectStore{currentStore, foreignStore, reopenedOriginal},
	}
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		factory,
		&fakeLicense{},
		fixedClock{now: time.Unix(400, 0)},
		&sequenceIDs{ids: []string{"snapshot-recovery"}},
		archive,
	)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}

	_, err := service.RestoreSnapshot(context.Background(), target.ID)
	if err == nil {
		t.Fatal("restored foreign project database")
	}
	if !archive.rolledBackRestore || archive.committedRestore {
		t.Fatalf("restore lifecycle = commit:%v rollback:%v", archive.committedRestore, archive.rolledBackRestore)
	}
	chapters, listErr := service.ListChapters(context.Background())
	if listErr != nil || chapters == nil {
		t.Fatalf("original workspace was not reopened: (%#v, %v)", chapters, listErr)
	}
}

func openVersionTestService(
	t *testing.T,
	store *fakeStore,
	archive *fakeSnapshotArchive,
	ids *sequenceIDs,
) *Service {
	t.Helper()
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		&fakeStoreFactory{openStore: store},
		&fakeLicense{},
		fixedClock{now: time.Unix(400, 0)},
		ids,
		archive,
	)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}
	return service
}

type fakeSnapshotArchive struct {
	snapshots          []revisiondomain.Snapshot
	recorded           []revisiondomain.Snapshot
	beganRestore       bool
	committedRestore   bool
	rolledBackRestore  bool
	rollbackErr        error
	recoveredDatabases []string
	recoveryRequired   bool
	events             *[]string
}

func (f *fakeSnapshotArchive) Prepare(directory string, id string) (string, error) {
	return directory + "/" + id + ".sqlite", nil
}

func (f *fakeSnapshotArchive) Record(_ string, snapshot revisiondomain.Snapshot) error {
	f.recorded = append(f.recorded, snapshot)
	f.snapshots = append(f.snapshots, snapshot)
	return nil
}

func (f *fakeSnapshotArchive) List(string) ([]revisiondomain.Snapshot, error) {
	return append([]revisiondomain.Snapshot(nil), f.snapshots...), nil
}

func (f *fakeSnapshotArchive) Find(directory string, id string) (revisiondomain.Snapshot, string, error) {
	for _, snapshot := range f.snapshots {
		if snapshot.ID == id {
			return snapshot, directory + "/" + id + ".sqlite", nil
		}
	}
	return revisiondomain.Snapshot{}, "", errors.New("snapshot missing")
}

func (f *fakeSnapshotArchive) Discard(string, string) error { return nil }

func (f *fakeSnapshotArchive) BeginRestore(string, string) error {
	f.beganRestore = true
	return nil
}

func (f *fakeSnapshotArchive) CommitRestore(string) error {
	f.committedRestore = true
	return nil
}

func (f *fakeSnapshotArchive) RollbackRestore(string) error {
	f.rolledBackRestore = true
	return f.rollbackErr
}

func (f *fakeSnapshotArchive) RecoverInterruptedRestore(databasePath string) error {
	appendEvent(f.events, "recover-snapshot")
	f.recoveredDatabases = append(f.recoveredDatabases, databasePath)
	return nil
}

func (f *fakeSnapshotArchive) RecoveryRequired(string) (bool, error) {
	return f.recoveryRequired, nil
}
