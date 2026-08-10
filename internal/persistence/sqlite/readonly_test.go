package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestOpenReadOnlyReadsCurrentSchemaWithoutModifyingProtectedProjectFiles(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	path := filepath.Join(directory, "project #.db")
	project, err := newReadOnlyTestProject()
	if err != nil {
		t.Fatal(err)
	}
	writable, err := (Factory{}).Create(ctx, path, project)
	if err != nil {
		t.Fatal(err)
	}
	defer writable.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := writable.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "recovery"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "recovery", "sentinel"), []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := collectReadOnlyProtectedFiles(t, directory, path)

	opened, err := (Factory{}).OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	readOnly, ok := opened.(*Store)
	if !ok {
		t.Fatalf("store type = %T, want *Store", opened)
	}
	defer readOnly.Close()
	gotProject, err := readOnly.Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gotProject.ID != project.ID {
		t.Fatalf("project ID = %q, want %q", gotProject.ID, project.ID)
	}
	gotChapter, err := readOnly.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotChapter != chapter {
		t.Fatalf("chapter = %#v, want %#v", gotChapter, chapter)
	}
	after := collectReadOnlyProtectedFiles(t, directory, path)
	if !sameFileHashes(before.hashes, after.hashes) {
		t.Fatalf("protected project files changed during read-only open\nbefore: %#v\nafter: %#v", before.hashes, after.hashes)
	}
	if err := assertReadOnlyWAL(t, before, after); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyStoreRejectsWritesAndCanCreateConsistentBackup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	project, err := newReadOnlyTestProject()
	if err != nil {
		t.Fatal(err)
	}
	writable, err := (Factory{}).Create(ctx, path, project)
	if err != nil {
		t.Fatal(err)
	}
	defer writable.Close()
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := writable.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}

	opened, err := (Factory{}).OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	readOnly := opened.(*Store)
	defer readOnly.Close()

	err = readOnly.SaveDraft(ctx, chapter.ID, "不能保存", chapter.Revision)
	if !errors.Is(err, ErrDatabasePermission) {
		t.Fatalf("SaveDraft error = %v, want ErrDatabasePermission", err)
	}
	if _, _, _, found, err := writable.Draft(ctx, chapter.ID); err != nil || found {
		t.Fatalf("draft after read-only write = (found %v, error %v)", found, err)
	}

	target := filepath.Join(t.TempDir(), "snapshot.sqlite")
	if err := readOnly.Backup(ctx, target); err != nil {
		t.Fatalf("Backup from read-only store: %v", err)
	}
	backup, err := (Factory{}).Open(ctx, target)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	got, err := backup.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != chapter {
		t.Fatalf("backup chapter = %#v, want %#v", got, chapter)
	}
}

func TestReadOnlyBackupEscapesTargetFileURI(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	project, err := newReadOnlyTestProject()
	if err != nil {
		t.Fatal(err)
	}
	writable, err := (Factory{}).Create(ctx, path, project)
	if err != nil {
		t.Fatal(err)
	}
	defer writable.Close()
	chapter := mustChapter(t, "chapter-target-uri", project.ID, "目标 URI", 1)
	if err := writable.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := opened.(*Store)
	defer readOnly.Close()

	targetDirectory := t.TempDir()
	target := filepath.Join(targetDirectory, "snapshot ?#.sqlite")
	truncatedSibling := filepath.Join(targetDirectory, "snapshot ")
	const sentinel = "do not replace"
	if err := os.WriteFile(truncatedSibling, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := readOnly.Backup(ctx, target); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if got, err := os.ReadFile(truncatedSibling); err != nil || string(got) != sentinel {
		t.Fatalf("truncated sibling changed: %q, error %v", got, err)
	}
	backup, err := (Factory{}).OpenReadOnly(ctx, target)
	if err != nil {
		t.Fatalf("open exact target: %v", err)
	}
	defer backup.Close()
	got, err := backup.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != chapter {
		t.Fatalf("exact target chapter = %#v, want %#v", got, chapter)
	}
}

func TestReserveBackupTargetRejectsFileCreatedAfterPreflight(t *testing.T) {
	target := filepath.Join(t.TempDir(), "snapshot.sqlite")
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("preflight target error = %v, want not exist", err)
	}
	if err := os.WriteFile(target, []byte("newly created"), 0o600); err != nil {
		t.Fatal(err)
	}

	cleanup, err := reserveBackupTarget(target)
	if cleanup != nil {
		t.Fatal("reserve returned cleanup for an existing target")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("reserve error = %v, want already exists", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "newly created" {
		t.Fatalf("target changed after failed reservation: %q, error %v", got, err)
	}
}

func TestSQLiteFileURIEscapesQuestionMarkInPath(t *testing.T) {
	ctx := context.Background()
	target := filepath.Join(t.TempDir(), "sqlite ?.db")
	uri, err := sqliteFileURI(target, "")
	if err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE probe(value TEXT)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := probeExistingDatabase(target); err != nil {
		t.Fatalf("escaped URI did not create exact target: %v", err)
	}
}

func TestOpenReadOnlyRetainsFutureSchemaErrorWithoutChanges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO goose_db_version(version_id, is_applied, tstamp) VALUES (999, 1, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := collectReadOnlyProtectedFiles(t, filepath.Dir(path), path)

	_, err = (Factory{}).OpenReadOnly(ctx, path)
	if !errors.Is(err, ErrDatabaseTooNew) {
		t.Fatalf("OpenReadOnly error = %v, want ErrDatabaseTooNew", err)
	}
	after := collectReadOnlyProtectedFiles(t, filepath.Dir(path), path)
	if !sameFileHashes(before.hashes, after.hashes) {
		t.Fatalf("future schema protected project files changed during read-only open\nbefore: %#v\nafter: %#v", before.hashes, after.hashes)
	}
	if err := assertReadOnlyWAL(t, before, after); err != nil {
		t.Fatal(err)
	}
}

func TestOpenReadOnlySeesCommittedChapterInActiveWAL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	project, err := newReadOnlyTestProject()
	if err != nil {
		t.Fatal(err)
	}
	writable, err := (Factory{}).Create(ctx, path, project)
	if err != nil {
		t.Fatal(err)
	}
	defer writable.Close()
	chapter := mustChapter(t, "chapter-in-wal", project.ID, "WAL 中的章节", 1)
	if err := writable.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + "-wal"); err != nil {
		t.Fatalf("active WAL missing: %v", err)
	}

	opened, err := (Factory{}).OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer opened.Close()
	got, err := opened.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != chapter {
		t.Fatalf("chapter from active WAL = %#v, want %#v", got, chapter)
	}
}

func newReadOnlyTestProject() (project projectdomain.Project, err error) {
	return projectdomain.NewProject("project-1", "长夜织境", "东方奇幻", "简介", time.Unix(100, 0))
}

func collectFileHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("collect file hashes: %v", err)
	}
	return result
}

func sameFileHashes(first, second map[string]string) bool {
	if len(first) != len(second) {
		return false
	}
	for path, hash := range first {
		if second[path] != hash {
			return false
		}
	}
	return true
}

type readOnlyProtectedFiles struct {
	hashes     map[string]string
	walPath    string
	walExisted bool
	walHash    string
}

func collectReadOnlyProtectedFiles(t *testing.T, root, databasePath string) readOnlyProtectedFiles {
	t.Helper()
	walPath := databasePath + "-wal"
	_, err := os.Stat(walPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inspect WAL: %v", err)
	}
	hashes := collectFileHashes(t, root)
	walName := filepath.Base(walPath)
	result := readOnlyProtectedFiles{
		hashes:     hashes,
		walPath:    walPath,
		walExisted: err == nil,
	}
	delete(result.hashes, filepath.Base(databasePath)+"-shm")
	result.walHash = result.hashes[walName]
	delete(result.hashes, walName)
	return result
}

func assertReadOnlyWAL(t *testing.T, before, after readOnlyProtectedFiles) error {
	t.Helper()
	if before.walExisted {
		if !after.walExisted {
			return fmt.Errorf("read-only open removed active WAL")
		}
		if after.walHash != before.walHash {
			return fmt.Errorf("read-only open changed active WAL")
		}
		return nil
	}
	if !after.walExisted {
		return nil
	}
	info, err := os.Stat(before.walPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect newly created WAL: %w", err)
	}
	if info.Size() != 0 {
		return fmt.Errorf("read-only open created non-empty WAL (%d bytes)", info.Size())
	}
	return nil
}
