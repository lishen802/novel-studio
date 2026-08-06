package legacyproject

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBuildCreatesLegacyV9ProjectPackage(t *testing.T) {
	parent := t.TempDir()
	fixture := Build(t, parent)

	wantPackage := filepath.Join(parent, packageName)
	if fixture.PackagePath != wantPackage {
		t.Fatalf("PackagePath = %q, want %q", fixture.PackagePath, wantPackage)
	}
	wantDatabase := filepath.Join(wantPackage, databaseFile)
	if fixture.DatabasePath != wantDatabase {
		t.Fatalf("DatabasePath = %q, want %q", fixture.DatabasePath, wantDatabase)
	}

	info, err := os.Stat(fixture.DatabasePath)
	if err != nil {
		t.Fatalf("Stat database: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("database mode = %v, want regular file", info.Mode())
	}
	if info.Size() == 0 {
		t.Fatalf("database file is empty")
	}

	// Legacy packages must not pre-create imports/, recovery/ or any other
	// layout directory; OpenLayout compatibility is covered separately.
	for _, name := range []string{"imports", "recovery", "snapshots", "branches", "attachments", "exports"} {
		path := filepath.Join(fixture.PackagePath, name)
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("legacy package pre-created %q: %v", name, err)
		}
	}

	// No stray -wal/-shm files should remain after Build checkpoints the
	// database, so the package is self-contained for copying across platforms.
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(fixture.DatabasePath + suffix); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("legacy database left a %q file behind", suffix)
		}
	}
}

func TestBuildStopsAtSchemaVersionNine(t *testing.T) {
	fixture := Build(t, t.TempDir())

	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var version int64
	if err := db.QueryRowContext(context.Background(), `
		SELECT MAX(version_id)
		FROM goose_db_version
		WHERE is_applied = 1
	`).Scan(&version); err != nil {
		t.Fatalf("read goose version: %v", err)
	}
	if version != 9 {
		t.Fatalf("schema version = %d, want 9", version)
	}

	for _, table := range []string{
		"projects", "chapters", "chapter_revisions", "drafts",
		"characters", "world_rules", "canon_facts",
		"change_sets", "change_operations",
	} {
		assertTableExists(t, db, table)
	}

	// v11+ tables must not exist; this guards against accidental migration
	// past v9 and ensures the fixture genuinely exercises the v9 -> v14 path.
	for _, table := range []string{
		"agent_runs", "agent_run_steps", "agent_artifacts",
		"agent_runtime_settings", "agent_profiles", "provider_connections",
		"import_jobs", "import_chapters",
		"extraction_tasks", "extraction_steps", "extraction_artifacts",
	} {
		assertTableMissing(t, db, table)
	}
}

func TestBuildIsDeterministicAcrossRunsAndMatchesExpected(t *testing.T) {
	fixtureA := Build(t, t.TempDir())
	fixtureB := Build(t, t.TempDir())

	sentinelsA := readBusinessSentinels(t, fixtureA.DatabasePath)
	sentinelsB := readBusinessSentinels(t, fixtureB.DatabasePath)
	if sentinelsA != sentinelsB {
		t.Fatalf("business sentinels differ between runs:\nA=%#v\nB=%#v", sentinelsA, sentinelsB)
	}

	if sentinelsA != fixtureA.Expected {
		t.Fatalf("sentinels do not match expected.json:\ngot=%#v\nwant=%#v", sentinelsA, fixtureA.Expected)
	}
	if fixtureA.Expected != fixtureB.Expected {
		t.Fatalf("Expected changed between runs:\nfirst=%#v\nsecond=%#v", fixtureA.Expected, fixtureB.Expected)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		name,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("table %q count = %d, want 1", name, count)
	}
}

func assertTableMissing(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		name,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("table %q count = %d, want 0 (table should not exist at v9)", name, count)
	}
}
