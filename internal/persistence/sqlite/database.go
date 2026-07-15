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

var ErrDatabaseCorrupt = errors.New("database is corrupt")

//go:embed migrations/*.sql
var migrations embed.FS

func OpenDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := ensureDatabaseFile(path); err != nil {
		return nil, err
	}

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
	return err
}
