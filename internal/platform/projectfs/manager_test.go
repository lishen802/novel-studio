package projectfs

import (
	"os"
	"testing"
)

func TestManagerCreatesInspectsAndPreparesEditableProjectPackage(t *testing.T) {
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
		created.Recovery,
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

	inspected, err := manager.Inspect(created.Root)
	if err != nil {
		t.Fatal(err)
	}
	if inspected != created {
		t.Fatalf("inspected = %#v, want %#v", inspected, created)
	}
	if err := manager.PrepareEditable(inspected); err != nil {
		t.Fatal(err)
	}
}

func TestManagerInspectDoesNotCreateImportsDirectoryForLegacyPackage(t *testing.T) {
	manager := Manager{}
	created, err := manager.Create(t.TempDir(), "旧作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(created.Imports); err != nil {
		t.Fatal(err)
	}

	opened, err := manager.Inspect(created.Root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(opened.Imports); !os.IsNotExist(err) {
		t.Fatalf("Inspect created %q: %v", opened.Imports, err)
	}
	if err := manager.PrepareEditable(opened); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(opened.Imports)
	if err != nil || !info.IsDir() {
		t.Fatalf("PrepareEditable did not create %q: %v", opened.Imports, err)
	}
}
