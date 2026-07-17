package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDatabaseMigratesEmptyDatabaseIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")

	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}

	for _, name := range []string{
		"projects", "chapters", "chapter_revisions", "drafts",
		"outline_nodes", "story_lines", "story_line_links", "scene_cards",
		"characters", "factions", "faction_memberships", "character_relationships",
		"chapter_characters", "scene_card_characters",
		"world_regions", "locations", "location_links", "items", "item_transfers",
		"world_rules", "foreshadowing_hooks",
		"story_events", "story_event_links", "canon_facts", "fact_evidence",
		"fact_scopes", "fact_conflicts",
	} {
		assertSchemaObjectExists(t, db, "table", name)
	}
	assertSchemaObjectExists(t, db, "index", "idx_chapters_project_position")
	assertSchemaObjectExists(t, db, "index", "idx_outline_nodes_parent_position")
	assertSchemaObjectExists(t, db, "index", "idx_story_line_links_position")
	assertSchemaObjectExists(t, db, "index", "idx_scene_cards_chapter_position")
	assertSchemaObjectExists(t, db, "index", "idx_characters_name")
	assertSchemaObjectExists(t, db, "index", "idx_relationships_source")
	assertSchemaObjectExists(t, db, "index", "idx_chapter_characters_character")
}

func TestOpenDatabaseClassifiesCorruptFileWithoutOverwritingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.db")
	want := []byte("this is not a sqlite database")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := OpenDatabase(context.Background(), path)
	if !errors.Is(err, ErrDatabaseCorrupt) {
		t.Fatalf("error = %v, want ErrDatabaseCorrupt", err)
	}

	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("database contents changed: %q", got)
	}
}

func assertSchemaObjectExists(t *testing.T, db *sql.DB, objectType, name string) {
	t.Helper()
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		objectType,
		name,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("%s %q count = %d, want 1", objectType, name, count)
	}
}
