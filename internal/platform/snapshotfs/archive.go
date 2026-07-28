package snapshotfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

const metadataVersion = 1

var ErrSnapshotMissing = errors.New("snapshot does not exist")

type Archive struct{}

type snapshotMetadata struct {
	Version   int                         `json:"version"`
	ID        string                      `json:"id"`
	ProjectID string                      `json:"projectId"`
	Name      string                      `json:"name"`
	Kind      revisiondomain.SnapshotKind `json:"kind"`
	CreatedAt time.Time                   `json:"createdAt"`
}

func (Archive) Prepare(directory string, id string) (string, error) {
	if err := validateSnapshotID(id); err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot archive: %w", err)
	}
	path := filepath.Join(directory, id+".sqlite")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("snapshot already exists: %s", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect snapshot target: %w", err)
	}
	return path, nil
}

func (Archive) Record(directory string, snapshot revisiondomain.Snapshot) error {
	if err := validateSnapshotID(snapshot.ID); err != nil {
		return err
	}
	databasePath := filepath.Join(directory, snapshot.ID+".sqlite")
	if info, err := os.Stat(databasePath); err != nil {
		return fmt.Errorf("inspect snapshot database: %w", err)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot database is not a regular file: %s", databasePath)
	}

	body, err := json.MarshalIndent(snapshotMetadata{
		Version:   metadataVersion,
		ID:        snapshot.ID,
		ProjectID: snapshot.ProjectID,
		Name:      snapshot.Name,
		Kind:      snapshot.Kind,
		CreatedAt: snapshot.CreatedAt,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot metadata: %w", err)
	}
	body = append(body, '\n')

	metadataPath := filepath.Join(directory, snapshot.ID+".json")
	tempPath := metadataPath + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create snapshot metadata: %w", err)
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return fmt.Errorf("write snapshot metadata: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync snapshot metadata: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close snapshot metadata: %w", err)
	}
	if err := os.Rename(tempPath, metadataPath); err != nil {
		return fmt.Errorf("commit snapshot metadata: %w", err)
	}
	removeTemp = false
	return syncDirectory(directory)
}

func (a Archive) List(directory string) ([]revisiondomain.Snapshot, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []revisiondomain.Snapshot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list snapshot archive: %w", err)
	}
	result := make([]revisiondomain.Snapshot, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		snapshot, _, err := a.Find(directory, id)
		if err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (Archive) Find(
	directory string,
	id string,
) (revisiondomain.Snapshot, string, error) {
	if err := validateSnapshotID(id); err != nil {
		return revisiondomain.Snapshot{}, "", err
	}
	metadataPath := filepath.Join(directory, id+".json")
	body, err := os.ReadFile(metadataPath)
	if errors.Is(err, os.ErrNotExist) {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("%w: %s", ErrSnapshotMissing, id)
	}
	if err != nil {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("read snapshot metadata: %w", err)
	}
	var metadata snapshotMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("decode snapshot metadata: %w", err)
	}
	if metadata.Version != metadataVersion || metadata.ID != id {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("invalid snapshot metadata: %s", id)
	}
	snapshot, err := revisiondomain.NewSnapshot(
		metadata.ID,
		metadata.ProjectID,
		metadata.Name,
		metadata.Kind,
		metadata.CreatedAt,
	)
	if err != nil {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("validate snapshot metadata: %w", err)
	}
	databasePath := filepath.Join(directory, id+".sqlite")
	if info, err := os.Stat(databasePath); errors.Is(err, os.ErrNotExist) {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("%w: database for %s", ErrSnapshotMissing, id)
	} else if err != nil {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("inspect snapshot database: %w", err)
	} else if !info.Mode().IsRegular() {
		return revisiondomain.Snapshot{}, "", fmt.Errorf("snapshot database is not a regular file: %s", id)
	}
	return snapshot, databasePath, nil
}

func (Archive) Discard(directory string, id string) error {
	if err := validateSnapshotID(id); err != nil {
		return err
	}
	for _, suffix := range []string{".json", ".json.tmp", ".sqlite"} {
		if err := os.Remove(filepath.Join(directory, id+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("discard snapshot file: %w", err)
		}
	}
	return nil
}

func (a Archive) BeginRestore(databasePath string, snapshotPath string) error {
	if err := a.RecoverInterruptedRestore(databasePath); err != nil {
		return err
	}
	if _, err := os.Stat(databasePath); err != nil {
		return fmt.Errorf("inspect current database: %w", err)
	}

	stagedPath := databasePath + ".restore-staged"
	backupPath := databasePath + ".restore-backup"
	if err := copyFile(snapshotPath, stagedPath); err != nil {
		return fmt.Errorf("stage snapshot database: %w", err)
	}
	if err := os.Rename(databasePath, backupPath); err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("preserve current database: %w", err)
	}
	movedCurrent := true
	defer func() {
		if movedCurrent {
			_ = a.RollbackRestore(databasePath)
		}
	}()
	if err := removeSQLiteSidecars(databasePath); err != nil {
		return err
	}
	if err := os.Rename(stagedPath, databasePath); err != nil {
		return fmt.Errorf("activate snapshot database: %w", err)
	}
	if err := syncDirectory(filepath.Dir(databasePath)); err != nil {
		return err
	}
	movedCurrent = false
	return nil
}

func (Archive) CommitRestore(databasePath string) error {
	if err := os.Remove(databasePath + ".restore-staged"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove staged restore database: %w", err)
	}
	if err := syncDirectory(filepath.Dir(databasePath)); err != nil {
		return err
	}
	if err := os.Remove(databasePath + ".restore-backup"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("commit restored database: %w", err)
	}
	return nil
}

func (Archive) RollbackRestore(databasePath string) error {
	backupPath := databasePath + ".restore-backup"
	if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(databasePath + ".restore-staged")
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect restore backup: %w", err)
	}
	if err := os.Remove(databasePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove restored database: %w", err)
	}
	if err := removeSQLiteSidecars(databasePath); err != nil {
		return err
	}
	if err := os.Rename(backupPath, databasePath); err != nil {
		return fmt.Errorf("restore original database: %w", err)
	}
	_ = os.Remove(databasePath + ".restore-staged")
	return syncDirectory(filepath.Dir(databasePath))
}

func (a Archive) RecoverInterruptedRestore(databasePath string) error {
	return a.RollbackRestore(databasePath)
}

func validateSnapshotID(id string) error {
	if id == "" {
		return errors.New("snapshot id must not be blank")
	}
	for _, value := range id {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || value == '-' || value == '_' {
			continue
		}
		return fmt.Errorf("snapshot id contains unsafe character: %q", value)
	}
	return nil
}

func copyFile(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	removeTarget := true
	defer func() {
		_ = target.Close()
		if removeTarget {
			_ = os.Remove(targetPath)
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

func removeSQLiteSidecars(databasePath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(databasePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove SQLite sidecar: %w", err)
		}
	}
	return nil
}
