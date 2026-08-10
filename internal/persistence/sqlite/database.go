package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pressly/goose/v3"
	sqlitedriver "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// OpenDatabase creates or opens a database at path, configures runtime
// PRAGMAs, runs all embedded migrations, and verifies integrity. It is the
// entry point for new project databases that may safely migrate in place
// because no author data exists yet.
func OpenDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := ensureDatabaseFile(path); err != nil {
		return nil, err
	}
	return openDatabase(ctx, path)
}

// OpenExistingDatabase opens a database that may contain author data. It
// first probes the file, recovers any interrupted migration, and then either
// opens it directly (when already at the current schema) or runs the safe
// staged migration coordinator. The returned database is always at the
// current schema version with runtime PRAGMAs configured.
func OpenExistingDatabase(ctx context.Context, path string) (*sql.DB, error) {
	return openExistingDatabase(ctx, path, migrationHooks{})
}

// OpenReadOnlyDatabase opens an existing, current-schema database without
// changing canonical or business project data. It deliberately avoids
// migration recovery, Goose, WAL configuration, and checkpointing.
func OpenReadOnlyDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := probeExistingDatabase(path); err != nil {
		return nil, err
	}

	uri, err := readOnlySQLiteURI(path)
	if err != nil {
		return nil, err
	}
	db, err := openSQLite(ctx, uri)
	if err != nil {
		return nil, err
	}
	closeWithError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, classifyDatabaseError(err)
	}
	if err := configureReadOnlyDatabase(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := validateReadOnlyDatabase(ctx, db); err != nil {
		return closeWithError(err)
	}
	return db, nil
}

// openDatabase opens a database and runs migrations directly. It is shared by
// OpenDatabase (for new databases) and tests that build staged fixtures.
func openDatabase(ctx context.Context, path string) (*sql.DB, error) {
	db, err := openSQLite(ctx, path)
	if err != nil {
		return nil, err
	}
	closeWithError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, classifyDatabaseError(err)
	}
	if err := configureRuntimeDatabase(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := verifySupportedSchema(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := migrateDatabase(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := quickCheck(ctx, db); err != nil {
		return closeWithError(err)
	}
	return db, nil
}

// openSQLite opens a SQLite database with a single-connection pool and
// verifies it is reachable. It does not configure runtime PRAGMAs or run
// migrations, so callers that need to inspect schema version without
// modifying the database (such as the migration coordinator) can do so.
func openSQLite(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, classifyDatabaseError(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, classifyDatabaseError(err)
	}
	return db, nil
}

func readOnlySQLiteURI(path string) (string, error) {
	return sqliteFileURI(path, "mode=ro")
}

func sqliteFileURI(path, rawQuery string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	uriPath := filepath.ToSlash(absolutePath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return (&url.URL{
		Scheme:   "file",
		Path:     uriPath,
		RawQuery: rawQuery,
	}).String(), nil
}

// configureRuntimeDatabase sets the PRAGMAs every open application database
// must have: foreign keys, busy timeout, and WAL journal mode. These PRAGMAs
// are safe to apply to a database that is already at the current schema.
func configureRuntimeDatabase(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return classifyDatabaseError(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return classifyDatabaseError(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

// configureReadOnlyDatabase applies only per-connection settings that do not
// modify the project database. query_only is the SQLite-side write barrier.
func configureReadOnlyDatabase(ctx context.Context, db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA query_only = ON",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return classifyDatabaseError(err)
		}
	}
	return nil
}

func validateReadOnlyDatabase(ctx context.Context, db *sql.DB) error {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	switch {
	case version < currentSchemaVersion:
		return fmt.Errorf("%w: found version %d, require %d", ErrDatabaseMigrationRequired, version, currentSchemaVersion)
	case version > currentSchemaVersion:
		return fmt.Errorf("%w: found version %d, support through %d", ErrDatabaseTooNew, version, currentSchemaVersion)
	}
	if err := quickCheck(ctx, db); err != nil {
		return err
	}
	_, err = readUniqueProjectID(ctx, db)
	return err
}

// migrateDatabase runs all embedded Goose migrations to the current version.
// It is a thin wrapper around the Goose provider so the migration
// coordinator can run migrations on a staged copy.
func migrateDatabase(ctx context.Context, db *sql.DB) error {
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

// validateDatabase confirms an open database is at the current schema
// version, passes quick_check, and (when projectID is non-empty) contains
// exactly one project with that ID. It does not run migrations.
func validateDatabase(ctx context.Context, db *sql.DB, projectID string) error {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("%w: schema version %d, want %d", ErrDatabaseCorrupt, version, currentSchemaVersion)
	}
	if err := quickCheck(ctx, db); err != nil {
		return err
	}
	if projectID != "" {
		var id string
		if err := db.QueryRowContext(ctx, "SELECT id FROM projects LIMIT 1").Scan(&id); err != nil {
			return classifyDatabaseError(err)
		}
		if id != projectID {
			return fmt.Errorf("%w: project id %q, want %q", ErrDatabaseCorrupt, id, projectID)
		}
	}
	return nil
}

// openRuntimeDatabase opens a database that is already at the current schema
// version. It configures runtime PRAGMAs and verifies schema version and
// quick_check. It never runs migrations or creates migration artifacts.
func openRuntimeDatabase(ctx context.Context, path string) (*sql.DB, error) {
	db, err := openSQLite(ctx, path)
	if err != nil {
		return nil, err
	}
	closeWithError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, classifyDatabaseError(err)
	}
	if err := configureRuntimeDatabase(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := verifySupportedSchema(ctx, db); err != nil {
		return closeWithError(err)
	}
	if err := quickCheck(ctx, db); err != nil {
		return closeWithError(err)
	}
	return db, nil
}

func ensureDatabaseFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open database file: %w", err)
	}
	return file.Close()
}

func classifyDatabaseError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %v", ErrDatabaseMissing, err)
	}
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w: %v", ErrDatabasePermission, err)
	}

	var sqliteErr *sqlitedriver.Error
	if errors.As(err, &sqliteErr) {
		const (
			sqliteCorrupt = 11
			sqliteNotADB  = 26
		)
		primaryCode := sqliteErr.Code() & 0xff
		if primaryCode == sqliteCorrupt || primaryCode == sqliteNotADB {
			return fmt.Errorf("%w: %v", ErrDatabaseCorrupt, err)
		}
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not a database") || strings.Contains(message, "database disk image is malformed") {
		return fmt.Errorf("%w: %v", ErrDatabaseCorrupt, err)
	}
	if strings.Contains(message, "permission denied") || strings.Contains(message, "readonly") || strings.Contains(message, "read-only") {
		return fmt.Errorf("%w: %v", ErrDatabasePermission, err)
	}
	if strings.Contains(message, "disk is full") || strings.Contains(message, "disk i/o") || strings.Contains(message, "ioerr") {
		return fmt.Errorf("%w: %v", ErrDatabaseWrite, err)
	}
	return err
}

func databaseOperationError(operation string, err error) error {
	return fmt.Errorf("%s: %w", operation, classifyDatabaseError(err))
}
