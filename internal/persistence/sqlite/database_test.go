package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
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
		"world_rules", "foreshadowing_hooks", "region_adjacencies",
		"story_events", "story_event_links", "canon_facts", "fact_evidence",
		"fact_scopes", "fact_conflicts", "character_knowledge",
		"change_sets", "change_operations",
	} {
		assertSchemaObjectExists(t, db, "table", name)
	}
	assertSchemaObjectExists(t, db, "index", "idx_chapters_project_position")
	assertSchemaObjectExists(t, db, "index", "idx_chapter_revisions_chapter_revision_desc")
	assertSchemaObjectExists(t, db, "index", "idx_outline_nodes_parent_position")
	assertSchemaObjectExists(t, db, "index", "idx_story_line_links_position")
	assertSchemaObjectExists(t, db, "index", "idx_scene_cards_chapter_position")
	assertSchemaObjectExists(t, db, "index", "idx_characters_name")
	assertSchemaObjectExists(t, db, "index", "idx_relationships_source")
	assertSchemaObjectExists(t, db, "index", "idx_chapter_characters_character")
}

func TestChangeSetSchemaSupportsStructuredPartialReview(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := formatTime(time.Unix(100, 0))

	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO change_sets(
			id, project_id, title, summary, created_by, status, created_at, updated_at
		) VALUES (?, ?, ?, '', 'agent:planner', 'partially_applied', ?, ?)
	`, "change-set-structured", project.ID, "结构化候选", now, now); err != nil {
		t.Fatalf("insert partially applied change set: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO change_operations(
			id, change_set_id, kind, target_id, base_revision, base_fingerprint,
			before_value, after_value, reason, status, created_at, updated_at
		) VALUES (?, ?, 'update_character', ?, 0, ?, ?, ?, '', 'pending', ?, ?)
	`,
		"operation-character",
		"change-set-structured",
		"character-1",
		"fingerprint",
		`{"name":"旧名"}`,
		`{"name":"新名"}`,
		now,
		now,
	); err != nil {
		t.Fatalf("insert structured change operation: %v", err)
	}
}

func TestStructuredChangeSetMigrationPreservesVersionNineCandidates(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 9); err != nil {
		t.Fatalf("migrate to version nine: %v", err)
	}
	now := formatTime(time.Unix(100, 0))
	if _, err := db.ExecContext(ctx, `
		INSERT INTO projects(id, title, genre, summary, created_at, updated_at)
		VALUES ('project-1', '旧项目', '', '', ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO change_sets(id, project_id, title, summary, created_by, status, created_at, updated_at)
		VALUES ('change-set-1', 'project-1', '旧候选', '', 'agent:writer', 'pending', ?, ?)
	`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO change_operations(
			id, change_set_id, kind, target_id, base_revision, before_value,
			after_value, reason, status, created_at, updated_at
		) VALUES (
			'operation-1', 'change-set-1', 'replace_chapter_body', 'chapter-1',
			1, '旧正文', '新正文', '', 'pending', ?, ?
		)
	`, now, now); err != nil {
		t.Fatal(err)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate existing candidate to version ten: %v", err)
	}
	var kind, beforeValue, afterValue, fingerprint string
	var position int
	if err := db.QueryRowContext(ctx, `
		SELECT kind, position, before_value, after_value, base_fingerprint
		FROM change_operations WHERE id = 'operation-1'
	`).Scan(&kind, &position, &beforeValue, &afterValue, &fingerprint); err != nil {
		t.Fatal(err)
	}
	if kind != "replace_chapter_body" || position != 1 ||
		beforeValue != "旧正文" || afterValue != "新正文" || fingerprint != "" {
		t.Fatalf(
			"migrated operation = (%q, %d, %q, %q, %q)",
			kind, position, beforeValue, afterValue, fingerprint,
		)
	}
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
