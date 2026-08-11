package projectfs

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

var (
	ErrRecoveryCopySourceChanged = ports.ErrRecoveryCopySourceChanged
	ErrProjectRecoveryRequired   = ports.ErrProjectRecoveryRequired
)

type recoveryCopyLayout struct {
	packageInfo ports.ProjectPackage
	temporary   string
	final       string
}

type recoveryCopyHooks struct {
	now                func() time.Time
	randomID           func() (string, error)
	copyPersistentFile func(context.Context, string, string) error
	syncDirectory      func(string) error
	renameNoReplace    func(string, string) error
	validate           func(context.Context, recoveryCopyLayout) error
}

type recoveryCopyFile struct {
	relative string
	sha256   [sha256.Size]byte
}

// CreateRecoveryCopy creates a new, independently editable project package.
// It never opens or mutates the source package itself: snapshot must be backed
// by a mode=ro source database connection, and every ordinary source file is
// read twice to prove its stable manifest before publication.
func (m Manager) CreateRecoveryCopy(
	ctx context.Context,
	source ports.ProjectPackage,
	parentDirectory string,
	snapshot ports.DatabaseSnapshot,
) (ports.ProjectPackage, error) {
	if err := ctx.Err(); err != nil {
		return ports.ProjectPackage{}, err
	}
	if snapshot == nil {
		return ports.ProjectPackage{}, errors.New("recovery copy database snapshot is required")
	}
	sourceLayout, err := InspectLayout(source.Root)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("inspect recovery copy source: %w", err)
	}
	source = packageFromLayout(sourceLayout)
	recoveryRequired, err := recoveryCopySourceRecoveryRequired(source)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("inspect recovery copy source recovery state: %w", err)
	}
	if recoveryRequired {
		return ports.ProjectPackage{}, ErrProjectRecoveryRequired
	}
	parentDirectory, err = recoveryCopyParent(parentDirectory)
	if err != nil {
		return ports.ProjectPackage{}, err
	}

	manifest, err := scanRecoveryCopyManifest(ctx, source.Root)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("scan recovery copy source: %w", err)
	}
	layout, err := m.newRecoveryCopyLayout(source, parentDirectory)
	if err != nil {
		return ports.ProjectPackage{}, err
	}
	if err := validateRecoveryCopyLayout(layout); err != nil {
		return ports.ProjectPackage{}, err
	}
	if err := os.Mkdir(layout.temporary, 0o700); err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("create recovery copy temporary package: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(layout.temporary)
		}
	}()

	if err := ctx.Err(); err != nil {
		return ports.ProjectPackage{}, err
	}
	temporaryPackage := packageFromLayout(layoutForRoot(layout.temporary))
	if err := snapshot(ctx, temporaryPackage.Database); err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("create consistent recovery copy database: %w", err)
	}
	for _, item := range manifest {
		if err := ctx.Err(); err != nil {
			return ports.ProjectPackage{}, err
		}
		target := filepath.Join(layout.temporary, filepath.FromSlash(item.relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return ports.ProjectPackage{}, fmt.Errorf("create recovery copy directory: %w", err)
		}
		if err := m.copyRecoveryCopyFile(ctx, filepath.Join(source.Root, filepath.FromSlash(item.relative)), target); err != nil {
			return ports.ProjectPackage{}, fmt.Errorf("copy recovery copy file %q: %w", item.relative, err)
		}
	}
	// Verify the bytes actually written to the temporary package against the
	// first source manifest. A source can otherwise change to different bytes
	// while a file is copied and change back before the second source scan
	// (an ABA mutation), leaving a valid-looking but mixed copy.
	temporaryManifest, err := scanRecoveryCopyManifest(ctx, layout.temporary)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("scan recovery copy temporary package: %w", err)
	}
	if !sameRecoveryCopyManifest(manifest, temporaryManifest) {
		return ports.ProjectPackage{}, ErrRecoveryCopySourceChanged
	}

	stableManifest, err := scanRecoveryCopyManifest(ctx, source.Root)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("rescan recovery copy source: %w", err)
	}
	recoveryRequired, err = recoveryCopySourceRecoveryRequired(source)
	if err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("rescan recovery copy source recovery state: %w", err)
	}
	if recoveryRequired {
		return ports.ProjectPackage{}, ErrProjectRecoveryRequired
	}
	if !sameRecoveryCopyManifest(manifest, stableManifest) {
		return ports.ProjectPackage{}, ErrRecoveryCopySourceChanged
	}
	if err := m.validateRecoveryCopy(ctx, recoveryCopyLayout{packageInfo: temporaryPackage, temporary: layout.temporary, final: layout.final}); err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("validate recovery copy: %w", err)
	}
	if err := m.syncRecoveryCopyDirectory(layout.temporary); err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("sync recovery copy temporary package: %w", err)
	}
	if err := m.renameRecoveryCopyNoReplace(layout.temporary, layout.final); err != nil {
		return ports.ProjectPackage{}, fmt.Errorf("publish recovery copy: %w", err)
	}
	published = true
	publishedPackage := packageFromLayout(layoutForRoot(layout.final))
	if err := m.syncRecoveryCopyDirectory(parentDirectory); err != nil {
		return publishedPackage, fmt.Errorf("sync published recovery copy parent directory: %w", err)
	}
	return publishedPackage, nil
}

func recoveryCopyParent(parentDirectory string) (string, error) {
	parentDirectory = strings.TrimSpace(parentDirectory)
	if parentDirectory == "" {
		return "", errors.New("recovery copy parent directory is blank")
	}
	parentDirectory, err := filepath.Abs(filepath.Clean(parentDirectory))
	if err != nil {
		return "", fmt.Errorf("resolve recovery copy parent directory: %w", err)
	}
	info, err := os.Stat(parentDirectory)
	if err != nil {
		return "", fmt.Errorf("inspect recovery copy parent directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("recovery copy parent path is not a directory")
	}
	return parentDirectory, nil
}

func (m Manager) newRecoveryCopyLayout(source ports.ProjectPackage, parent string) (recoveryCopyLayout, error) {
	hooks := m.recoveryCopyOperations()
	now := time.Now().UTC()
	if hooks.now != nil {
		now = hooks.now().UTC()
	}
	id, err := m.recoveryCopyID()
	if err != nil {
		return recoveryCopyLayout{}, err
	}
	base := strings.TrimSuffix(filepath.Base(source.Root), packageExtension)
	base, err = portableProjectName(base)
	if err != nil {
		base = "project"
	}
	base = truncateRecoveryCopyName(base, 48)
	name := fmt.Sprintf("%s-recovery-%s-%s%s", base, now.Format("20060102T150405Z"), id, packageExtension)
	final := filepath.Join(parent, name)
	if _, err := os.Lstat(final); err == nil {
		return recoveryCopyLayout{}, fmt.Errorf("recovery copy target already exists: %w", fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return recoveryCopyLayout{}, fmt.Errorf("inspect recovery copy target: %w", err)
	}
	temporary := filepath.Join(parent, fmt.Sprintf(".%s.recovery-copy-%s.tmp%s", base, id, packageExtension))
	if _, err := os.Lstat(temporary); err == nil {
		return recoveryCopyLayout{}, fmt.Errorf("recovery copy temporary target already exists: %w", fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return recoveryCopyLayout{}, fmt.Errorf("inspect recovery copy temporary target: %w", err)
	}
	return recoveryCopyLayout{temporary: temporary, final: final}, nil
}

func truncateRecoveryCopyName(name string, max int) string {
	runes := []rune(name)
	if len(runes) > max {
		return string(runes[:max])
	}
	return name
}

func (m Manager) recoveryCopyID() (string, error) {
	if hooks := m.recoveryCopyOperations(); hooks.randomID != nil {
		return hooks.randomID()
	}
	buf := make([]byte, 4)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", fmt.Errorf("create recovery copy id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func (m Manager) recoveryCopyOperations() recoveryCopyHooks {
	if m.recoveryCopyHooks == nil {
		return recoveryCopyHooks{}
	}
	return *m.recoveryCopyHooks
}

func (m Manager) copyRecoveryCopyFile(ctx context.Context, source, target string) error {
	if hooks := m.recoveryCopyOperations(); hooks.copyPersistentFile != nil {
		return hooks.copyPersistentFile(ctx, source, target)
	}
	return copyRecoveryCopyFile(ctx, source, target, recoveryCopyHooks{})
}

func copyRecoveryCopyFile(ctx context.Context, source, target string, hooks recoveryCopyHooks) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		_ = output.Close()
		if !completed {
			_ = os.Remove(target)
		}
	}()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			if err := writeRecoveryCopyAll(ctx, output, buffer[:read]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

func writeRecoveryCopyAll(ctx context.Context, output *os.File, body []byte) error {
	for len(body) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		written, err := output.Write(body)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		body = body[written:]
	}
	return nil
}

func scanRecoveryCopyManifest(ctx context.Context, root string) ([]recoveryCopyFile, error) {
	manifest := make([]recoveryCopyFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if excludeRecoveryCopyPath(relative, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("recovery copy does not support symbolic link %q", relative)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("recovery copy does not support non-regular file %q", relative)
		}
		sum, err := hashRecoveryCopyFile(ctx, path)
		if err != nil {
			return err
		}
		manifest = append(manifest, recoveryCopyFile{relative: relative, sha256: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(manifest, func(i, j int) bool { return manifest[i].relative < manifest[j].relative })
	return manifest, nil
}

func excludeRecoveryCopyPath(relative string, isDirectory bool) bool {
	parts := strings.Split(relative, "/")
	first := parts[0]
	if first == "recovery" || first == "project.lease" || first == "project.db" || strings.HasPrefix(first, "project.db-") {
		return true
	}
	if first == "branches" && len(parts) > 1 && parts[1] == "prepared" {
		return true
	}
	name := parts[len(parts)-1]
	if strings.HasSuffix(name, ".sqlite-wal") || strings.HasSuffix(name, ".sqlite-shm") {
		return true
	}
	if first == "snapshots" && strings.HasSuffix(name, ".json.tmp") {
		return true
	}
	if first == "branches" && (name == "operation.json" || name == "index.json.tmp" || name == "index.json.backup") {
		return true
	}
	if first == "branches" && len(parts) > 1 && parts[1] == "heads" &&
		(strings.HasSuffix(name, ".branch-backup") || strings.HasSuffix(name, ".branch-staged")) {
		return true
	}
	return false
}

// recoveryCopySourceRecoveryRequired mirrors the fixed-path recovery probes
// without reading journal bodies. Some branch journals contain absolute source
// paths, so treating their mere presence as a stop condition avoids both data
// disclosure and copying an inconsistent operation.
func recoveryCopySourceRecoveryRequired(source ports.ProjectPackage) (bool, error) {
	paths := []string{
		source.Database + ".restore-staged",
		source.Database + ".restore-backup",
		source.Database + ".branch-backup",
		source.Database + ".branch-staged",
		filepath.Join(source.Recovery, "migration.json"),
		filepath.Join(source.Recovery, "migration-stage.sqlite"),
		filepath.Join(source.Recovery, "migration-original.sqlite"),
		filepath.Join(source.Branches, "operation.json"),
		filepath.Join(source.Branches, "index.json.tmp"),
		filepath.Join(source.Branches, "index.json.backup"),
	}
	for _, path := range paths {
		if found, err := recoveryCopyArtifactExists(path); err != nil || found {
			return found, err
		}
	}
	headDirectory := filepath.Join(source.Branches, "heads")
	entries, err := os.ReadDir(headDirectory)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".branch-backup") || strings.HasSuffix(entry.Name(), ".branch-staged") {
			return true, nil
		}
	}
	return false, nil
}

func recoveryCopyArtifactExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func hashRecoveryCopyFile(ctx context.Context, path string) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return empty, err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			if _, err := hash.Write(buffer[:read]); err != nil {
				return empty, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return empty, readErr
		}
	}
	copy(empty[:], hash.Sum(nil))
	return empty, nil
}

func sameRecoveryCopyManifest(first, second []recoveryCopyFile) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func (m Manager) validateRecoveryCopy(ctx context.Context, layout recoveryCopyLayout) error {
	if hooks := m.recoveryCopyOperations(); hooks.validate != nil {
		return hooks.validate(ctx, layout)
	}
	// The freshly created online-backup image is a stable file, not a live WAL
	// database. Immutable validation therefore avoids creating coordination
	// sidecars in the published recovery package.
	projectID, err := recoveryCopyProjectID(ctx, layout.packageInfo.Database)
	if err != nil {
		return err
	}
	return validateRecoveryCopyArchives(ctx, layout.packageInfo, projectID)
}

func recoveryCopyProjectID(ctx context.Context, path string) (string, error) {
	db, err := sqlite.OpenReadOnlyImmutableDatabase(ctx, path)
	if err != nil {
		return "", err
	}
	var projectID string
	queryErr := db.QueryRowContext(ctx, `SELECT id FROM projects LIMIT 1`).Scan(&projectID)
	closeErr := db.Close()
	if queryErr != nil {
		return "", queryErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return projectID, nil
}

func validateRecoveryCopyArchives(ctx context.Context, packageInfo ports.ProjectPackage, projectID string) error {
	snapshots, err := (snapshotfs.Archive{}).List(packageInfo.Snapshots)
	if err != nil {
		return fmt.Errorf("validate recovery copy snapshot metadata: %w", err)
	}
	snapshotIDs := make(map[string]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.ProjectID != projectID {
			return fmt.Errorf("recovery copy snapshot %q belongs to project %q, want %q", snapshot.ID, snapshot.ProjectID, projectID)
		}
		snapshotIDs[snapshot.ID] = struct{}{}
	}
	if err := validateRecoveryCopySnapshotFiles(packageInfo.Snapshots, snapshotIDs); err != nil {
		return err
	}
	for _, directory := range []string{packageInfo.Snapshots, packageInfo.Branches} {
		if err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					return nil
				}
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sqlite") {
				return nil
			}
			db, err := sqlite.OpenReadOnlyImmutableDatabase(ctx, path)
			if err != nil {
				return err
			}
			return db.Close()
		}); err != nil {
			return fmt.Errorf("validate recovery copy archive %q: %w", directory, err)
		}
	}
	if err := validateRecoveryCopyBranchCatalog(packageInfo.Branches, projectID); err != nil {
		return err
	}
	return nil
}

func validateRecoveryCopySnapshotFiles(directory string, metadata map[string]struct{}) error {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect recovery copy snapshots: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sqlite") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".sqlite")
		if _, found := metadata[id]; !found {
			return fmt.Errorf("recovery copy snapshot database %q has no metadata", id)
		}
	}
	return nil
}

func validateRecoveryCopyBranchCatalog(directory, projectID string) error {
	catalog, err := (branchfs.Archive{}).ValidateReadOnly(directory)
	if errors.Is(err, branchfs.ErrNotInitialized) {
		return validateRecoveryCopyUncatalogedHeads(directory)
	}
	if err != nil {
		return fmt.Errorf("validate recovery copy branch catalog: %w", err)
	}
	if catalog.ProjectID != projectID {
		return fmt.Errorf("recovery copy branch catalog belongs to project %q, want %q", catalog.ProjectID, projectID)
	}
	return nil
}

func validateRecoveryCopyUncatalogedHeads(directory string) error {
	entries, err := os.ReadDir(filepath.Join(directory, "heads"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect uncataloged recovery copy branch heads: %w", err)
	}
	if len(entries) > 0 {
		return errors.New("recovery copy branch heads exist without a catalog")
	}
	return nil
}

func (m Manager) syncRecoveryCopyDirectory(directory string) error {
	if hooks := m.recoveryCopyOperations(); hooks.syncDirectory != nil {
		return hooks.syncDirectory(directory)
	}
	return syncRecoveryCopyDirectory(directory)
}

func (m Manager) renameRecoveryCopyNoReplace(source, target string) error {
	if hooks := m.recoveryCopyOperations(); hooks.renameNoReplace != nil {
		return hooks.renameNoReplace(source, target)
	}
	return renameRecoveryCopyNoReplace(source, target)
}
