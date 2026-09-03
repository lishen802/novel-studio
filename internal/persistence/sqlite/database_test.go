package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
		"provider_connections", "agent_profiles", "agent_runtime_settings", "agent_runs",
		"agent_run_steps", "agent_artifacts", "context_manifests", "context_manifest_items",
		"agent_review_findings",
		"import_jobs", "import_chapters",
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
	assertSchemaObjectExists(t, db, "index", "idx_agent_runs_project_created")
	assertSchemaObjectExists(t, db, "index", "idx_agent_run_steps_run_position_attempt")
	assertSchemaObjectExists(t, db, "index", "idx_import_jobs_project_source_hash")
	assertSchemaObjectExists(t, db, "index", "idx_import_chapters_job_position")
	assertSchemaObjectExists(t, db, "table", "extraction_tasks")
	assertSchemaObjectExists(t, db, "table", "extraction_steps")
	assertSchemaObjectExists(t, db, "table", "extraction_artifacts")
	assertSchemaObjectExists(t, db, "index", "idx_extraction_tasks_project")
	assertSchemaObjectExists(t, db, "index", "idx_extraction_steps_task")
	assertSchemaObjectExists(t, db, "table", "creation_sessions")
	assertSchemaObjectExists(t, db, "table", "creation_candidates")
	assertSchemaObjectExists(t, db, "table", "story_assets")
	assertSchemaObjectExists(t, db, "table", "assistant_actions")

	var version int64
	if err := db.QueryRow(`
		SELECT MAX(version_id)
		FROM goose_db_version
		WHERE is_applied = 1
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestJobSnapshotConsistencyMigrationBackfillsQueuedRuns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 15); err != nil {
		t.Fatalf("migrate to version fifteen: %v", err)
	}
	now := formatTime(time.Unix(100, 0))
	for _, statement := range []string{
		`INSERT INTO projects(id, title, created_at, updated_at) VALUES ('project-1', '旧项目', ?, ?)`,
		`INSERT INTO chapters(id, project_id, title, position, revision, created_at, updated_at) VALUES ('chapter-1', 'project-1', '第一章', 1, 1, ?, ?)`,
		`INSERT INTO agent_runs(id, project_id, chapter_id, chapter_base_revision, prompt, status, created_at, updated_at) VALUES ('queued-run', 'project-1', 'chapter-1', 1, '续写', 'queued', ?, ?)`,
	} {
		if _, err := db.ExecContext(ctx, statement, now, now); err != nil {
			t.Fatalf("seed v15 queued run: %v", err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate to current: %v", err)
	}
	var eventType, status, stepStates string
	var sequence, maxSequence int
	if err := db.QueryRowContext(ctx, `
		SELECT e.event_type, e.sequence_number, s.status, s.max_sequence, s.step_states
		FROM job_events e JOIN job_snapshots s ON s.job_id = e.job_id
		WHERE e.job_id = 'queued-run'
	`).Scan(&eventType, &sequence, &status, &maxSequence, &stepStates); err != nil {
		t.Fatalf("read backfilled queued job: %v", err)
	}
	if eventType != "job_queued" || sequence != 1 || status != "queued" || maxSequence != 1 || stepStates != "[]" {
		t.Fatalf("backfilled queued job = (%q, %d, %q, %d, %q)", eventType, sequence, status, maxSequence, stepStates)
	}
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

func TestAgentRuntimeControlsMigrationPreservesVersionElevenRuns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 11); err != nil {
		t.Fatalf("migrate to version eleven: %v", err)
	}
	now := formatTime(time.Unix(100, 0))
	statements := []string{
		`INSERT INTO projects(id, title, created_at, updated_at) VALUES ('project-1', '旧项目', ?, ?)`,
		`INSERT INTO chapters(id, project_id, title, position, revision, created_at, updated_at) VALUES ('chapter-1', 'project-1', '第一章', 1, 1, ?, ?)`,
		`INSERT INTO provider_connections(id, project_id, name, kind, base_url, credential_ref, created_at, updated_at) VALUES ('provider-1', 'project-1', 'OpenAI', 'openai', 'https://example.test/v1', 'provider:1', ?, ?)`,
		`INSERT INTO agent_profiles(id, project_id, role, connection_id, model, max_output_tokens, temperature, enabled, created_at, updated_at) VALUES ('profile-1', 'project-1', 'planner', 'provider-1', 'model', 1000, 0.2, 1, ?, ?)`,
		`INSERT INTO agent_runs(id, project_id, chapter_id, chapter_base_revision, prompt, status, created_at, updated_at) VALUES ('run-1', 'project-1', 'chapter-1', 1, '续写', 'failed', ?, ?)`,
		`INSERT INTO agent_run_steps(id, run_id, role, profile_id, position, status, output, created_at, updated_at) VALUES ('step-1', 'run-1', 'planner', 'profile-1', 1, 'completed', '旧规划', ?, ?)`,
		`INSERT INTO agent_artifacts(id, run_id, step_id, kind, content, sha256, created_at) VALUES ('artifact-1', 'run-1', 'step-1', 'plan', '旧规划', 'hash', ?)`,
	}
	for index, statement := range statements {
		args := []any{now, now}
		if index == len(statements)-1 {
			args = []any{now}
		}
		if _, err := db.ExecContext(ctx, statement, args...); err != nil {
			t.Fatalf("seed version eleven statement %d: %v", index, err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate agent runtime controls: %v", err)
	}
	var tokenBudget, contextBudget, inputTokens, outputTokens int
	if err := db.QueryRowContext(ctx, `
		SELECT token_budget, context_char_budget, input_tokens, output_tokens
		FROM agent_runs WHERE id = 'run-1'
	`).Scan(&tokenBudget, &contextBudget, &inputTokens, &outputTokens); err != nil {
		t.Fatal(err)
	}
	if tokenBudget != 30000 || contextBudget != 120000 ||
		inputTokens != 0 || outputTokens != 0 {
		t.Fatalf(
			"migrated budgets = (%d, %d, %d, %d)",
			tokenBudget,
			contextBudget,
			inputTokens,
			outputTokens,
		)
	}
	var output, artifact string
	if err := db.QueryRowContext(ctx, "SELECT output FROM agent_run_steps WHERE id = 'step-1'").Scan(&output); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT content FROM agent_artifacts WHERE id = 'artifact-1'").Scan(&artifact); err != nil {
		t.Fatal(err)
	}
	if output != "旧规划" || artifact != "旧规划" {
		t.Fatalf("legacy records changed: output=%q artifact=%q", output, artifact)
	}
}

func TestManuscriptImportMigrationPreservesVersionTwelveProjectDataAndDownIsClean(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
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
	if _, err := provider.UpTo(ctx, 12); err != nil {
		t.Fatalf("migrate to version twelve: %v", err)
	}

	now := formatTime(time.Unix(100, 0))
	statements := []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO projects(id, title, genre, summary, created_at, updated_at)
			 VALUES ('project-1', '旧项目', '奇幻', '旧简介', ?, ?)`,
			[]any{now, now},
		},
		{
			`INSERT INTO chapters(
				id, project_id, title, position, body, revision,
				revision_source, revision_summary, created_at, updated_at
			 ) VALUES ('chapter-1', 'project-1', '第一章', 1, '旧正文', 3, 'author', '旧修订', ?, ?)`,
			[]any{now, now},
		},
		{
			`INSERT INTO provider_connections(
				id, project_id, name, kind, base_url, credential_ref, created_at, updated_at
			 ) VALUES (
				'provider-1', 'project-1', 'OpenAI', 'openai',
				'https://example.test/v1', 'provider:1', ?, ?
			 )`,
			[]any{now, now},
		},
		{
			`INSERT INTO agent_profiles(
				id, project_id, role, connection_id, model, max_output_tokens,
				temperature, enabled, created_at, updated_at
			 ) VALUES (
				'profile-1', 'project-1', 'planner', 'provider-1', 'model',
				1000, 0.2, 1, ?, ?
			 )`,
			[]any{now, now},
		},
		{
			`INSERT INTO agent_runs(
				id, project_id, chapter_id, chapter_base_revision, prompt, status,
				token_budget, context_char_budget, input_tokens, output_tokens,
				context_chars, context_truncated, created_at, updated_at
			 ) VALUES (
				'run-1', 'project-1', 'chapter-1', 3, '续写', 'completed',
				4000, 8000, 100, 50, 2000, 0, ?, ?
			 )`,
			[]any{now, now},
		},
		{
			`INSERT INTO agent_run_steps(
				id, run_id, role, profile_id, position, status, output,
				provider_request_id, input_tokens, output_tokens, created_at, updated_at
			 ) VALUES (
				'step-1', 'run-1', 'planner', 'profile-1', 1, 'completed', '旧规划',
				'request-1', 100, 50, ?, ?
			 )`,
			[]any{now, now},
		},
		{
			`INSERT INTO agent_artifacts(
				id, run_id, step_id, kind, content, sha256, created_at
			 ) VALUES ('artifact-1', 'run-1', 'step-1', 'plan', '旧规划', 'hash', ?)`,
			[]any{now},
		},
	}
	for index, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed version twelve statement %d: %v", index, err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate to version thirteen: %v", err)
	}
	assertSchemaObjectExists(t, db, "table", "import_jobs")
	assertSchemaObjectExists(t, db, "table", "import_chapters")

	var projectTitle, chapterBody, revisionSource, artifact string
	var chapterRevision, inputTokens int
	if err := db.QueryRowContext(ctx, `
		SELECT p.title, c.body, c.revision, c.revision_source, r.input_tokens, a.content
		FROM projects p
		JOIN chapters c ON c.project_id = p.id
		JOIN agent_runs r ON r.chapter_id = c.id
		JOIN agent_artifacts a ON a.run_id = r.id
		WHERE p.id = 'project-1'
	`).Scan(
		&projectTitle,
		&chapterBody,
		&chapterRevision,
		&revisionSource,
		&inputTokens,
		&artifact,
	); err != nil {
		t.Fatal(err)
	}
	if projectTitle != "旧项目" || chapterBody != "旧正文" || chapterRevision != 3 ||
		revisionSource != "author" || inputTokens != 100 || artifact != "旧规划" {
		t.Fatalf(
			"legacy records changed: project=%q body=%q revision=%d source=%q input=%d artifact=%q",
			projectTitle,
			chapterBody,
			chapterRevision,
			revisionSource,
			inputTokens,
			artifact,
		)
	}

	if _, err := provider.DownTo(ctx, 12); err != nil {
		t.Fatalf("migrate down to version twelve: %v", err)
	}
	assertSchemaObjectMissing(t, db, "table", "import_chapters")
	assertSchemaObjectMissing(t, db, "table", "import_jobs")
	if err := db.QueryRowContext(ctx, "SELECT body FROM chapters WHERE id = 'chapter-1'").
		Scan(&chapterBody); err != nil {
		t.Fatalf("read chapter after down migration: %v", err)
	}
	if chapterBody != "旧正文" {
		t.Fatalf("chapter body after down migration = %q", chapterBody)
	}
}

func TestExtractionRuntimeMigrationPreservesVersionThirteenDataAndDownIsClean(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
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
	if _, err := provider.UpTo(ctx, 13); err != nil {
		t.Fatalf("migrate to version thirteen: %v", err)
	}

	now := formatTime(time.Unix(100, 0))
	seedStatements := []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO projects(id, title, genre, summary, created_at, updated_at)
			 VALUES ('project-1', '旧项目', '奇幻', '旧简介', ?, ?)`,
			[]any{now, now},
		},
		{
			`INSERT INTO chapters(
				id, project_id, title, position, body, revision,
				revision_source, revision_summary, created_at, updated_at
			 ) VALUES ('chapter-1', 'project-1', '第一章', 1, '旧正文', 2, 'author', '旧修订', ?, ?)`,
			[]any{now, now},
		},
		{
			`INSERT INTO characters(
				id, project_id, name, aliases_json, role, summary, motivation, fear,
				secret, arc, status, created_at, updated_at, deleted_at
			 ) VALUES ('character-1', 'project-1', '林默', '[]', 'protagonist', '旧摘要', '', '', '', '', 'active', ?, ?, NULL)`,
			[]any{now, now},
		},
		{
			`INSERT INTO change_sets(
				id, project_id, title, summary, created_by, status, created_at, updated_at
			 ) VALUES ('change-set-1', 'project-1', '旧候选', '', 'agent:planner', 'applied', ?, ?)`,
			[]any{now, now},
		},
		{
			`INSERT INTO change_operations(
				id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
				before_value, after_value, reason, status, created_at, updated_at
			 ) VALUES (
				'operation-1', 'change-set-1', 'update_character', 1, 'character-1', 0, 'fp',
				'{"name":"林默"}', '{"name":"林默","summary":"新摘要"}', '更新摘要', 'applied', ?, ?
			 )`,
			[]any{now, now},
		},
		{
			`INSERT INTO import_jobs(
				id, project_id, source_name, source_format, source_encoding,
				source_hash, source_bytes, archived_relative_path, status,
				chapter_count, selected_chapter_count, error_code, error_message,
				created_at, updated_at, applied_at
			 ) VALUES (
				'import-job-1', 'project-1', 'novel.txt', 'txt', 'UTF-8',
				'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1024, 'imports/novel.txt', 'applied',
				1, 1, '', '', ?, ?, ?
			 )`,
			[]any{now, now, now},
		},
	}
	for index, statement := range seedStatements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed version thirteen statement %d: %v", index, err)
		}
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate to version fourteen: %v", err)
	}

	// New extraction runtime tables exist.
	assertSchemaObjectExists(t, db, "table", "extraction_tasks")
	assertSchemaObjectExists(t, db, "table", "extraction_steps")
	assertSchemaObjectExists(t, db, "table", "extraction_artifacts")
	assertSchemaObjectExists(t, db, "index", "idx_extraction_tasks_project")
	assertSchemaObjectExists(t, db, "index", "idx_extraction_steps_task")

	// change_operations gained an evidence column defaulting to "[]" for
	// existing rows, and the kind CHECK now admits the create_* family.
	var evidence, kind, afterValue string
	if err := db.QueryRowContext(ctx, `
		SELECT evidence, kind, after_value
		FROM change_operations
		WHERE id = 'operation-1'
	`).Scan(&evidence, &kind, &afterValue); err != nil {
		t.Fatalf("read migrated change operation: %v", err)
	}
	if evidence != "[]" || kind != "update_character" || afterValue != `{"name":"林默","summary":"新摘要"}` {
		t.Fatalf("migrated operation = (evidence=%q kind=%q after=%q)", evidence, kind, afterValue)
	}
	// The create_* family is now accepted by the kind CHECK. Insert one of
	// each kind to prove the constraint admits them, then verify the downgrade
	// guard refuses to silently discard them.
	for _, accepted := range []string{
		"create_outline_node", "create_character", "create_relationship",
		"create_world_rule", "create_story_event", "create_fact",
		"create_foreshadowing_hook",
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO change_operations(
				id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
				before_value, after_value, reason, evidence, status, created_at, updated_at
			) VALUES (
				?, 'change-set-1', ?, 2, 'target-1', 0, ?,
				'{}', '{"v":1}', '抽取候选', '[]', 'pending', ?, ?
			)
		`, "operation-"+accepted, accepted, "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae", now, now); err != nil {
			t.Fatalf("insert %s operation: %v", accepted, err)
		}
	}

	// import_jobs gained extraction_status (default "none") and extraction_run_id.
	var extractionStatus string
	var extractionRunID sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT extraction_status, extraction_run_id
		FROM import_jobs
		WHERE id = 'import-job-1'
	`).Scan(&extractionStatus, &extractionRunID); err != nil {
		t.Fatalf("read migrated import job: %v", err)
	}
	if extractionStatus != "none" || extractionRunID.Valid {
		t.Fatalf("migrated import job extraction = (status=%q runID=%v)", extractionStatus, extractionRunID)
	}

	// Legacy chapter and character data are untouched.
	var chapterBody string
	var chapterRevision int
	if err := db.QueryRowContext(ctx, "SELECT body, revision FROM chapters WHERE id = 'chapter-1'").
		Scan(&chapterBody, &chapterRevision); err != nil {
		t.Fatalf("read chapter after upgrade: %v", err)
	}
	if chapterBody != "旧正文" || chapterRevision != 2 {
		t.Fatalf("chapter changed after upgrade: body=%q revision=%d", chapterBody, chapterRevision)
	}
	var characterSummary string
	if err := db.QueryRowContext(ctx, "SELECT summary FROM characters WHERE id = 'character-1'").
		Scan(&characterSummary); err != nil {
		t.Fatalf("read character after upgrade: %v", err)
	}
	if characterSummary != "旧摘要" {
		t.Fatalf("character summary changed after upgrade: %q", characterSummary)
	}

	// Down migration to thirteen must refuse to silently discard create_*
	// rows. The guard aborts via a NOT NULL constraint on a temp table.
	if _, err := provider.DownTo(ctx, 13); err == nil {
		t.Fatal("downgrade with create_* operations should have been refused")
	}

	// After removing the create_* rows, the downgrade succeeds and preserves
	// the pre-existing update_* rows.
	if _, err := db.ExecContext(ctx, `
		DELETE FROM change_operations
		WHERE kind IN (
			'create_outline_node', 'create_character', 'create_relationship',
			'create_world_rule', 'create_story_event', 'create_fact',
			'create_foreshadowing_hook'
		)
	`); err != nil {
		t.Fatalf("delete create_* operations before downgrade: %v", err)
	}
	if _, err := provider.DownTo(ctx, 13); err != nil {
		t.Fatalf("migrate down to version thirteen: %v", err)
	}
	assertSchemaObjectMissing(t, db, "table", "extraction_tasks")
	assertSchemaObjectMissing(t, db, "table", "extraction_steps")
	assertSchemaObjectMissing(t, db, "table", "extraction_artifacts")

	// The original update_character row remains intact after downgrade.
	var survivingKind, survivingAfter string
	if err := db.QueryRowContext(ctx, `
		SELECT kind, after_value
		FROM change_operations
		WHERE id = 'operation-1'
	`).Scan(&survivingKind, &survivingAfter); err != nil {
		t.Fatalf("read surviving change operation after downgrade: %v", err)
	}
	if survivingKind != "update_character" || survivingAfter != `{"name":"林默","summary":"新摘要"}` {
		t.Fatalf("surviving operation = (kind=%q after=%q)", survivingKind, survivingAfter)
	}

	if err := db.QueryRowContext(ctx, "SELECT body FROM chapters WHERE id = 'chapter-1'").
		Scan(&chapterBody); err != nil {
		t.Fatalf("read chapter after downgrade: %v", err)
	}
	if chapterBody != "旧正文" {
		t.Fatalf("chapter body changed after downgrade: %q", chapterBody)
	}
}

func TestManuscriptImportSchemaEnforcesLifecycleCountsUniquenessAndReferences(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := formatTime(time.Unix(100, 0))

	insertJob := func(
		id, sourceHash, status string,
		chapterCount, selectedCount int,
		errorCode, errorMessage string,
		appliedAt any,
	) error {
		_, err := store.db.ExecContext(ctx, `
			INSERT INTO import_jobs(
				id, project_id, source_name, source_format, source_encoding,
				source_hash, source_bytes, archived_relative_path, status,
				chapter_count, selected_chapter_count, error_code, error_message,
				created_at, updated_at, applied_at
			) VALUES (?, ?, 'novel.txt', 'txt', 'UTF-8', ?, 100, 'imports/job.txt', ?,
				?, ?, ?, ?, ?, ?, ?)
		`,
			id,
			project.ID,
			sourceHash,
			status,
			chapterCount,
			selectedCount,
			errorCode,
			errorMessage,
			now,
			now,
			appliedAt,
		)
		return err
	}

	hashA := strings.Repeat("a", 64)
	if err := insertJob("prepared", hashA, "prepared", 2, 0, "", "", nil); err != nil {
		t.Fatalf("insert prepared job: %v", err)
	}
	if err := insertJob(
		"failed-zero-chapters",
		strings.Repeat("d", 64),
		"failed",
		0,
		0,
		"parse_empty",
		"manuscript contains no chapters",
		nil,
	); err != nil {
		t.Fatalf("insert zero-chapter failed job: %v", err)
	}
	for name, args := range map[string]struct {
		status                      string
		chapterCount, selectedCount int
		errorCode, errorMessage     string
		appliedAt                   any
	}{
		"unknown status":            {"unknown", 1, 0, "", "", nil},
		"zero chapters":             {"prepared", 0, 0, "", "", nil},
		"prepared selected":         {"prepared", 1, 1, "", "", nil},
		"applied without selection": {"applied", 1, 0, "", "", now},
		"applied without time":      {"applied", 1, 1, "", "", nil},
		"failed without safe error": {"failed", 1, 0, "", "", nil},
	} {
		t.Run(name, func(t *testing.T) {
			err := insertJob(
				"invalid-"+strings.ReplaceAll(name, " ", "-"),
				strings.Repeat("b", 63)+string(rune('0'+len(name)%10)),
				args.status,
				args.chapterCount,
				args.selectedCount,
				args.errorCode,
				args.errorMessage,
				args.appliedAt,
			)
			if err == nil {
				t.Fatal("insert succeeded, want CHECK constraint failure")
			}
		})
	}
	if err := insertJob("duplicate-source", hashA, "prepared", 1, 0, "", "", nil); err == nil {
		t.Fatal("duplicate project/source hash succeeded")
	}
	if err := insertJob(
		"invalid-source-hash",
		strings.Repeat("z", 64),
		"prepared",
		1,
		0,
		"",
		"",
		nil,
	); err == nil {
		t.Fatal("non-hexadecimal source hash succeeded")
	}

	insertChapter := func(id, jobID string, position, selected int, createdChapterID any) error {
		_, err := store.db.ExecContext(ctx, `
			INSERT INTO import_chapters(
				id, job_id, volume_title, title, body, position,
				source_start_line, source_end_line, content_hash,
				selected, created_chapter_id
			) VALUES (?, ?, '', '第一章', '正文', ?, 1, 2, ?, ?, ?)
		`,
			id,
			jobID,
			position,
			strings.Repeat("c", 64),
			selected,
			createdChapterID,
		)
		return err
	}
	if err := insertChapter("candidate-1", "prepared", 1, 0, nil); err != nil {
		t.Fatalf("insert import chapter: %v", err)
	}
	if err := insertChapter("candidate-duplicate-position", "prepared", 1, 0, nil); err == nil {
		t.Fatal("duplicate job position succeeded")
	}
	if err := insertChapter("candidate-missing-job", "missing", 2, 0, nil); err == nil {
		t.Fatal("missing job reference succeeded")
	}
	if err := insertChapter("candidate-missing-chapter", "prepared", 2, 1, "missing"); err == nil {
		t.Fatal("missing created chapter reference succeeded")
	}
	if err := insertChapter("candidate-selected-without-mapping", "prepared", 2, 1, nil); err == nil {
		t.Fatal("selected candidate without created chapter mapping succeeded")
	}
	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO import_chapters(
			id, job_id, volume_title, title, body, position,
			source_start_line, source_end_line, content_hash, selected
		) VALUES ('candidate-invalid-hash', 'prepared', '', '第二章', '正文', 2, 3, 4, ?, 0)
	`, strings.Repeat("z", 64)); err == nil {
		t.Fatal("non-hexadecimal content hash succeeded")
	}

	if _, err := store.db.ExecContext(ctx, "DELETE FROM import_jobs WHERE id = 'prepared'"); err != nil {
		t.Fatalf("delete prepared import job: %v", err)
	}
	var candidateCount int
	if err := store.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM import_chapters WHERE job_id = 'prepared'",
	).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 0 {
		t.Fatalf("candidate count after job delete = %d, want 0", candidateCount)
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

func assertSchemaObjectMissing(t *testing.T, db *sql.DB, objectType, name string) {
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
	if count != 0 {
		t.Fatalf("%s %q count = %d, want 0", objectType, name, count)
	}
}
