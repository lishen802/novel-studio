package projectfs

import (
	"os"
	"testing"
)

func TestManagerCreatesAndOpensProjectPackage(t *testing.T) {
	manager := Manager{}

	created, err := manager.Create(t.TempDir(), "长夜织境")
	if err != nil {
		t.Fatal(err)
	}
	if created.Root == "" || created.Database == "" {
		t.Fatalf("created package = %#v", created)
	}
	for _, directory := range []string{
		created.Attachments,
		created.Exports,
		created.Imports,
		created.Snapshots,
		created.Branches,
	} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("Stat(%q): %v", directory, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", directory)
		}
	}

	opened, err := manager.Open(created.Root)
	if err != nil {
		t.Fatal(err)
	}
	if opened != created {
		t.Fatalf("opened = %#v, want %#v", opened, created)
	}
}

func TestManagerOpenCreatesImportsDirectoryForLegacyPackage(t *testing.T) {
	manager := Manager{}
	created, err := manager.Create(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(created.Imports); err != nil {
		t.Fatal(err)
	}

	opened, err := manager.Open(created.Root)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(opened.Imports)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", opened.Imports)
	}
}
