package legacyproject

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// readBusinessSentinels opens the fixture database read-only and reads every
// sentinel row that expected.json describes. The returned Expected is
// directly comparable to Fixture.Expected with == so a single assertion
// catches any drift introduced by migration, seed, or schema changes.
func readBusinessSentinels(t *testing.T, databasePath string) Expected {
	t.Helper()
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var got Expected

	if err := db.QueryRow(`
		SELECT id, title, genre, summary
		FROM projects
		WHERE id = 'legacy-project-1'
	`).Scan(
		&got.Project.ID,
		&got.Project.Title,
		&got.Project.Genre,
		&got.Project.Summary,
	); err != nil {
		t.Fatalf("read project sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, title, position, body, revision
		FROM chapters
		WHERE id = 'legacy-chapter-1'
	`).Scan(
		&got.Chapter.ID,
		&got.Chapter.Title,
		&got.Chapter.Position,
		&got.Chapter.Body,
		&got.Chapter.Revision,
	); err != nil {
		t.Fatalf("read chapter sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, chapter_id, revision, title, body
		FROM chapter_revisions
		WHERE chapter_id = 'legacy-chapter-1' AND revision = 1
	`).Scan(
		&got.ChapterRevision.ID,
		&got.ChapterRevision.ChapterID,
		&got.ChapterRevision.Revision,
		&got.ChapterRevision.Title,
		&got.ChapterRevision.Body,
	); err != nil {
		t.Fatalf("read chapter revision sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT chapter_id, body, base_revision
		FROM drafts
		WHERE chapter_id = 'legacy-chapter-1'
	`).Scan(
		&got.Draft.ChapterID,
		&got.Draft.Body,
		&got.Draft.BaseRevision,
	); err != nil {
		t.Fatalf("read draft sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, name, role, summary
		FROM characters
		WHERE id = 'legacy-character-1'
	`).Scan(
		&got.Character.ID,
		&got.Character.Name,
		&got.Character.Role,
		&got.Character.Summary,
	); err != nil {
		t.Fatalf("read character sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, name, description
		FROM world_rules
		WHERE id = 'legacy-world-rule-1'
	`).Scan(
		&got.WorldRule.ID,
		&got.WorldRule.Name,
		&got.WorldRule.Description,
	); err != nil {
		t.Fatalf("read world rule sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, fact_text, category, origin, status
		FROM canon_facts
		WHERE id = 'legacy-fact-1'
	`).Scan(
		&got.CanonFact.ID,
		&got.CanonFact.FactText,
		&got.CanonFact.Category,
		&got.CanonFact.Origin,
		&got.CanonFact.Status,
	); err != nil {
		t.Fatalf("read canon fact sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, title, status
		FROM change_sets
		WHERE id = 'legacy-change-set-1'
	`).Scan(
		&got.ChangeSet.ID,
		&got.ChangeSet.Title,
		&got.ChangeSet.Status,
	); err != nil {
		t.Fatalf("read change set sentinel: %v", err)
	}

	if err := db.QueryRow(`
		SELECT id, change_set_id, kind, target_id, base_revision,
		       before_value, after_value
		FROM change_operations
		WHERE id = 'legacy-change-op-1'
	`).Scan(
		&got.ChangeOperation.ID,
		&got.ChangeOperation.ChangeSetID,
		&got.ChangeOperation.Kind,
		&got.ChangeOperation.TargetID,
		&got.ChangeOperation.BaseRevision,
		&got.ChangeOperation.BeforeValue,
		&got.ChangeOperation.AfterValue,
	); err != nil {
		t.Fatalf("read change operation sentinel: %v", err)
	}

	return got
}
