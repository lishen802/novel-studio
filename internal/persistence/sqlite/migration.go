package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Migration journal phases. See migrationJournal.Phase.
const (
	migrationPhasePrepared  = "prepared"
	migrationPhaseActivated = "activated"
	migrationPhaseCommitted = "committed"
)

// migrationJournalSchema is the JSON schema version for migration.json.
// Unknown schemas must stop recovery rather than guess.
const migrationJournalSchema = 1

var (
	// ErrDatabaseMigration wraps any failure during staged migration. The
	// underlying cause is preserved via errors.Join so callers can still
	// distinguish corrupt/permission/write errors.
	ErrDatabaseMigration = errors.New("database migration failed")
	// ErrDatabaseMigrationRecovery is returned when the recovery routine
	// cannot determine the safe action for an interrupted migration. No file
	// that could be the only copy of the database is deleted in this state.
	ErrDatabaseMigrationRecovery = errors.New("database migration recovery required")
)

// migrationPaths holds the fixed short-path file locations used by the safe
// migration coordinator. All paths are derived from the database location;
// the journal never carries arbitrary paths.
type migrationPaths struct {
	database string // <package>/project.db
	recovery string // <package>/recovery
	journal  string // <package>/recovery/migration.json
	stage    string // <package>/recovery/migration-stage.sqlite
	original string // <package>/recovery/migration-original.sqlite
	backup   string // <package>/recovery/migration-v09-v14.sqlite
}

// newMigrationPaths derives all migration coordinator paths from the database
// location. The backup filename is fixed to the v9 -> v14 path; other source
// versions reuse the same recovery directory but are not expected in R0.
func newMigrationPaths(databasePath string) migrationPaths {
	root := filepath.Dir(databasePath)
	recovery := filepath.Join(root, "recovery")
	return migrationPaths{
		database: databasePath,
		recovery: recovery,
		journal:  filepath.Join(recovery, "migration.json"),
		stage:    filepath.Join(recovery, "migration-stage.sqlite"),
		original: filepath.Join(recovery, "migration-original.sqlite"),
		backup:   filepath.Join(recovery, "migration-v09-v14.sqlite"),
	}
}

// migrationRecoveryRequired reports recovery artifacts without invoking the
// writable migration coordinator. A read-only caller must stop before opening
// SQLite when any artifact that could describe a prepared or activated
// migration is present.
func migrationRecoveryRequired(databasePath string) (bool, error) {
	paths := newMigrationPaths(databasePath)
	for _, path := range []string{paths.journal, paths.stage, paths.original} {
		_, err := os.Stat(path)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("inspect migration recovery artifact: %w", err)
		}
	}
	return false, nil
}

// migrationBackupName returns the fixed backup filename for a from -> to
// migration. R0 only contemplates v9 -> v14; the parameterized form keeps
// future versions from colliding with the v9 backup.
func migrationBackupName(from, to int64) string {
	return fmt.Sprintf("migration-v%02d-v%02d.sqlite", from, to)
}

// migrationJournal is the external transaction journal written atomically to
// migration.json. It carries only schema version, source/target versions and
// phase — never paths, body text, or credentials.
type migrationJournal struct {
	Schema      int    `json:"schema"`
	FromVersion int64  `json:"from_version"`
	ToVersion   int64  `json:"to_version"`
	Phase       string `json:"phase"`
}

// migrationHooks are package-private test injection points. Production code
// calls openExistingDatabase with empty hooks. Forbidden failure injection
// methods include time.Sleep, disk filling, and process killing.
type migrationHooks struct {
	afterStageValidated func() error
	afterActivated      func() error
}

// schemaVersion returns the highest applied Goose migration version, or 0 if
// the goose_db_version table does not exist (a fresh database).
func schemaVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var hasVersionTable int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'goose_db_version'
	`).Scan(&hasVersionTable); err != nil {
		return 0, classifyDatabaseError(err)
	}
	if hasVersionTable == 0 {
		return 0, nil
	}
	var version int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_id), 0)
		FROM goose_db_version
		WHERE is_applied = 1
	`).Scan(&version); err != nil {
		return 0, classifyDatabaseError(err)
	}
	return version, nil
}

// readUniqueProjectID returns the single project ID from the database, or an
// error wrapping ErrProjectCount when the count is not exactly one.
func readUniqueProjectID(ctx context.Context, db *sql.DB) (string, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		return "", classifyDatabaseError(err)
	}
	if count == 0 {
		return "", fmt.Errorf("%w: project table is empty", ErrProjectCount)
	}
	if count > 1 {
		return "", fmt.Errorf("%w: found %d projects", ErrProjectCount, count)
	}
	var id string
	if err := db.QueryRowContext(ctx, "SELECT id FROM projects LIMIT 1").Scan(&id); err != nil {
		return "", classifyDatabaseError(err)
	}
	return id, nil
}

// openExistingDatabase is the package-private entry point that takes test
// hooks. OpenExistingDatabase calls it with empty hooks. It probes the file,
// recovers any interrupted migration, and then either opens the database
// directly (when already at the current schema) or runs the safe staged
// migration coordinator.
func openExistingDatabase(ctx context.Context, path string, hooks migrationHooks) (*sql.DB, error) {
	if err := probeExistingDatabase(path); err != nil {
		return nil, err
	}

	paths := newMigrationPaths(path)
	if err := recoverInterruptedMigration(ctx, paths); err != nil {
		return nil, err
	}

	// Open source without configuring WAL. We only need to inspect schema and
	// produce a VACUUM INTO backup; keeping the source in its original journal
	// mode means the file bytes are unchanged if migration aborts before
	// activation. The WAL PRAGMA is applied only to the reopened (post-activation)
	// database, or when the source is already at the current schema.
	source, err := openSQLite(ctx, path)
	if err != nil {
		return nil, err
	}

	if err := verifySupportedSchema(ctx, source); err != nil {
		_ = source.Close()
		return nil, err
	}

	fromVersion, err := schemaVersion(ctx, source)
	if err != nil {
		_ = source.Close()
		return nil, err
	}

	// Already at current schema: configure runtime PRAGMAs and return.
	if fromVersion == currentSchemaVersion {
		if err := configureRuntimeDatabase(ctx, source); err != nil {
			_ = source.Close()
			return nil, classifyDatabaseError(err)
		}
		if err := quickCheck(ctx, source); err != nil {
			_ = source.Close()
			return nil, err
		}
		return source, nil
	}

	// Legacy schema: run safe staged migration.
	if err := quickCheck(ctx, source); err != nil {
		_ = source.Close()
		return nil, err
	}
	projectID, err := readUniqueProjectID(ctx, source)
	if err != nil {
		_ = source.Close()
		return nil, err
	}

	// Create recovery directory before writing any artifacts.
	if err := os.MkdirAll(paths.recovery, 0o755); err != nil {
		_ = source.Close()
		return nil, fmt.Errorf("create recovery directory: %w", err)
	}

	// Prepare backup (VACUUM INTO or reuse) and stage copy while the source is open.
	if err := prepareMigrationBackup(ctx, source, paths, fromVersion, currentSchemaVersion, projectID); err != nil {
		_ = source.Close()
		return nil, abortMigration(paths, err)
	}

	// Close source before the staged migration and rename sequence so that
	// no SQLite handle references project.db during activation.
	if err := source.Close(); err != nil {
		return nil, abortMigration(paths, fmt.Errorf("close source database: %w", err))
	}
	if err := removeSQLiteSidecars(path); err != nil {
		return nil, abortMigration(paths, err)
	}

	// Migrate and validate the stage.
	if err := migrateAndValidateStage(ctx, paths, projectID); err != nil {
		return nil, abortMigration(paths, err)
	}

	// Test hook: abort after stage validation, before writing the prepared
	// journal. The original database is untouched; only stage and journal
	// (which has not been written yet) need cleanup.
	if hooks.afterStageValidated != nil {
		if err := hooks.afterStageValidated(); err != nil {
			return nil, abortMigration(paths, fmt.Errorf("%w: afterStageValidated hook: %v", ErrDatabaseMigration, err))
		}
	}

	// Write prepared journal: stage is migrated and validated, original is
	// still in place at project.db.
	journal := migrationJournal{
		Schema:      migrationJournalSchema,
		FromVersion: fromVersion,
		ToVersion:   currentSchemaVersion,
		Phase:       migrationPhasePrepared,
	}
	if err := writeJournalAtomic(paths, journal); err != nil {
		return nil, abortMigration(paths, err)
	}

	// Activate: rename sequence, reopen, validate.
	db, err := activateMigratedDatabase(ctx, paths, journal, projectID, hooks)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// recoverInterruptedMigration inspects the recovery directory for an
// interrupted migration and restores a safe state. When the journal or file
// combination cannot be interpreted safely it returns
// ErrDatabaseMigrationRecovery without deleting any file that could be the
// only copy of the database.
func recoverInterruptedMigration(ctx context.Context, paths migrationPaths) error {
	_, err := os.Stat(paths.recovery)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect recovery directory: %w", err)
	}

	journalData, journalErr := os.ReadFile(paths.journal)
	if errors.Is(journalErr, fs.ErrNotExist) {
		// No journal. If migration-original.sqlite exists, treat it as the
		// authoritative copy and restore it (ambiguous interrupt).
		if _, statErr := os.Stat(paths.original); statErr == nil {
			return restoreOriginalDatabase(paths)
		}
		// No journal and no original: nothing to recover.
		return nil
	}
	if journalErr != nil {
		return fmt.Errorf("read migration journal: %w", journalErr)
	}

	var journal migrationJournal
	if err := json.Unmarshal(journalData, &journal); err != nil {
		return fmt.Errorf("%w: parse migration journal: %v", ErrDatabaseMigrationRecovery, err)
	}
	if journal.Schema != migrationJournalSchema {
		return fmt.Errorf("%w: unknown journal schema %d", ErrDatabaseMigrationRecovery, journal.Schema)
	}

	switch journal.Phase {
	case migrationPhasePrepared:
		// Original is still at project.db; clean stage and journal, keep backup.
		if err := removeIfExists(paths.stage); err != nil {
			return fmt.Errorf("clean prepared stage: %w", err)
		}
		if err := removeIfExists(paths.journal); err != nil {
			return fmt.Errorf("clean prepared journal: %w", err)
		}
		return nil

	case migrationPhaseActivated:
		// Original is at migration-original.sqlite; the new (failed) db is at
		// project.db. Restore the original.
		return restoreOriginalDatabase(paths)

	case migrationPhaseCommitted:
		// New db is at project.db; original was deleted. Validate and clean
		// journal, keep backup.
		return recoverCommittedState(ctx, paths)

	default:
		return fmt.Errorf("%w: unknown journal phase %q", ErrDatabaseMigrationRecovery, journal.Phase)
	}
}

// restoreOriginalDatabase deletes the current project.db (the failed new
// database), removes SQLite sidecars, renames migration-original.sqlite back
// to project.db, syncs the directory, and cleans up the stage and journal.
// The backup is preserved.
func restoreOriginalDatabase(paths migrationPaths) error {
	if err := os.Remove(paths.database); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove activated database: %w", err)
	}
	if err := removeSQLiteSidecars(paths.database); err != nil {
		return err
	}
	if err := os.Rename(paths.original, paths.database); err != nil {
		return fmt.Errorf("restore original database: %w", err)
	}
	if err := syncDirectory(paths.database); err != nil {
		return err
	}
	if err := removeIfExists(paths.stage); err != nil {
		return fmt.Errorf("clean restored stage: %w", err)
	}
	if err := removeIfExists(paths.journal); err != nil {
		return fmt.Errorf("clean restored journal: %w", err)
	}
	return nil
}

// recoverCommittedState validates that the committed new database is at the
// current schema and passes quick_check, then cleans the journal. The backup
// is preserved.
func recoverCommittedState(ctx context.Context, paths migrationPaths) error {
	db, err := openSQLite(ctx, paths.database)
	if err != nil {
		return fmt.Errorf("%w: open committed database: %v", ErrDatabaseMigrationRecovery, err)
	}
	defer db.Close()

	version, err := schemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("%w: read committed schema: %v", ErrDatabaseMigrationRecovery, err)
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("%w: committed schema %d, want %d", ErrDatabaseMigrationRecovery, version, currentSchemaVersion)
	}
	if err := quickCheck(ctx, db); err != nil {
		return fmt.Errorf("%w: committed quick_check: %v", ErrDatabaseMigrationRecovery, err)
	}

	if err := removeIfExists(paths.original); err != nil {
		return fmt.Errorf("clean committed original: %w", err)
	}
	if err := removeIfExists(paths.journal); err != nil {
		return fmt.Errorf("clean committed journal: %w", err)
	}
	return nil
}

// prepareMigrationBackup creates a consistency backup of the source database
// using SQLite VACUUM INTO, verifies it is independently openable and at the
// expected source schema, then copies it to the stage file with O_CREATE|O_EXCL.
// If a backup from a recovered interrupted migration already exists, it is
// validated and reused rather than overwritten. The source is read-only during
// this operation.
func prepareMigrationBackup(ctx context.Context, source *sql.DB, paths migrationPaths, from, to int64, projectID string) error {
	// Remove any stale stage from a prior attempt so the O_EXCL copy below
	// cannot collide with a leftover file.
	if err := removeIfExists(paths.stage); err != nil {
		return fmt.Errorf("%w: clean stale stage: %v", ErrDatabaseMigration, err)
	}

	// If a backup already exists from a recovered interrupted migration,
	// validate and reuse it. The plan requires not overwriting an existing
	// backup; if it is invalid we stop with a recovery error so the only copy
	// is not destroyed.
	if _, err := os.Stat(paths.backup); err == nil {
		if err := validateBackupForReuse(ctx, paths.backup, from, projectID); err != nil {
			return fmt.Errorf("%w: existing backup is invalid: %v", ErrDatabaseMigrationRecovery, err)
		}
		if err := copyFileWithSync(paths.backup, paths.stage); err != nil {
			return fmt.Errorf("%w: copy existing backup to stage: %v", ErrDatabaseMigration, err)
		}
		return nil
	}

	// VACUUM INTO writes a single-file consistent copy of the source database
	// (including any committed WAL content) to the backup path. The path is
	// escaped by doubling single quotes; all paths are derived from the
	// database location and never carry user-supplied content.
	escaped := strings.ReplaceAll(paths.backup, "'", "''")
	if _, err := source.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return fmt.Errorf("%w: vacuum into backup: %v", ErrDatabaseMigration, classifyDatabaseError(err))
	}

	// Verify the fresh backup is independently openable, at the source version,
	// and passes quick_check before copying it to the stage.
	if err := validateBackupForReuse(ctx, paths.backup, from, projectID); err != nil {
		return fmt.Errorf("%w: %v", ErrDatabaseMigration, err)
	}

	// Copy backup to stage with O_CREATE|O_EXCL and fsync.
	if err := copyFileWithSync(paths.backup, paths.stage); err != nil {
		return fmt.Errorf("%w: copy backup to stage: %v", ErrDatabaseMigration, err)
	}

	return nil
}

// validateBackupForReuse opens a backup database and confirms it is at the
// expected source schema, passes quick_check, and (when projectID is
// non-empty) contains the same project ID as the source. Used both for
// validating a fresh VACUUM INTO backup and for validating an existing backup
// before reuse.
func validateBackupForReuse(ctx context.Context, backupPath string, expectedVersion int64, expectedProjectID string) error {
	backup, err := openSQLite(ctx, backupPath)
	if err != nil {
		return fmt.Errorf("open backup for verification: %v", err)
	}
	defer backup.Close()

	version, err := schemaVersion(ctx, backup)
	if err != nil {
		return fmt.Errorf("read backup schema: %v", err)
	}
	if version != expectedVersion {
		return fmt.Errorf("backup schema %d, want %d", version, expectedVersion)
	}
	if err := quickCheck(ctx, backup); err != nil {
		return fmt.Errorf("backup quick_check: %v", err)
	}
	if expectedProjectID != "" {
		var id string
		if err := backup.QueryRowContext(ctx, "SELECT id FROM projects LIMIT 1").Scan(&id); err != nil {
			return classifyDatabaseError(err)
		}
		if id != expectedProjectID {
			return fmt.Errorf("backup project id %q, want %q", id, expectedProjectID)
		}
	}
	return nil
}

// migrateAndValidateStage opens the stage database, configures runtime
// PRAGMAs, runs all embedded Goose migrations, validates the schema version,
// quick_check, and unique project ID, then closes it and removes SQLite
// sidecars so the stage is a self-contained single-file database ready for
// the atomic rename sequence.
func migrateAndValidateStage(ctx context.Context, paths migrationPaths, projectID string) error {
	stage, err := openSQLite(ctx, paths.stage)
	if err != nil {
		return fmt.Errorf("%w: open stage: %v", ErrDatabaseMigration, err)
	}
	closeStage := func(err error) error {
		_ = stage.Close()
		return err
	}

	if err := configureRuntimeDatabase(ctx, stage); err != nil {
		return closeStage(fmt.Errorf("%w: configure stage: %v", ErrDatabaseMigration, err))
	}
	if err := migrateDatabase(ctx, stage); err != nil {
		return closeStage(fmt.Errorf("%w: migrate stage: %v", ErrDatabaseMigration, err))
	}
	if err := validateDatabase(ctx, stage, projectID); err != nil {
		return closeStage(fmt.Errorf("%w: validate stage: %v", ErrDatabaseMigration, err))
	}

	if err := stage.Close(); err != nil {
		return fmt.Errorf("%w: close stage: %v", ErrDatabaseMigration, err)
	}

	// Remove stage sidecars after close so the rename sequence copies only
	// the single stage file.
	if err := removeSQLiteSidecars(paths.stage); err != nil {
		return fmt.Errorf("%w: clean stage sidecars: %v", ErrDatabaseMigration, err)
	}

	return nil
}

// activateMigratedDatabase performs the atomic rename sequence that replaces
// project.db with the validated stage, writes the activated journal, reopens
// and validates the new database, writes the committed journal, and cleans
// up operation artifacts (original + journal). The backup is preserved.
//
// If any step fails before the original is deleted, rollbackActivation
// restores the original database and returns the cause wrapped with
// ErrDatabaseMigration.
func activateMigratedDatabase(
	ctx context.Context,
	paths migrationPaths,
	journal migrationJournal,
	projectID string,
	hooks migrationHooks,
) (*sql.DB, error) {
	// Step 1: rename project.db -> migration-original.sqlite to preserve the
	// original before replacing it.
	if err := os.Rename(paths.database, paths.original); err != nil {
		return nil, fmt.Errorf("%w: preserve original database: %v", ErrDatabaseMigration, err)
	}
	if err := syncDirectory(paths.database); err != nil {
		return nil, rollbackActivation(paths, fmt.Errorf("%w: sync original rename: %v", ErrDatabaseMigration, err))
	}

	// Step 2: rename migration-stage.sqlite -> project.db to activate the
	// migrated database.
	if err := os.Rename(paths.stage, paths.database); err != nil {
		// Rollback: rename original back to project.db.
		_ = os.Rename(paths.original, paths.database)
		return nil, fmt.Errorf("%w: activate staged database: %v", ErrDatabaseMigration, err)
	}
	if err := syncDirectory(paths.database); err != nil {
		return nil, rollbackActivation(paths, fmt.Errorf("%w: sync stage activation: %v", ErrDatabaseMigration, err))
	}

	// Step 3: write activated journal. The original is now at
	// migration-original.sqlite and the new db is at project.db.
	journal.Phase = migrationPhaseActivated
	if err := writeJournalAtomic(paths, journal); err != nil {
		return nil, rollbackActivation(paths, fmt.Errorf("%w: write activated journal: %v", ErrDatabaseMigration, err))
	}

	// Test hook: abort after activation, before reopening. The original is
	// still at migration-original.sqlite and can be restored.
	if hooks.afterActivated != nil {
		if err := hooks.afterActivated(); err != nil {
			return nil, rollbackActivation(paths, fmt.Errorf("%w: afterActivated hook: %v", ErrDatabaseMigration, err))
		}
	}

	// Step 4: reopen the new project.db and validate schema, quick_check,
	// and project ID.
	db, err := openRuntimeDatabase(ctx, paths.database)
	if err != nil {
		return nil, rollbackActivation(paths, fmt.Errorf("%w: reopen activated database: %v", ErrDatabaseMigration, err))
	}
	if err := validateDatabase(ctx, db, projectID); err != nil {
		_ = db.Close()
		return nil, rollbackActivation(paths, fmt.Errorf("%w: validate activated database: %v", ErrDatabaseMigration, err))
	}

	// Step 5: write committed journal. The new db is validated and live.
	journal.Phase = migrationPhaseCommitted
	if err := writeJournalAtomic(paths, journal); err != nil {
		_ = db.Close()
		return nil, rollbackActivation(paths, fmt.Errorf("%w: write committed journal: %v", ErrDatabaseMigration, err))
	}

	// Step 6: clean up operation artifacts (original + journal). Keep the
	// backup as the pre-upgrade recovery copy.
	if err := removeIfExists(paths.original); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: remove original after commit: %v", ErrDatabaseMigration, err)
	}
	if err := removeIfExists(paths.journal); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: remove journal after commit: %v", ErrDatabaseMigration, err)
	}

	return db, nil
}

// rollbackActivation restores the original database after a failed activation.
// It deletes the new (failed) project.db, removes SQLite sidecars, renames
// migration-original.sqlite back to project.db, syncs the directory, and
// cleans up the stage and journal. The backup is preserved. The cause error is
// returned wrapped with ErrDatabaseMigration so callers know migration failed
// even though the original was recovered.
func rollbackActivation(paths migrationPaths, cause error) error {
	var recoveryErr error
	if err := os.Remove(paths.database); err != nil && !errors.Is(err, fs.ErrNotExist) {
		recoveryErr = fmt.Errorf("remove failed database: %w", err)
	}
	if err := removeSQLiteSidecars(paths.database); err != nil && recoveryErr == nil {
		recoveryErr = err
	}
	if err := os.Rename(paths.original, paths.database); err != nil && recoveryErr == nil {
		recoveryErr = fmt.Errorf("restore original database: %w", err)
	}
	if err := syncDirectory(paths.database); err != nil && recoveryErr == nil {
		recoveryErr = fmt.Errorf("sync restored database: %w", err)
	}
	_ = removeIfExists(paths.stage)
	_ = removeIfExists(paths.journal)

	if recoveryErr != nil {
		return fmt.Errorf("%w: %v (recovery also failed: %v)", ErrDatabaseMigration, cause, recoveryErr)
	}
	return cause
}

// abortMigration cleans up the stage and journal when migration fails before
// activation (stage migration failure, stage validation failure, or the
// afterStageValidated test hook). The original database is untouched and the
// backup is preserved. The cause error is returned unchanged.
func abortMigration(paths migrationPaths, cause error) error {
	_ = removeIfExists(paths.stage)
	_ = removeIfExists(paths.journal)
	return cause
}

// writeJournalAtomic writes the migration journal to a temporary file, fsyncs
// it, renames it to migration.json, and syncs the directory. The journal is
// never partially visible: either the old journal or the new one is on disk.
func writeJournalAtomic(paths migrationPaths, journal migrationJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("marshal migration journal: %w", err)
	}
	tempPath := paths.journal + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create journal temp file: %w", err)
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write migration journal: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync migration journal: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close migration journal: %w", err)
	}
	if err := os.Rename(tempPath, paths.journal); err != nil {
		return fmt.Errorf("commit migration journal: %w", err)
	}
	removeTemp = false
	return syncDirectory(paths.journal)
}

// copyFileWithSync copies src to dst with O_CREATE|O_EXCL (fails if dst
// already exists), fsyncs the destination, and closes it. Used to copy the
// VACUUM INTO backup to the stage file.
func copyFileWithSync(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	removeTarget := true
	defer func() {
		_ = target.Close()
		if removeTarget {
			_ = os.Remove(dst)
		}
	}()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	removeTarget = false
	return nil
}

// removeSQLiteSidecars removes the -wal and -shm sidecar files that SQLite
// creates in WAL mode. They are removed after a database is closed so that
// rename operations copy only the single database file.
func removeSQLiteSidecars(databasePath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(databasePath + suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove SQLite sidecar %s: %w", suffix, err)
		}
	}
	return nil
}

// removeIfExists removes a file, returning nil if it does not exist.
func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
