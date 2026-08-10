package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	sqlitedriver "modernc.org/sqlite"
)

type sqliteBackuper interface {
	NewBackup(string) (*sqlitedriver.Backup, error)
}

// backupDatabase uses SQLite's online backup API. Unlike VACUUM INTO, it is
// valid for a source opened mode=ro with query_only enabled, and it copies the
// source's committed WAL state without opening a writable source connection.
func backupDatabase(ctx context.Context, db *sql.DB, targetPath string) error {
	cleanup, err := reserveBackupTarget(targetPath)
	if err != nil {
		return fmt.Errorf("reserve database snapshot target: %w", err)
	}
	completed := false
	defer func() {
		if !completed {
			cleanup()
		}
	}()

	targetURI, err := sqliteFileURI(targetPath, "")
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return databaseOperationError("acquire database snapshot connection", err)
	}
	defer conn.Close()

	err = conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(sqliteBackuper)
		if !ok {
			return fmt.Errorf("SQLite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(targetURI)
		if err != nil {
			return err
		}
		more, stepErr := backup.Step(-1)
		finishErr := backup.Finish()
		if stepErr != nil {
			return stepErr
		}
		if finishErr != nil {
			return finishErr
		}
		if more {
			return fmt.Errorf("online backup did not complete")
		}
		return nil
	})
	if err != nil {
		return databaseOperationError("create consistent database snapshot", err)
	}
	completed = true
	return nil
}

// reserveBackupTarget closes the TOCTOU window between Store.Backup's
// user-facing existence check and SQLite opening its destination RW|CREATE.
// The cleanup closure is safe only for a failed backup that owns this exact
// reservation.
func reserveBackupTarget(targetPath string) (func(), error) {
	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	return func() {
		_ = os.Remove(targetPath)
	}, nil
}
