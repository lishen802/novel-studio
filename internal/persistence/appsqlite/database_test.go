package appsqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesVersionedApplicationSchema(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{
		"app_preferences", "provider_connections", "creative_profile_versions",
		"creative_profile_roles", "recent_projects", "legacy_imports",
	} {
		assertTableExists(t, db, table)
	}
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.db")
	db1, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()
	db2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	assertTableExists(t, db2, "app_preferences")
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?
	`, table).Scan(&count)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("table %s does not exist", table)
	}
}
