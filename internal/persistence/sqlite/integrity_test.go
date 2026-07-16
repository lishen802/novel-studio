package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenExistingDatabaseRejectsMissingFileWithoutCreatingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")

	_, err := OpenExistingDatabase(context.Background(), path)
	if !errors.Is(err, ErrDatabaseMissing) {
		t.Fatalf("error = %v, want ErrDatabaseMissing", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database was created while opening: %v", statErr)
	}
}

func TestOpenExistingDatabaseRejectsNewerSchemaWithoutChangingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO goose_db_version (version_id, is_applied, tstamp)
		VALUES (999, 1, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = OpenExistingDatabase(context.Background(), path)
	if !errors.Is(err, ErrDatabaseTooNew) {
		t.Fatalf("error = %v, want ErrDatabaseTooNew", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("newer database was changed while rejecting it")
	}
}

func TestOpenExistingDatabaseRejectsInvalidSQLiteHeaderWithoutChangingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	want := []byte("not a sqlite project database")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := OpenExistingDatabase(context.Background(), path)
	if !errors.Is(err, ErrDatabaseCorrupt) {
		t.Fatalf("error = %v, want ErrDatabaseCorrupt", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(want) {
		t.Fatal("corrupt database was changed while rejecting it")
	}
}

func TestFactoryOpenDoesNotCreateMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")

	_, err := (Factory{}).Open(context.Background(), path)
	if !errors.Is(err, ErrDatabaseMissing) {
		t.Fatalf("error = %v, want ErrDatabaseMissing", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("database was created while opening: %v", statErr)
	}
}

func TestQuickCheckClassifiesDamagedBTreeAsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA writable_schema = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		UPDATE sqlite_schema
		SET rootpage = 999999
		WHERE type = 'table' AND name = 'chapters'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA writable_schema = RESET"); err != nil {
		t.Fatal(err)
	}

	if err := quickCheck(context.Background(), db); !errors.Is(err, ErrDatabaseCorrupt) {
		t.Fatalf("error = %v, want ErrDatabaseCorrupt", err)
	}
}
