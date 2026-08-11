package projectfs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/lishen802/novel-studio/internal/application/ports"
	_ "modernc.org/sqlite"
)

const windowsCompatiblePathLimit = 240

const longestProjectRelativePath = "recovery/.r1-4-probe-0123456789abcdef/replace-target-0123456789abcdef.sqlite"

func validateProjectPath(root string) error {
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	if len(utf16.Encode([]rune(filepath.Join(absolute, longestProjectRelativePath)))) > windowsCompatiblePathLimit {
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPathTooLong}
	}
	return nil
}

func (Manager) ValidateCreatePath(parentDirectory, projectTitle string) error {
	name, err := portableProjectName(projectTitle)
	if err != nil {
		return err
	}
	return validateProjectPath(filepath.Join(parentDirectory, name+packageExtension))
}

func (manager Manager) ValidateRecoveryCopyPath(source ports.ProjectPackage, parentDirectory string) error {
	layout, err := manager.newRecoveryCopyLayout(source, parentDirectory)
	if err != nil {
		return err
	}
	if err := validateProjectPath(layout.temporary); err != nil {
		return err
	}
	return validateProjectPath(layout.final)
}

func validateRecoveryCopyLayout(layout recoveryCopyLayout) error {
	if err := validateProjectPath(layout.temporary); err != nil {
		return err
	}
	return validateProjectPath(layout.final)
}

func (Manager) ClassifyRuntimeError(err error) *ports.ProjectFilesystemFailure {
	return ClassifyFilesystemError(err)
}

func ClassifyFilesystemError(err error) *ports.ProjectFilesystemFailure {
	if err == nil {
		return nil
	}
	var failure *ports.ProjectFilesystemFailure
	if errors.As(err, &failure) {
		return failure
	}
	if errors.Is(err, os.ErrPermission) {
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemReadOnly}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "database is locked"), strings.Contains(message, "database table is locked"):
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemBusy}
	case strings.Contains(message, "readonly"), strings.Contains(message, "read-only"):
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemReadOnly}
	case strings.Contains(message, "disk is full"), strings.Contains(message, "disk full"):
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemDiskFull}
	case strings.Contains(message, "disk i/o"), strings.Contains(message, "ioerr"):
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemIO}
	}
	return nil
}

func (Manager) PreflightEditable(pkg ports.ProjectPackage) (ports.ProjectFilesystemHealth, error) {
	if err := validateProjectPath(pkg.Root); err != nil {
		return ports.NormalProjectFilesystemHealth(), err
	}
	probeRoot, err := os.MkdirTemp(pkg.Root, ".r1-4-probe-")
	if err != nil {
		return ports.NormalProjectFilesystemHealth(), preflightFailure(err)
	}
	defer os.RemoveAll(probeRoot)
	if err := probeDirectory(probeRoot); err != nil {
		return ports.NormalProjectFilesystemHealth(), preflightFailure(err)
	}
	if err := probeSQLite(filepath.Join(probeRoot, "probe.sqlite")); err != nil {
		return ports.NormalProjectFilesystemHealth(), preflightFailure(err)
	}
	if err := probeAtomicReplacement(probeRoot); err != nil {
		return ports.NormalProjectFilesystemHealth(), preflightFailure(err)
	}
	return ports.NormalProjectFilesystemHealth(), nil
}

func preflightFailure(err error) error {
	return errors.Join(&ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPreflight}, err)
}

func probeDirectory(root string) error {
	path := filepath.Join(root, "directory-probe")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("probe"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

func probeSQLite(path string) error {
	ctx := context.Background()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE probe(value TEXT NOT NULL)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO probe(value) VALUES ('kept')"); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO probe(value) VALUES ('rolled-back')"); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Rollback(); err != nil {
		return err
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM probe").Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("probe transaction count = %d", count)
	}
	_, err = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func probeAtomicReplacement(root string) error {
	source := filepath.Join(root, "replace-source")
	target := filepath.Join(root, "replace-target")
	for _, item := range []struct{ path, body string }{{source, "source"}, {target, "target"}} {
		file, err := os.OpenFile(item.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := file.WriteString(item.body); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	if err := os.Rename(source, target); err != nil {
		return err
	}
	body, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if string(body) != "source" {
		return fmt.Errorf("replacement body = %q", body)
	}
	return nil
}
