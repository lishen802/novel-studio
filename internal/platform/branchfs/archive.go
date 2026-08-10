package branchfs

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	_ "modernc.org/sqlite"
)

const (
	catalogVersion = 1
	journalVersion = 1
)

var (
	ErrNotInitialized    = errors.New("branch archive is not initialized")
	ErrBranchNotFound    = errors.New("branch does not exist")
	ErrDuplicateBranch   = errors.New("branch id or name already exists")
	ErrProtectedBranch   = errors.New("main or active branch cannot be deleted")
	ErrBranchHasChildren = errors.New("branch with child branches cannot be deleted")
	ErrBranchHeadChanged = errors.New("branch head changed outside the branch archive")
)

type Archive struct{}

type catalogFile struct {
	Version        int                     `json:"version"`
	Revision       int64                   `json:"revision"`
	ProjectID      string                  `json:"projectId"`
	ActiveBranchID string                  `json:"activeBranchId"`
	Branches       []revisiondomain.Branch `json:"branches"`
}

type operationJournal struct {
	Version     int    `json:"version"`
	Database    string `json:"database,omitempty"`
	Head        string `json:"head,omitempty"`
	OldRevision int64  `json:"oldRevision"`
	NewRevision int64  `json:"newRevision"`
}

func (Archive) Prepare(directory, id string) (string, error) {
	if err := validateBranchID(id); err != nil {
		return "", err
	}
	preparedDirectory := filepath.Join(directory, "prepared")
	if err := os.MkdirAll(preparedDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create prepared branch directory: %w", err)
	}
	path := filepath.Join(preparedDirectory, id+".sqlite")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("prepared branch backup already exists: %s", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func (Archive) DiscardPrepared(directory, id string) error {
	if err := validateBranchID(id); err != nil {
		return err
	}
	path := filepath.Join(directory, "prepared", id+".sqlite")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("discard prepared branch backup: %w", err)
	}
	return nil
}

func (a Archive) Initialize(
	directory, projectID, mainID, databasePath string,
	now time.Time,
) (revisiondomain.BranchCatalog, error) {
	if catalog, err := a.Load(directory); err == nil {
		if catalog.ProjectID != projectID {
			return revisiondomain.BranchCatalog{}, errors.New("branch archive belongs to another project")
		}
		return catalog, nil
	} else if !errors.Is(err, ErrNotInitialized) {
		return revisiondomain.BranchCatalog{}, err
	}
	if err := validateBranchID(mainID); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	if err := os.MkdirAll(headsDirectory(directory), 0o755); err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("create branch archive: %w", err)
	}
	digest, err := digestFile(databasePath)
	if err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("digest project database: %w", err)
	}
	main, err := revisiondomain.NewMainBranch(mainID, projectID, digest, now)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	head := a.HeadPath(directory, mainID)
	if err := copyFile(databasePath, head); err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("create main branch head: %w", err)
	}
	catalog := revisiondomain.BranchCatalog{
		Revision:       1,
		ProjectID:      projectID,
		ActiveBranchID: mainID,
		Branches:       []revisiondomain.Branch{main},
	}
	if err := a.writeCatalog(directory, catalog); err != nil {
		_ = os.Remove(head)
		return revisiondomain.BranchCatalog{}, err
	}
	return catalog, nil
}

func (a Archive) Load(directory string) (revisiondomain.BranchCatalog, error) {
	if err := recoverCatalogFile(directory); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	body, err := os.ReadFile(catalogPath(directory))
	if errors.Is(err, os.ErrNotExist) {
		return revisiondomain.BranchCatalog{}, ErrNotInitialized
	}
	if err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("read branch catalog: %w", err)
	}
	var stored catalogFile
	if err := json.Unmarshal(body, &stored); err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("decode branch catalog: %w", err)
	}
	if stored.Version != catalogVersion || stored.Revision < 1 ||
		strings.TrimSpace(stored.ProjectID) == "" || len(stored.Branches) == 0 {
		return revisiondomain.BranchCatalog{}, errors.New("invalid branch catalog")
	}
	catalog := revisiondomain.BranchCatalog{
		Revision:       stored.Revision,
		ProjectID:      stored.ProjectID,
		ActiveBranchID: stored.ActiveBranchID,
		Branches:       stored.Branches,
	}
	if _, found := catalog.Find(catalog.ActiveBranchID); !found {
		return revisiondomain.BranchCatalog{}, errors.New("branch catalog has no active branch")
	}
	for _, branch := range catalog.Branches {
		if err := validateBranchID(branch.ID); err != nil ||
			branch.ProjectID != catalog.ProjectID || branch.HeadDigest == "" {
			return revisiondomain.BranchCatalog{}, errors.New("branch catalog contains invalid branch metadata")
		}
		info, err := os.Stat(a.HeadPath(directory, branch.ID))
		if err != nil {
			return revisiondomain.BranchCatalog{}, fmt.Errorf("inspect branch head %s: %w", branch.ID, err)
		}
		if !info.Mode().IsRegular() {
			return revisiondomain.BranchCatalog{}, fmt.Errorf("branch head is not a regular file: %s", branch.ID)
		}
	}
	return catalog, nil
}

func (a Archive) HeadPath(directory, branchID string) string {
	return filepath.Join(headsDirectory(directory), branchID+".sqlite")
}

func (a Archive) Checkpoint(
	directory, branchID, snapshotPath string,
	now time.Time,
) (revisiondomain.Branch, error) {
	catalog, err := a.Load(directory)
	if err != nil {
		return revisiondomain.Branch{}, err
	}
	index := branchIndex(catalog, branchID)
	if index < 0 {
		return revisiondomain.Branch{}, fmt.Errorf("%w: %s", ErrBranchNotFound, branchID)
	}
	digest, err := digestFile(snapshotPath)
	if err != nil {
		return revisiondomain.Branch{}, fmt.Errorf("digest branch checkpoint: %w", err)
	}
	updated := catalog.Branches[index].Checkpoint(digest, now)
	if updated == catalog.Branches[index] {
		return updated, nil
	}
	next := cloneCatalog(catalog)
	next.Revision++
	next.Branches[index] = updated
	if err := a.replaceHeadAndCatalog(
		directory,
		a.HeadPath(directory, branchID),
		snapshotPath,
		catalog,
		next,
	); err != nil {
		return revisiondomain.Branch{}, err
	}
	return updated, nil
}

func (a Archive) Fork(
	directory, id, name, description, parentID, snapshotPath string,
	now time.Time,
) (revisiondomain.Branch, error) {
	if err := validateBranchID(id); err != nil {
		return revisiondomain.Branch{}, err
	}
	catalog, err := a.Load(directory)
	if err != nil {
		return revisiondomain.Branch{}, err
	}
	parent, found := catalog.Find(parentID)
	if !found {
		return revisiondomain.Branch{}, fmt.Errorf("%w: %s", ErrBranchNotFound, parentID)
	}
	for _, branch := range catalog.Branches {
		if branch.ID == id || strings.EqualFold(branch.Name, strings.TrimSpace(name)) {
			return revisiondomain.Branch{}, ErrDuplicateBranch
		}
	}
	if _, err := a.Checkpoint(directory, parentID, snapshotPath, now); err != nil {
		return revisiondomain.Branch{}, fmt.Errorf("checkpoint parent before branch fork: %w", err)
	}
	catalog, err = a.Load(directory)
	if err != nil {
		return revisiondomain.Branch{}, err
	}
	parent, found = catalog.Find(parentID)
	if !found {
		return revisiondomain.Branch{}, fmt.Errorf("%w: %s", ErrBranchNotFound, parentID)
	}
	child, err := revisiondomain.ForkBranch(id, name, description, parent, now)
	if err != nil {
		return revisiondomain.Branch{}, err
	}
	head := a.HeadPath(directory, id)
	if err := copyFile(snapshotPath, head); err != nil {
		return revisiondomain.Branch{}, fmt.Errorf("create branch head: %w", err)
	}
	next := cloneCatalog(catalog)
	next.Revision++
	parentIndex := branchIndex(next, parent.ID)
	next.Branches[parentIndex] = parent
	next.Branches = append(next.Branches, child)
	if err := a.writeCatalog(directory, next); err != nil {
		_ = os.Remove(head)
		return revisiondomain.Branch{}, err
	}
	return child, nil
}

func (a Archive) Switch(
	directory, databasePath, targetID, currentSnapshotPath string,
	now time.Time,
) (revisiondomain.BranchCatalog, error) {
	if err := a.RecoverInterruptedOperation(directory, databasePath); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	catalog, err := a.Load(directory)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	if catalog.ActiveBranchID == targetID {
		return catalog, nil
	}
	target, found := catalog.Find(targetID)
	if !found {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("%w: %s", ErrBranchNotFound, targetID)
	}
	targetDigest, err := digestFile(a.HeadPath(directory, targetID))
	if err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("verify target branch head: %w", err)
	}
	if targetDigest != target.HeadDigest {
		return revisiondomain.BranchCatalog{}, ErrBranchHeadChanged
	}
	if _, err := a.Checkpoint(directory, catalog.ActiveBranchID, currentSnapshotPath, now); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	catalog, err = a.Load(directory)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	next := cloneCatalog(catalog)
	next.Revision++
	next.ActiveBranchID = targetID
	if err := a.replaceDatabaseAndCatalog(
		directory,
		databasePath,
		a.HeadPath(directory, targetID),
		"",
		"",
		catalog,
		next,
	); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	return next, nil
}

func (a Archive) PlanMerge(
	directory, sourceID, targetID string,
) (revisiondomain.BranchMergePlan, error) {
	catalog, err := a.Load(directory)
	if err != nil {
		return revisiondomain.BranchMergePlan{}, err
	}
	source, found := catalog.Find(sourceID)
	if !found {
		return revisiondomain.BranchMergePlan{}, fmt.Errorf("%w: %s", ErrBranchNotFound, sourceID)
	}
	target, found := catalog.Find(targetID)
	if !found {
		return revisiondomain.BranchMergePlan{}, fmt.Errorf("%w: %s", ErrBranchNotFound, targetID)
	}
	return revisiondomain.PlanFastForwardMerge(source, target)
}

func (a Archive) Merge(
	directory, databasePath string,
	plan revisiondomain.BranchMergePlan,
	currentSnapshotPath string,
	now time.Time,
) (revisiondomain.BranchCatalog, error) {
	if err := a.RecoverInterruptedOperation(directory, databasePath); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	catalog, err := a.Load(directory)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	if catalog.ActiveBranchID != plan.TargetID {
		return revisiondomain.BranchCatalog{}, errors.New("merge target must be the active branch")
	}
	sourceBranch, found := catalog.Find(plan.SourceID)
	if !found {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("%w: %s", ErrBranchNotFound, plan.SourceID)
	}
	sourceDigest, err := digestFile(a.HeadPath(directory, sourceBranch.ID))
	if err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("verify source branch head: %w", err)
	}
	if sourceDigest != sourceBranch.HeadDigest || sourceDigest != plan.SourceDigest {
		return revisiondomain.BranchCatalog{}, ErrBranchHeadChanged
	}
	if _, err := a.Checkpoint(directory, plan.TargetID, currentSnapshotPath, now); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	catalog, err = a.Load(directory)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	sourceIndex := branchIndex(catalog, plan.SourceID)
	targetIndex := branchIndex(catalog, plan.TargetID)
	if sourceIndex < 0 || targetIndex < 0 {
		return revisiondomain.BranchCatalog{}, ErrBranchNotFound
	}
	source, target, err := revisiondomain.CompleteFastForwardMerge(
		catalog.Branches[sourceIndex],
		catalog.Branches[targetIndex],
		plan,
		now,
	)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	next := cloneCatalog(catalog)
	next.Revision++
	next.Branches[sourceIndex] = source
	next.Branches[targetIndex] = target
	targetHead := a.HeadPath(directory, target.ID)
	if err := a.replaceDatabaseAndCatalog(
		directory,
		databasePath,
		a.HeadPath(directory, source.ID),
		targetHead,
		a.HeadPath(directory, source.ID),
		catalog,
		next,
	); err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	return next, nil
}

func (a Archive) Delete(directory, id string) error {
	catalog, err := a.Load(directory)
	if err != nil {
		return err
	}
	index := branchIndex(catalog, id)
	if index < 0 {
		return fmt.Errorf("%w: %s", ErrBranchNotFound, id)
	}
	if catalog.ActiveBranchID == id || catalog.Branches[index].ParentID == "" {
		return ErrProtectedBranch
	}
	for _, branch := range catalog.Branches {
		if branch.ParentID == id {
			return ErrBranchHasChildren
		}
	}
	next := cloneCatalog(catalog)
	next.Revision++
	next.Branches = append(next.Branches[:index], next.Branches[index+1:]...)
	if err := a.writeCatalog(directory, next); err != nil {
		return err
	}
	if err := os.Remove(a.HeadPath(directory, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete branch head: %w", err)
	}
	return syncDirectory(headsDirectory(directory))
}

func (a Archive) RecoverInterruptedOperation(directory, databasePath string) error {
	if err := recoverCatalogFile(directory); err != nil {
		return err
	}
	body, err := os.ReadFile(journalPath(directory))
	if errors.Is(err, os.ErrNotExist) {
		return recoverLooseDatabaseBackup(databasePath)
	}
	if err != nil {
		return fmt.Errorf("read branch operation journal: %w", err)
	}
	var journal operationJournal
	if err := json.Unmarshal(body, &journal); err != nil || journal.Version != journalVersion {
		return errors.New("invalid branch operation journal")
	}
	catalog, loadErr := a.Load(directory)
	committed := loadErr == nil && catalog.Revision == journal.NewRevision
	if !committed {
		if err := restoreBackup(journal.Database); err != nil {
			return err
		}
		if err := restoreBackup(journal.Head); err != nil {
			return err
		}
	} else {
		if err := removeOperationArtifact(journal.Database, ".branch-backup"); err != nil {
			return err
		}
		if err := removeOperationArtifact(journal.Head, ".branch-backup"); err != nil {
			return err
		}
	}
	if err := removeOperationArtifact(journal.Database, ".branch-staged"); err != nil {
		return err
	}
	if err := removeOperationArtifact(journal.Head, ".branch-staged"); err != nil {
		return err
	}
	if err := os.Remove(journalPath(directory)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove branch operation journal: %w", err)
	}
	return syncDirectory(directory)
}

// RecoveryRequired reports whether branch-operation artifacts are present
// without repairing or deleting them. The writable recovery routine has side
// effects, so it must never be used as a read-only preflight check.
func (Archive) RecoveryRequired(directory, databasePath string) (bool, error) {
	paths := []string{
		journalPath(directory),
		catalogPath(directory) + ".backup",
		catalogPath(directory) + ".tmp",
	}
	if databasePath != "" {
		paths = append(paths,
			databasePath+".branch-backup",
			databasePath+".branch-staged",
		)
	}
	for _, path := range paths {
		found, err := branchRecoveryArtifactExists(path)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	headDirectory := headsDirectory(directory)
	entries, err := os.ReadDir(headDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect branch heads for recovery artifacts: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".branch-backup") || strings.HasSuffix(entry.Name(), ".branch-staged") {
			return true, nil
		}
	}
	return false, nil
}

func branchRecoveryArtifactExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect branch recovery artifact: %w", err)
}

func (a Archive) replaceHeadAndCatalog(
	directory, headPath, sourcePath string,
	oldCatalog, newCatalog revisiondomain.BranchCatalog,
) error {
	if err := stageFile(sourcePath, headPath); err != nil {
		return err
	}
	journal := operationJournal{
		Version:     journalVersion,
		Head:        headPath,
		OldRevision: oldCatalog.Revision,
		NewRevision: newCatalog.Revision,
	}
	if err := a.writeJournal(directory, journal); err != nil {
		discardStage(headPath)
		return err
	}
	if err := activateStage(headPath); err != nil {
		_ = a.RecoverInterruptedOperation(directory, "")
		return err
	}
	if err := a.writeCatalog(directory, newCatalog); err != nil {
		_ = a.RecoverInterruptedOperation(directory, "")
		return err
	}
	a.finishOperation(directory, journal)
	return nil
}

func (a Archive) replaceDatabaseAndCatalog(
	directory, databasePath, databaseSource, headPath, headSource string,
	oldCatalog, newCatalog revisiondomain.BranchCatalog,
) error {
	if err := stageFile(databaseSource, databasePath); err != nil {
		return err
	}
	if headPath != "" {
		if err := stageFile(headSource, headPath); err != nil {
			discardStage(databasePath)
			return err
		}
	}
	journal := operationJournal{
		Version:     journalVersion,
		Database:    databasePath,
		Head:        headPath,
		OldRevision: oldCatalog.Revision,
		NewRevision: newCatalog.Revision,
	}
	if err := a.writeJournal(directory, journal); err != nil {
		discardStage(databasePath)
		discardStage(headPath)
		return err
	}
	if err := activateStage(databasePath); err != nil {
		_ = a.RecoverInterruptedOperation(directory, databasePath)
		return err
	}
	if headPath != "" {
		if err := activateStage(headPath); err != nil {
			_ = a.RecoverInterruptedOperation(directory, databasePath)
			return err
		}
	}
	if err := a.writeCatalog(directory, newCatalog); err != nil {
		_ = a.RecoverInterruptedOperation(directory, databasePath)
		return err
	}
	a.finishOperation(directory, journal)
	return nil
}

func (a Archive) finishOperation(directory string, journal operationJournal) {
	for _, artifact := range []struct {
		target string
		suffix string
	}{
		{journal.Database, ".branch-backup"},
		{journal.Head, ".branch-backup"},
		{journal.Database, ".branch-staged"},
		{journal.Head, ".branch-staged"},
	} {
		if err := removeOperationArtifact(artifact.target, artifact.suffix); err != nil {
			return
		}
	}
	if err := os.Remove(journalPath(directory)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	_ = syncDirectory(directory)
}

func (a Archive) writeCatalog(directory string, catalog revisiondomain.BranchCatalog) error {
	if err := os.MkdirAll(headsDirectory(directory), 0o755); err != nil {
		return fmt.Errorf("create branch archive: %w", err)
	}
	body, err := json.MarshalIndent(catalogFile{
		Version:        catalogVersion,
		Revision:       catalog.Revision,
		ProjectID:      catalog.ProjectID,
		ActiveBranchID: catalog.ActiveBranchID,
		Branches:       catalog.Branches,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode branch catalog: %w", err)
	}
	body = append(body, '\n')
	return atomicWriteFile(catalogPath(directory), body)
}

func (a Archive) writeJournal(directory string, journal operationJournal) error {
	body, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode branch operation journal: %w", err)
	}
	body = append(body, '\n')
	return atomicWriteFile(journalPath(directory), body)
}

func cloneCatalog(catalog revisiondomain.BranchCatalog) revisiondomain.BranchCatalog {
	catalog.Branches = append([]revisiondomain.Branch(nil), catalog.Branches...)
	return catalog
}

func branchIndex(catalog revisiondomain.BranchCatalog, id string) int {
	for index, branch := range catalog.Branches {
		if branch.ID == id {
			return index
		}
	}
	return -1
}

func headsDirectory(directory string) string {
	return filepath.Join(directory, "heads")
}

func catalogPath(directory string) string {
	return filepath.Join(directory, "index.json")
}

func journalPath(directory string) string {
	return filepath.Join(directory, "operation.json")
}

func validateBranchID(id string) error {
	if id == "" {
		return revisiondomain.ErrBlankBranchID
	}
	for _, value := range id {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || value == '-' || value == '_' {
			continue
		}
		return fmt.Errorf("branch id contains unsafe character: %q", value)
	}
	return nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	header := make([]byte, 100)
	read, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	header = header[:read]
	if len(header) == 100 && string(header[:16]) == "SQLite format 3\x00" {
		return digestSQLiteDatabase(path)
	}
	if _, err := hash.Write(header); err != nil {
		return "", err
	}
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func digestSQLiteDatabase(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	urlPath := filepath.ToSlash(absolutePath)
	if filepath.VolumeName(absolutePath) != "" && !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	dsn := (&url.URL{Scheme: "file", Path: urlPath}).String() + "?mode=ro"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", err
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	rows, err := database.Query(`
		SELECT name, COALESCE(sql, '')
		FROM sqlite_master
		WHERE type = 'table'
		ORDER BY name
	`)
	if err != nil {
		return "", err
	}
	type tableDefinition struct {
		name   string
		schema string
	}
	tables := make([]tableDefinition, 0)
	for rows.Next() {
		var table tableDefinition
		if err := rows.Scan(&table.name, &table.schema); err != nil {
			_ = rows.Close()
			return "", err
		}
		tables = append(tables, table)
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, table := range tables {
		writeDigestPart(hash, table.name)
		writeDigestPart(hash, table.schema)
		columns, err := sqliteColumns(database, table.name)
		if err != nil {
			return "", err
		}
		for _, column := range columns {
			writeDigestPart(hash, column)
		}
		if len(columns) == 0 {
			continue
		}
		quotedColumns := make([]string, 0, len(columns))
		orderColumns := make([]string, 0, len(columns))
		for _, column := range columns {
			identifier := quoteIdentifier(column)
			quotedColumns = append(quotedColumns, "quote("+identifier+")")
			orderColumns = append(orderColumns, identifier)
		}
		query := "SELECT " + strings.Join(quotedColumns, ",") +
			" FROM " + quoteIdentifier(table.name) +
			" ORDER BY " + strings.Join(orderColumns, ",")
		tableRows, err := database.Query(query)
		if err != nil {
			return "", err
		}
		for tableRows.Next() {
			values := make([]sql.NullString, len(columns))
			destinations := make([]any, len(columns))
			for index := range values {
				destinations[index] = &values[index]
			}
			if err := tableRows.Scan(destinations...); err != nil {
				_ = tableRows.Close()
				return "", err
			}
			for _, value := range values {
				writeDigestPart(hash, value.String)
			}
		}
		if err := tableRows.Close(); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sqliteColumns(database *sql.DB, table string) ([]string, error) {
	rows, err := database.Query("PRAGMA table_info(" + quoteIdentifier(table) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var sequence int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&sequence, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func writeDigestPart(writer io.Writer, value string) {
	_, _ = fmt.Fprintf(writer, "%d:", len(value))
	_, _ = io.WriteString(writer, value)
}

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
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

func stageFile(sourcePath, targetPath string) error {
	if targetPath == "" {
		return nil
	}
	discardStage(targetPath)
	return copyFile(sourcePath, targetPath+".branch-staged")
}

func activateStage(targetPath string) error {
	if targetPath == "" {
		return nil
	}
	backup := targetPath + ".branch-backup"
	_ = os.Remove(backup)
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backup); err != nil {
			return fmt.Errorf("preserve branch operation target: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := removeSQLiteSidecars(targetPath); err != nil {
		return err
	}
	if err := os.Rename(targetPath+".branch-staged", targetPath); err != nil {
		_ = restoreBackup(targetPath)
		return fmt.Errorf("activate branch operation target: %w", err)
	}
	return syncDirectory(filepath.Dir(targetPath))
}

func restoreBackup(targetPath string) error {
	if targetPath == "" {
		return nil
	}
	backup := targetPath + ".branch-backup"
	if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := removeSQLiteSidecars(targetPath); err != nil {
		return err
	}
	if err := os.Rename(backup, targetPath); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(targetPath))
}

func discardBackup(targetPath string) {
	_ = removeOperationArtifact(targetPath, ".branch-backup")
}

func discardStage(targetPath string) {
	_ = removeOperationArtifact(targetPath, ".branch-staged")
}

func removeOperationArtifact(targetPath, suffix string) error {
	if targetPath == "" {
		return nil
	}
	if err := os.Remove(targetPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove branch operation artifact: %w", err)
	}
	return nil
}

func recoverLooseDatabaseBackup(databasePath string) error {
	if databasePath == "" {
		return nil
	}
	return restoreBackup(databasePath)
}

func removeSQLiteSidecars(databasePath string) error {
	if databasePath == "" {
		return nil
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(databasePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func atomicWriteFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	backup := path + ".backup"
	_ = os.Remove(temp)
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	_ = os.Remove(backup)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			_ = os.Remove(temp)
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		_ = os.Remove(path)
		_ = os.Rename(backup, path)
		_ = syncDirectory(filepath.Dir(path))
		return err
	}
	_ = os.Remove(backup)
	_ = syncDirectory(filepath.Dir(path))
	return nil
}

func recoverCatalogFile(directory string) error {
	path := catalogPath(directory)
	backup := path + ".backup"
	_, pathErr := os.Stat(path)
	_, backupErr := os.Stat(backup)
	if errors.Is(pathErr, os.ErrNotExist) && backupErr == nil {
		if err := os.Rename(backup, path); err != nil {
			return fmt.Errorf("recover branch catalog: %w", err)
		}
		return syncDirectory(directory)
	}
	if pathErr == nil && backupErr == nil {
		if err := os.Remove(backup); err != nil {
			return err
		}
	}
	return nil
}
