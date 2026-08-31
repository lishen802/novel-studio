package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrDatabaseMissing           = errors.New("database does not exist")
	ErrDatabasePermission        = errors.New("database permission denied")
	ErrDatabaseCorrupt           = errors.New("database is corrupt")
	ErrDatabaseMigrationRequired = errors.New("database migration required")
	ErrDatabaseTooNew            = errors.New("database schema is newer than this application")
	ErrDatabaseWrite             = errors.New("database write failed")
)

const (
	currentSchemaVersion int64 = 21
	sqliteHeader               = "SQLite format 3\x00"
)

func probeExistingDatabase(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrDatabaseMissing, path)
	}
	if err != nil {
		return classifyDatabaseError(err)
	}
	defer file.Close()

	header := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("%w: read SQLite header: %v", ErrDatabaseCorrupt, err)
	}
	if !bytes.Equal(header, []byte(sqliteHeader)) {
		return fmt.Errorf("%w: invalid SQLite header", ErrDatabaseCorrupt)
	}
	return nil
}

func verifySupportedSchema(ctx context.Context, db *sql.DB) error {
	var hasVersionTable int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'goose_db_version'
	`).Scan(&hasVersionTable); err != nil {
		return classifyDatabaseError(err)
	}
	if hasVersionTable == 0 {
		return nil
	}

	var version int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_id), 0)
		FROM goose_db_version
		WHERE is_applied = 1
	`).Scan(&version); err != nil {
		return classifyDatabaseError(err)
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("%w: found version %d, support through %d", ErrDatabaseTooNew, version, currentSchemaVersion)
	}
	return nil
}

func quickCheck(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return classifyDatabaseError(err)
	}
	defer rows.Close()

	checked := false
	for rows.Next() {
		checked = true
		var result string
		if err := rows.Scan(&result); err != nil {
			return classifyDatabaseError(err)
		}
		if result != "ok" {
			return fmt.Errorf("%w: quick_check failed", ErrDatabaseCorrupt)
		}
	}
	if err := rows.Err(); err != nil {
		return classifyDatabaseError(err)
	}
	if !checked {
		return fmt.Errorf("%w: quick_check returned no result", ErrDatabaseCorrupt)
	}
	return nil
}
