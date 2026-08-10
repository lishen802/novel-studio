package projectfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewLayoutSanitizesCrossPlatformName(t *testing.T) {
	parent := t.TempDir()

	layout, err := NewLayout(parent, "  My / Novel  ")
	if err != nil {
		t.Fatal(err)
	}

	wantRoot := filepath.Join(parent, "My - Novel.zjproject")
	if layout.Root != wantRoot {
		t.Fatalf("Root = %q, want %q", layout.Root, wantRoot)
	}
	if layout.Database != filepath.Join(wantRoot, "project.db") {
		t.Fatalf("Database = %q", layout.Database)
	}
	if layout.Attachments != filepath.Join(wantRoot, "attachments") {
		t.Fatalf("Attachments = %q", layout.Attachments)
	}
	if layout.Exports != filepath.Join(wantRoot, "exports") {
		t.Fatalf("Exports = %q", layout.Exports)
	}
	if layout.Imports != filepath.Join(wantRoot, "imports") {
		t.Fatalf("Imports = %q", layout.Imports)
	}
	if layout.Snapshots != filepath.Join(wantRoot, "snapshots") {
		t.Fatalf("Snapshots = %q", layout.Snapshots)
	}
	if layout.Recovery != filepath.Join(wantRoot, "recovery") {
		t.Fatalf("Recovery = %q", layout.Recovery)
	}
	if layout.Branches != filepath.Join(wantRoot, "branches") {
		t.Fatalf("Branches = %q", layout.Branches)
	}
}

func TestNewLayoutMakesWindowsReservedNamePortable(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "CON")
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(layout.Root) != "CON-project.zjproject" {
		t.Fatalf("package name = %q", filepath.Base(layout.Root))
	}
}

func TestLayoutCreateCreatesRequiredDirectories(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}

	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}

	for _, directory := range []string{
		layout.Root,
		layout.Attachments,
		layout.Exports,
		layout.Imports,
		layout.Recovery,
		layout.Snapshots,
		layout.Branches,
	} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("Stat(%q): %v", directory, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", directory)
		}
	}

	if _, err := os.Stat(layout.Database); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("database should be created by persistence, Stat error = %v", err)
	}
}

func TestLayoutCreateDoesNotOverwriteExistingPackage(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}

	err = layout.Create()
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("error = %v, want fs.ErrExist", err)
	}
}

func TestLayoutCreateIncludesRecoveryDirectory(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}

	if layout.Recovery != filepath.Join(layout.Root, "recovery") {
		t.Fatalf("Recovery = %q, want %q", layout.Recovery, filepath.Join(layout.Root, "recovery"))
	}
	info, err := os.Stat(layout.Recovery)
	if err != nil {
		t.Fatalf("Stat recovery: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("recovery mode = %v, want directory", info.Mode())
	}
}

func TestInspectLayoutDoesNotCreateMissingCompatibilityDirectories(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{layout.Imports, layout.Recovery} {
		if err := os.Remove(directory); err != nil {
			t.Fatal(err)
		}
	}

	opened, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Imports != layout.Imports || opened.Recovery != layout.Recovery {
		t.Fatalf("opened = %#v, want paths from %#v", opened, layout)
	}
	for _, directory := range []string{layout.Imports, layout.Recovery} {
		if _, err := os.Stat(directory); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("InspectLayout created %q: %v", directory, err)
		}
	}
}

func TestPrepareEditableLayoutCreatesMissingRecoveryDirectoryForLegacyPackage(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(layout.Recovery); err != nil {
		t.Fatal(err)
	}

	opened, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(opened); err != nil {
		t.Fatal(err)
	}
	if opened.Recovery != layout.Recovery {
		t.Fatalf("opened.Recovery = %q, want %q", opened.Recovery, layout.Recovery)
	}
	info, err := os.Stat(opened.Recovery)
	if err != nil {
		t.Fatalf("Stat recovery: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("recovery mode = %v, want directory", info.Mode())
	}
}

func TestPrepareEditableLayoutRecoveryDirectoryFailureDoesNotTouchDatabase(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "损坏的旧项目")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	// Simulate a real database so the test can prove OpenLayout leaves it
	// alone when recovery directory creation fails.
	wantDatabase := []byte("SQLite format 3\x00placeholder for legacy project.db")
	if err := os.WriteFile(layout.Database, wantDatabase, 0o600); err != nil {
		t.Fatal(err)
	}
	// Block recovery/ creation by replacing the directory Create made with a
	// regular file at the same path.
	if err := os.Remove(layout.Recovery); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.Recovery, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	inspected, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(inspected); err == nil {
		t.Fatal("PrepareEditableLayout accepted a non-directory recovery path")
	}

	gotDatabase, err := os.ReadFile(layout.Database)
	if err != nil {
		t.Fatalf("read database after failure: %v", err)
	}
	if string(gotDatabase) != string(wantDatabase) {
		t.Fatalf("database changed during recovery failure:\nwant=%q\ngot =%q", wantDatabase, gotDatabase)
	}
}

func TestInspectLayoutRejectsOrdinaryDirectory(t *testing.T) {
	_, err := InspectLayout(t.TempDir())

	if !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("error = %v, want ErrInvalidPackage", err)
	}
}

func TestInspectLayoutReturnsPathsForExistingPackage(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}

	opened, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Root == "" || opened != layout {
		t.Fatalf("opened = %#v, want %#v", opened, layout)
	}
}

func TestInspectLayoutDoesNotCreateImportsDirectoryForLegacyPackage(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(layout.Imports); err != nil {
		t.Fatal(err)
	}

	opened, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(opened.Imports); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("InspectLayout created %q: %v", opened.Imports, err)
	}
}

func TestInspectLayoutDoesNotCleanOrphanImportTemporaryFiles(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	orphanName := ".import-source-" + strings.Repeat("a", 32)
	otherHiddenName := ".import-source-keep"
	directoryName := ".import-source-" + strings.Repeat("b", 32)
	if err := os.WriteFile(filepath.Join(layout.Imports, orphanName), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-importTemporaryOrphanAge - time.Hour)
	if err := os.Chtimes(filepath.Join(layout.Imports, orphanName), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.Imports, otherHiddenName), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(layout.Imports, directoryName), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := InspectLayout(layout.Root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(layout.Imports, orphanName)); err != nil {
		t.Fatalf("InspectLayout removed orphan import temporary: %v", err)
	}
	for _, name := range []string{otherHiddenName, directoryName} {
		if _, err := os.Stat(filepath.Join(layout.Imports, name)); err != nil {
			t.Fatalf("unrelated hidden entry %q was removed: %v", name, err)
		}
	}
}

func TestPrepareEditableLayoutCleansOnlyOwnedOrphanImportTemporaryFiles(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	orphanName := ".import-source-" + strings.Repeat("a", 32)
	otherHiddenName := ".import-source-keep"
	directoryName := ".import-source-" + strings.Repeat("b", 32)
	if err := os.WriteFile(filepath.Join(layout.Imports, orphanName), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-importTemporaryOrphanAge - time.Hour)
	if err := os.Chtimes(filepath.Join(layout.Imports, orphanName), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.Imports, otherHiddenName), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(layout.Imports, directoryName), 0o700); err != nil {
		t.Fatal(err)
	}

	inspected, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(inspected); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(layout.Imports, orphanName)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("owned orphan was not removed: %v", err)
	}
	for _, name := range []string{otherHiddenName, directoryName} {
		if _, err := os.Stat(filepath.Join(layout.Imports, name)); err != nil {
			t.Fatalf("unrelated hidden entry %q was removed: %v", name, err)
		}
	}
}

func TestPrepareEditableLayoutLeavesRecentOwnedImportTemporaryFile(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "进行中的作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	recentName := ".import-source-" + strings.Repeat("c", 32)
	if err := os.WriteFile(filepath.Join(layout.Imports, recentName), []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}

	inspected, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(inspected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(layout.Imports, recentName)); err != nil {
		t.Fatalf("recent import temporary was removed: %v", err)
	}
}

func TestPrepareEditableLayoutIgnoresFailureToRemoveOneOldImportTemporary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory mode bits do not reliably deny deletion on Windows")
	}
	layout, err := NewLayout(t.TempDir(), "只读旧项目")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	orphanName := ".import-source-" + strings.Repeat("d", 32)
	orphanPath := filepath.Join(layout.Imports, orphanName)
	if err := os.WriteFile(orphanPath, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-importTemporaryOrphanAge - time.Hour)
	if err := os.Chtimes(orphanPath, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(layout.Imports, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(layout.Imports, 0o700)
	})

	inspected, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(inspected); err != nil {
		t.Fatalf("PrepareEditableLayout failed because one orphan could not be removed: %v", err)
	}
	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("unremovable orphan should remain for a later cleanup: %v", err)
	}
}

func TestPrepareEditableLayoutStillRejectsUnusableImportsDirectory(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "损坏的旧项目")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(layout.Imports); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.Imports, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	inspected, err := InspectLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareEditableLayout(inspected); err == nil {
		t.Fatal("PrepareEditableLayout accepted a non-directory imports path")
	}
}
