package appdirs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCreatesNoDirectoryAndUsesStableSettingsPath(t *testing.T) {
	root := t.TempDir()
	paths, err := Resolve(func() (string, error) { return root, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got, want := paths.Database, filepath.Join(root, "StoryStudio", "settings.db"); got != want {
		t.Fatalf("Database = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Dir(paths.Database)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Resolve mutated filesystem: %v", err)
	}
}

func TestResolvePropagatesBaseDirError(t *testing.T) {
	expected := errors.New("no home directory")
	_, err := Resolve(func() (string, error) { return "", expected })
	if !errors.Is(err, expected) {
		t.Fatalf("err = %v, want %v", err, expected)
	}
}

func TestResolveDefaultReturnsNonEmptyPath(t *testing.T) {
	paths, err := ResolveDefault()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Database == "" {
		t.Fatal("Database path is empty")
	}
	if filepath.Base(paths.Database) != "settings.db" {
		t.Fatalf("Database basename = %q, want settings.db", filepath.Base(paths.Database))
	}
}
