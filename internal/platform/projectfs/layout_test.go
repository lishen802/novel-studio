package projectfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
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
	if layout.Snapshots != filepath.Join(wantRoot, "snapshots") {
		t.Fatalf("Snapshots = %q", layout.Snapshots)
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

func TestOpenLayoutRejectsOrdinaryDirectory(t *testing.T) {
	_, err := OpenLayout(t.TempDir())

	if !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("error = %v, want ErrInvalidPackage", err)
	}
}

func TestOpenLayoutReturnsPathsForExistingPackage(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.Create(); err != nil {
		t.Fatal(err)
	}

	opened, err := OpenLayout(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Root == "" || opened != layout {
		t.Fatalf("opened = %#v, want %#v", opened, layout)
	}
}
