package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pressly/goose/v3"
	sqlitedriver "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrationsFS returns the versioned Goose SQL migrations that bring a SQLite
// database up to the current application schema. Production code should keep
// calling OpenDatabase or OpenExistingDatabase; this export exists so that
// testsupport helpers can build deterministic legacy fixtures from the same
// canonical migration source instead of duplicating or rediscovering it.
func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		// The sub-FS layout is fixed at compile time; if this ever fails it
		// indicates the embed directive was edited incorrectly.
		panic(fmt.Sprintf("sqlite: invalid embedded migrations layout: %v", err))
	}
	return sub
}

func OpenDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := ensureDatabaseFile(path); err != nil {
		return nil, err
	}
	return openDatabase(ctx, path)
}

func OpenExistingDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := probeExistingDatabase(path); err != nil {
		return nil, err
	}
	return openDatabase(ctx, path)
}

func openDatabase(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, classifyDatabaseError(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	closeWithError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, classifyDatabaseError(err)
	}

	if err := db.PingContext(ctx); err != nil {
		return closeWithError(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return closeWithError(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return closeWithError(err)
	}
	if err := verifySupportedSchema(ctx, db); err != nil {
		return closeWithError(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return closeWithError(err)
	}

	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return closeWithError(fmt.Errorf("open embedded migrations: %w", err))
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		return closeWithError(fmt.Errorf("create migration provider: %w", err))
	}
	if _, err := provider.Up(ctx); err != nil {
		return closeWithError(fmt.Errorf("migrate database: %w", err))
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
