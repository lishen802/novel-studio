package appsqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open creates or opens the application settings database at path, configures
// runtime PRAGMAs, runs all embedded migrations, and verifies integrity. It
// creates the parent directory if it does not exist.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create application data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open settings database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	closeWithError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		return closeWithError(fmt.Errorf("ping settings database: %w", err))
	}

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return closeWithError(fmt.Errorf("configure settings database: %w", err))
		}
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
		return closeWithError(fmt.Errorf("migrate settings database: %w", err))
	}

	var quickCheckResult string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&quickCheckResult); err != nil {
		return closeWithError(fmt.Errorf("quick_check settings database: %w", err))
	}
	if quickCheckResult != "ok" {
		return closeWithError(fmt.Errorf("settings database quick_check failed: %s", quickCheckResult))
	}

	return db, nil
}

// Close releases the database handle. It is safe to call on a nil or already
// closed database.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
