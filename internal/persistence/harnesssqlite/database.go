package harnesssqlite

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

// Open 创建独立的 Harness 数据库并执行内嵌迁移，不接触任何项目数据库。
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create harness data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open harness database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	closeWithError := func(cause error) (*sql.DB, error) {
		_ = db.Close()
		return nil, cause
	}
	if err := db.PingContext(ctx); err != nil {
		return closeWithError(fmt.Errorf("ping harness database: %w", err))
	}
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000", "PRAGMA journal_mode = WAL"} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return closeWithError(fmt.Errorf("configure harness database: %w", err))
		}
	}
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return closeWithError(fmt.Errorf("open harness migrations: %w", err))
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		return closeWithError(fmt.Errorf("create harness migration provider: %w", err))
	}
	if _, err := provider.Up(ctx); err != nil {
		return closeWithError(fmt.Errorf("migrate harness database: %w", err))
	}
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return closeWithError(fmt.Errorf("quick_check harness database: %w", err))
	}
	if check != "ok" {
		return closeWithError(fmt.Errorf("harness database quick_check failed: %s", check))
	}
	return db, nil
}
