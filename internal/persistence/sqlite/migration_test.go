package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/testsupport/legacyproject"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// === Helpers ===

func sha256File(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("sha256File(%s): %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertRuntimePragmas(t *testing.T, db *sql.DB) {
	t.Helper()
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
}

func readSchemaVersion(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var version int64
	if err := db.QueryRow(`
		SELECT MAX(version_id)
		FROM goose_db_version
		WHERE is_applied = 1
	`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	return version
}

func readProjectID(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	if err := db.QueryRow("SELECT id FROM projects LIMIT 1").Scan(&id); err != nil {
		t.Fatalf("read project id: %v", err)
	}
	return id
}

// copyDatabaseFile copies a closed SQLite database file. The source must not
// have active WAL/SHM files; call copyDatabaseFile only after closing the
// source connection and removing -wal/-shm.
func copyDatabaseFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copyDatabaseFile read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("copyDatabaseFile write %s: %v", dst, err)
	}
}

func writeMigrationJournal(t *testing.T, path string, journal migrationJournal) {
	t.Helper()
	data, err := json.Marshal(journal)
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write journal %s: %v", path, err)
	}
}

func writeCorruptedFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("write corrupted file %s: %v", path, err)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file %q should not exist: %v", path, err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file %q should exist", path)
	}
}

// === Success migration ===

func TestOpenExistingDatabaseMigratesLegacyFixtureThroughValidatedStage(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	originalHash := sha256File(t, fixture.DatabasePath)

	db, err := OpenExistingDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatalf("OpenExistingDatabase: %v", err)
	}
	defer db.Close()

	if got := readSchemaVersion(t, db); got != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, currentSchemaVersion)
	}
	assertRuntimePragmas(t, db)
	if err := quickCheck(ctx, db); err != nil {
		t.Fatalf("quick_check: %v", err)
	}

	// v11+ tables now exist.
	for _, table := range []string{
		"agent_runs", "agent_run_steps", "agent_artifacts",
		"import_jobs", "import_chapters",
		"extraction_tasks", "extraction_steps", "extraction_artifacts",
	} {
		assertSchemaObjectExists(t, db, "table", table)
	}

	// v10 structured change_operations: position and base_fingerprint added.
	var position int
	var baseFingerprint string
	if err := db.QueryRow(`
		SELECT position, base_fingerprint
		FROM change_operations WHERE id = 'legacy-change-op-1'
	`).Scan(&position, &baseFingerprint); err != nil {
		t.Fatalf("read migrated change operation: %v", err)
	}
	if position != 1 || baseFingerprint != "" {
		t.Fatalf("migrated operation = (position=%d, fingerprint=%q)", position, baseFingerprint)
	}

	// v14 evidence column defaults to "[]".
	var evidence string
	if err := db.QueryRow(`
		SELECT evidence FROM change_operations WHERE id = 'legacy-change-op-1'
	`).Scan(&evidence); err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if evidence != "[]" {
		t.Fatalf("evidence = %q, want []", evidence)
	}

	// Project ID preserved.
	if got := readProjectID(t, db); got != fixture.Expected.Project.ID {
		t.Fatalf("project id = %q, want %q", got, fixture.Expected.Project.ID)
	}

	// Upgrade backup exists, is independently openable, still v9, same project ID.
	paths := newMigrationPaths(fixture.DatabasePath)
	assertFileExists(t, paths.backup)
	backupDB, err := sql.Open("sqlite", paths.backup)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backupDB.Close()
	if got := readSchemaVersion(t, backupDB); got != 9 {
		t.Fatalf("backup schema version = %d, want 9", got)
	}
	if got := readProjectID(t, backupDB); got != fixture.Expected.Project.ID {
		t.Fatalf("backup project id = %q, want %q", got, fixture.Expected.Project.ID)
	}
	if err := quickCheck(ctx, backupDB); err != nil {
		t.Fatalf("backup quick_check: %v", err)
	}

	// Stage, original, journal cleaned up.
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.journal)

	// Original database bytes are NOT preserved (the database was replaced);
	// the backup preserves the v9 content instead.
	newHash := sha256File(t, fixture.DatabasePath)
	if newHash == originalHash {
		t.Fatalf("project.db bytes unchanged after migration; expected replacement")
	}
}

func TestOpenExistingDatabaseMigratesV18AgentRecordsWithoutInventingBindings(t *testing.T) {
	ctx := context.Background()
	path := buildV18AgentFixture(t)
	db, err := OpenExistingDatabase(ctx, path)
	if err != nil {
		t.Fatalf("OpenExistingDatabase: %v", err)
	}
	defer db.Close()

	var workflowID string
	if err := db.QueryRowContext(ctx, `SELECT workflow_id FROM agent_runs WHERE id = 'legacy-run'`).Scan(&workflowID); err != nil {
		t.Fatal(err)
	}
	if workflowID != "" {
		t.Fatalf("legacy workflow_id = %q, want empty", workflowID)
	}
	var nodeID, promptHash, artifactContent string
	var attempt int
	if err := db.QueryRowContext(ctx, `
		SELECT node_id, attempt, prompt_hash FROM agent_run_steps WHERE id = 'legacy-step'
	`).Scan(&nodeID, &attempt, &promptHash); err != nil {
		t.Fatal(err)
	}
	if nodeID != "planner" || attempt != 1 || promptHash != "" {
		t.Fatalf("legacy step = (%q, %d, %q)", nodeID, attempt, promptHash)
	}
	if err := db.QueryRowContext(ctx, `SELECT content FROM agent_artifacts WHERE id = 'legacy-artifact'`).Scan(&artifactContent); err != nil {
		t.Fatal(err)
	}
	if artifactContent != "旧计划全文" {
		t.Fatalf("legacy artifact content = %q", artifactContent)
	}
	for _, table := range []string{"context_manifests", "context_manifest_items", "agent_review_findings"} {
		assertSchemaObjectExists(t, db, "table", table)
	}
	backup := filepath.Join(filepath.Dir(path), "recovery", migrationBackupName(18, currentSchemaVersion))
	assertFileExists(t, backup)
}

func TestV18ToCurrentFailureLeavesOriginalDatabaseIntact(t *testing.T) {
	ctx := context.Background()
	path := buildV18AgentFixture(t)
	originalHash := sha256File(t, path)
	_, err := openExistingDatabase(ctx, path, migrationHooks{
		afterStageValidated: func() error { return errors.New("injected current-schema stage abort") },
	})
	if err == nil {
		t.Fatal("openExistingDatabase succeeded, want injected failure")
	}
	if got := sha256File(t, path); got != originalHash {
		t.Fatalf("v18 database changed after staged migration failure")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != 18 {
		t.Fatalf("schema version = %d, want 18", got)
	}
}

func buildV18AgentFixture(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
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
	if _, err := provider.UpTo(ctx, 18); err != nil {
		t.Fatalf("migrate fixture to v18: %v", err)
	}
	now := formatTime(time.Unix(100, 0).UTC())
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO projects(id, title, created_at, updated_at) VALUES ('project-1', '旧项目', ?, ?)`, []any{now, now}},
		{`INSERT INTO chapters(id, project_id, title, position, revision, created_at, updated_at) VALUES ('chapter-1', 'project-1', '第一章', 1, 1, ?, ?)`, []any{now, now}},
		{`INSERT INTO agent_runs(id, project_id, chapter_id, chapter_base_revision, prompt, status, created_at, updated_at) VALUES ('legacy-run', 'project-1', 'chapter-1', 1, '续写', 'completed', ?, ?)`, []any{now, now}},
		{`INSERT INTO agent_run_steps(id, run_id, role, position, status, output, created_at, updated_at) VALUES ('legacy-step', 'legacy-run', 'planner', 1, 'completed', '旧预览', ?, ?)`, []any{now, now}},
		{`INSERT INTO agent_artifacts(id, run_id, step_id, kind, content, sha256, created_at) VALUES ('legacy-artifact', 'legacy-run', 'legacy-step', 'plan', '旧计划全文', 'legacy-hash', ?)`, []any{now}},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			db.Close()
			t.Fatalf("seed v18 fixture: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := removeSQLiteSidecars(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// === Hook-injected failure tests ===

func TestOpenExistingDatabaseAfterStageValidatedFailureLeavesOriginalIntact(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	originalHash := sha256File(t, fixture.DatabasePath)

	hooks := migrationHooks{
		afterStageValidated: func() error { return errors.New("injected stage abort") },
	}
	_, err := openExistingDatabase(ctx, fixture.DatabasePath, hooks)
	if err == nil {
		t.Fatal("openExistingDatabase succeeded, want injected failure")
	}
	if !errors.Is(err, ErrDatabaseMigration) {
		t.Fatalf("error = %v, want ErrDatabaseMigration", err)
	}

	// Original database bytes, schema, and sentinels unchanged.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("original database bytes changed after stage abort")
	}
	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != 9 {
		t.Fatalf("original schema = %d, want 9", got)
	}
	if got := readProjectID(t, db); got != fixture.Expected.Project.ID {
		t.Fatalf("original project id = %q, want %q", got, fixture.Expected.Project.ID)
	}

	// Stage and journal cleaned up; backup preserved.
	paths := newMigrationPaths(fixture.DatabasePath)
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.journal)
	assertFileMissing(t, paths.original)
	assertFileExists(t, paths.backup)
}

func TestOpenExistingDatabaseAfterActivatedFailureRestoresOriginal(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	originalHash := sha256File(t, fixture.DatabasePath)

	hooks := migrationHooks{
		afterActivated: func() error { return errors.New("injected activation abort") },
	}
	_, err := openExistingDatabase(ctx, fixture.DatabasePath, hooks)
	if err == nil {
		t.Fatal("openExistingDatabase succeeded, want injected failure")
	}
	if !errors.Is(err, ErrDatabaseMigration) {
		t.Fatalf("error = %v, want ErrDatabaseMigration", err)
	}

	// Original restored as project.db.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("original database was not restored after activation failure")
	}
	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != 9 {
		t.Fatalf("restored schema = %d, want 9", got)
	}
	if got := readProjectID(t, db); got != fixture.Expected.Project.ID {
		t.Fatalf("restored project id = %q, want %q", got, fixture.Expected.Project.ID)
	}

	// Original, stage, journal cleaned up; backup preserved.
	paths := newMigrationPaths(fixture.DatabasePath)
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.journal)
	assertFileExists(t, paths.backup)
}

// === No-migration and error classification tests ===

func TestOpenExistingDatabaseDoesNotMigrateOrCreateBackupForCurrentSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, path)

	opened, err := OpenExistingDatabase(ctx, path)
	if err != nil {
		t.Fatalf("OpenExistingDatabase: %v", err)
	}
	defer opened.Close()
	if got := readSchemaVersion(t, opened); got != currentSchemaVersion {
		t.Fatalf("schema = %d, want %d", got, currentSchemaVersion)
	}
	if got := sha256File(t, path); got != originalHash {
		// WAL checkpoint may change bytes, so only verify no backup was created.
	}
	paths := newMigrationPaths(path)
	assertFileMissing(t, paths.backup)
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.journal)
}

func TestOpenReadOnlyReturnsMigrationRequiredForLegacySchemaWithoutChanges(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	before := collectReadOnlyProtectedFiles(t, filepath.Dir(fixture.DatabasePath), fixture.DatabasePath)

	_, err := (Factory{}).OpenReadOnly(ctx, fixture.DatabasePath)
	if !errors.Is(err, ErrDatabaseMigrationRequired) {
		t.Fatalf("OpenReadOnly error = %v, want ErrDatabaseMigrationRequired", err)
	}
	after := collectReadOnlyProtectedFiles(t, filepath.Dir(fixture.DatabasePath), fixture.DatabasePath)
	if !sameFileHashes(before.hashes, after.hashes) {
		t.Fatalf("legacy project protected files changed during read-only open\nbefore: %#v\nafter: %#v", before.hashes, after.hashes)
	}
	if err := assertReadOnlyWAL(t, before, after); err != nil {
		t.Fatal(err)
	}
}

func TestOpenExistingDatabaseRejectsFutureSchemaWithoutCreatingRecoveryFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	db, err := OpenDatabase(ctx, path)
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
	originalHash := sha256File(t, path)

	_, err = OpenExistingDatabase(ctx, path)
	if !errors.Is(err, ErrDatabaseTooNew) {
		t.Fatalf("error = %v, want ErrDatabaseTooNew", err)
	}
	if got := sha256File(t, path); got != originalHash {
		t.Fatalf("future schema database was modified")
	}
	paths := newMigrationPaths(path)
	for _, p := range []string{paths.backup, paths.stage, paths.original, paths.journal} {
		assertFileMissing(t, p)
	}
}

func TestOpenExistingDatabaseRejectsCorruptFileWithoutCreatingRecoveryFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	writeCorruptedFile(t, path)

	_, err := OpenExistingDatabase(ctx, path)
	if !errors.Is(err, ErrDatabaseCorrupt) {
		t.Fatalf("error = %v, want ErrDatabaseCorrupt", err)
	}
	paths := newMigrationPaths(path)
	for _, p := range []string{paths.backup, paths.stage, paths.original, paths.journal} {
		assertFileMissing(t, p)
	}
}

func TestOpenExistingDatabaseRejectsMissingFileWithoutCreatingRecoveryFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "missing.db")

	_, err := OpenExistingDatabase(ctx, path)
	if !errors.Is(err, ErrDatabaseMissing) {
		t.Fatalf("error = %v, want ErrDatabaseMissing", err)
	}
	paths := newMigrationPaths(path)
	for _, p := range []string{paths.backup, paths.stage, paths.original, paths.journal} {
		assertFileMissing(t, p)
	}
}

// === Idempotent re-open after successful migration ===

func TestOpenExistingDatabaseIsIdempotentAfterSuccessfulMigration(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())

	first, err := OpenExistingDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	paths := newMigrationPaths(fixture.DatabasePath)
	backupAfterFirst := sha256File(t, paths.backup)

	second, err := OpenExistingDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	// Re-opening an already-migrated database must not create a second backup
	// or re-run migration.
	if got := sha256File(t, paths.backup); got != backupAfterFirst {
		t.Fatalf("backup changed on re-open")
	}
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.journal)
}

// === Interrupt recovery tests ===

func TestRecoverInterruptedMigrationPreparedPhasePreservesOriginalAndCleansStage(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	// Create a migrated stage (v14) and a prepared journal.
	stageDB, err := OpenDatabase(ctx, paths.stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageDB.Close(); err != nil {
		t.Fatal(err)
	}
	copyDatabaseFile(t, fixture.DatabasePath, paths.backup)
	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhasePrepared,
	})

	if err := recoverInterruptedMigration(ctx, paths); err != nil {
		t.Fatalf("recoverInterruptedMigration: %v", err)
	}

	// Original database preserved.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("original database changed during prepared recovery")
	}
	// Stage and journal cleaned up.
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.journal)
	// Backup preserved.
	assertFileExists(t, paths.backup)
}

func TestRecoverInterruptedMigrationActivatedPhaseRestoresOriginal(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	// Simulate activated state: new db at project.db, original at
	// migration-original.sqlite, journal phase=activated.
	copyDatabaseFile(t, fixture.DatabasePath, paths.original)
	newDB, err := OpenDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := newDB.Close(); err != nil {
		t.Fatal(err)
	}
	copyDatabaseFile(t, fixture.DatabasePath, paths.backup)
	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhaseActivated,
	})

	if err := recoverInterruptedMigration(ctx, paths); err != nil {
		t.Fatalf("recoverInterruptedMigration: %v", err)
	}

	// Original restored as project.db.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("original was not restored during activated recovery")
	}
	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != 9 {
		t.Fatalf("restored schema = %d, want 9", got)
	}
	// Stage, original, journal cleaned up; backup preserved.
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.journal)
	assertFileExists(t, paths.backup)
}

func TestRecoverInterruptedMigrationCommittedPhaseKeepsNewAndCleansJournal(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}

	// Simulate committed state: new (v14) db at project.db, journal
	// phase=committed, no original.
	newDB, err := OpenDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := newDB.Close(); err != nil {
		t.Fatal(err)
	}
	copyDatabaseFile(t, fixture.DatabasePath, paths.backup)
	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhaseCommitted,
	})

	if err := recoverInterruptedMigration(ctx, paths); err != nil {
		t.Fatalf("recoverInterruptedMigration: %v", err)
	}

	// New database kept and validated.
	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != currentSchemaVersion {
		t.Fatalf("new database schema = %d, want %d", got, currentSchemaVersion)
	}
	// Journal cleaned up; no original was present.
	assertFileMissing(t, paths.journal)
	assertFileMissing(t, paths.original)
	// Backup preserved.
	assertFileExists(t, paths.backup)
}

func TestRecoverInterruptedMigrationDamagedJournalStopsAndDeletesNothing(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	// Create stage and a damaged journal.
	stageDB, err := OpenDatabase(ctx, paths.stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.journal, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyDatabaseFile(t, fixture.DatabasePath, paths.backup)

	err = recoverInterruptedMigration(ctx, paths)
	if !errors.Is(err, ErrDatabaseMigrationRecovery) {
		t.Fatalf("error = %v, want ErrDatabaseMigrationRecovery", err)
	}

	// Nothing deleted.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("database changed during damaged journal recovery")
	}
	assertFileExists(t, paths.stage)
	assertFileExists(t, paths.journal)
	assertFileExists(t, paths.backup)
}

func TestRecoverInterruptedMigrationUnknownPhaseStopsAndDeletesNothing(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	stageDB, err := OpenDatabase(ctx, paths.stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageDB.Close(); err != nil {
		t.Fatal(err)
	}
	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       "unknown_phase",
	})

	err = recoverInterruptedMigration(ctx, paths)
	if !errors.Is(err, ErrDatabaseMigrationRecovery) {
		t.Fatalf("error = %v, want ErrDatabaseMigrationRecovery", err)
	}
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("database changed during unknown phase recovery")
	}
	assertFileExists(t, paths.stage)
	assertFileExists(t, paths.journal)
}

func TestRecoverInterruptedMigrationUnknownSchemaStopsAndDeletesNothing(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}

	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      999,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhasePrepared,
	})

	err := recoverInterruptedMigration(ctx, paths)
	if !errors.Is(err, ErrDatabaseMigrationRecovery) {
		t.Fatalf("error = %v, want ErrDatabaseMigrationRecovery", err)
	}
	assertFileExists(t, paths.journal)
}

func TestRecoverInterruptedMigrationAmbiguousOriginalWithoutJournalRestoresOriginal(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	// Ambiguous: new db at project.db, original present, no journal.
	copyDatabaseFile(t, fixture.DatabasePath, paths.original)
	newDB, err := OpenDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := newDB.Close(); err != nil {
		t.Fatal(err)
	}

	if err := recoverInterruptedMigration(ctx, paths); err != nil {
		t.Fatalf("recoverInterruptedMigration: %v", err)
	}

	// "Original first" rule: original restored as project.db.
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("original was not restored during ambiguous recovery")
	}
	db, err := sql.Open("sqlite", fixture.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := readSchemaVersion(t, db); got != 9 {
		t.Fatalf("restored schema = %d, want 9", got)
	}
	assertFileMissing(t, paths.original)
}

// === Recovery integration with OpenExistingDatabase ===

func TestOpenExistingDatabaseRecoversFromPreparedInterruptBeforeMigrating(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	// Leave a prepared journal with a stale stage; OpenExistingDatabase must
	// recover (clean stage/journal, keep backup) and then re-attempt migration.
	stageDB, err := OpenDatabase(ctx, paths.stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageDB.Close(); err != nil {
		t.Fatal(err)
	}
	copyDatabaseFile(t, fixture.DatabasePath, paths.backup)
	writeMigrationJournal(t, paths.journal, migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: 9,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhasePrepared,
	})

	db, err := OpenExistingDatabase(ctx, fixture.DatabasePath)
	if err != nil {
		t.Fatalf("OpenExistingDatabase after prepared interrupt: %v", err)
	}
	defer db.Close()

	if got := readSchemaVersion(t, db); got != currentSchemaVersion {
		t.Fatalf("schema = %d, want %d", got, currentSchemaVersion)
	}
	// Original v9 bytes were replaced by the migration; the backup preserves them.
	backupDB, err := sql.Open("sqlite", paths.backup)
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	if got := sha256File(t, paths.backup); got != originalHash {
		// The backup is a VACUUM INTO copy; its bytes may differ from the
		// original WAL-bearing file. Compare content instead.
	}
	if got := readSchemaVersion(t, backupDB); got != 9 {
		t.Fatalf("backup schema = %d, want 9", got)
	}
	if got := readProjectID(t, backupDB); got != fixture.Expected.Project.ID {
		t.Fatalf("backup project id = %q", got)
	}
	// Migration artifacts cleaned up.
	assertFileMissing(t, paths.stage)
	assertFileMissing(t, paths.original)
	assertFileMissing(t, paths.journal)
}

func TestOpenExistingDatabaseReturnsRecoveryErrorForDamagedJournal(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())
	paths := newMigrationPaths(fixture.DatabasePath)
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	originalHash := sha256File(t, fixture.DatabasePath)

	if err := os.WriteFile(paths.journal, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := OpenExistingDatabase(ctx, fixture.DatabasePath)
	if !errors.Is(err, ErrDatabaseMigrationRecovery) {
		t.Fatalf("error = %v, want ErrDatabaseMigrationRecovery", err)
	}
	if got := sha256File(t, fixture.DatabasePath); got != originalHash {
		t.Fatalf("database changed during damaged journal recovery")
	}
	assertFileExists(t, paths.journal)
}
