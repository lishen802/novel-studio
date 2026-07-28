package snapshotfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestArchiveRecordsListsAndFindsImmutableSnapshots(t *testing.T) {
	root := t.TempDir()
	archive := Archive{}
	older, _ := revisiondomain.NewSnapshot(
		"snapshot-1", "project-1", "第一幕", revisiondomain.SnapshotNamed, time.Unix(100, 0),
	)
	newer, _ := revisiondomain.NewSnapshot(
		"snapshot-2", "project-1", "第二幕", revisiondomain.SnapshotNamed, time.Unix(200, 0),
	)

	olderPath, err := archive.Prepare(root, older.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(olderPath, []byte("older"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := archive.Record(root, older); err != nil {
		t.Fatal(err)
	}
	newerPath, err := archive.Prepare(root, newer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newerPath, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := archive.Record(root, newer); err != nil {
		t.Fatal(err)
	}

	listed, err := archive.List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0] != newer || listed[1] != older {
		t.Fatalf("listed snapshots = %#v", listed)
	}
	found, path, err := archive.Find(root, older.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found != older || path != olderPath {
		t.Fatalf("found snapshot = (%#v, %q)", found, path)
	}
}

func TestArchiveRejectsUnsafeSnapshotID(t *testing.T) {
	_, err := (Archive{}).Prepare(t.TempDir(), "../outside")
	if err == nil {
		t.Fatal("Prepare accepted unsafe snapshot id")
	}
}

func TestDatabaseReplacementCanCommitOrRollback(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	snapshotPath := filepath.Join(root, "snapshot.sqlite")
	if err := os.WriteFile(databasePath, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := Archive{}

	if err := archive.BeginRestore(databasePath, snapshotPath); err != nil {
		t.Fatal(err)
	}
	assertFileBody(t, databasePath, "snapshot")
	if err := archive.RollbackRestore(databasePath); err != nil {
		t.Fatal(err)
	}
	assertFileBody(t, databasePath, "current")

	if err := archive.BeginRestore(databasePath, snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := archive.CommitRestore(databasePath); err != nil {
		t.Fatal(err)
	}
	assertFileBody(t, databasePath, "snapshot")
	if _, err := os.Stat(databasePath + ".restore-backup"); !os.IsNotExist(err) {
		t.Fatalf("restore backup still exists: %v", err)
	}
}

func TestRecoverInterruptedRestorePrefersOriginalDatabase(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	if err := os.WriteFile(databasePath, []byte("partially-restored"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(databasePath+".restore-backup", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := (Archive{}).RecoverInterruptedRestore(databasePath); err != nil {
		t.Fatal(err)
	}
	assertFileBody(t, databasePath, "original")
}

func assertFileBody(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("%s body = %q, want %q", path, body, want)
	}
}
