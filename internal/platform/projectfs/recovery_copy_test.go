package projectfs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
)

func TestRecoveryCopyIncludesCommittedActiveWALAndExcludesCoordinationFiles(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	chapter := mustRecoveryCopyChapter(t, "chapter-wal", "来自活跃 WAL 的正文")
	if err := sourceStore.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source.Database + "-wal"); err != nil {
		t.Fatalf("active WAL: %v", err)
	}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	if err := os.WriteFile(source.Lease, []byte("diagnostic only"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.Snapshots, "partial.json.tmp"), []byte("temporary"), 0o600); err != nil {
		t.Fatal(err)
	}
	attachment := filepath.Join(source.Attachments, "cover.txt")
	if err := os.WriteFile(attachment, []byte("persistent attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyPackage, err := (Manager{}).CreateRecoveryCopy(ctx, source, t.TempDir(), readOnly.Backup)
	if err != nil {
		t.Fatalf("CreateRecoveryCopy: %v", err)
	}
	copyStore, err := (sqlite.Factory{}).OpenReadOnlyImmutable(ctx, copyPackage.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer copyStore.Close()
	got, err := copyStore.Chapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != chapter.Body || got.Revision != chapter.Revision {
		t.Fatalf("copied chapter = %#v, want %#v", got, chapter)
	}
	for _, path := range []string{
		copyPackage.Lease,
		copyPackage.Database + "-wal",
		copyPackage.Database + "-shm",
		filepath.Join(copyPackage.Snapshots, "partial.json.tmp"),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("excluded artifact %q exists (err %v)", path, err)
		}
	}
	if body, err := os.ReadFile(filepath.Join(copyPackage.Attachments, "cover.txt")); err != nil || string(body) != "persistent attachment" {
		t.Fatalf("persistent attachment = %q, error %v", body, err)
	}
}

func TestRecoveryCopyNeverOverwritesExistingTarget(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	parent := t.TempDir()
	manager := Manager{recoveryCopyHooks: &recoveryCopyHooks{
		now:      func() time.Time { return time.Unix(0, 0) },
		randomID: func() (string, error) { return "fixed-id", nil },
	}}
	target := filepath.Join(parent, "恢复副本-recovery-19700101T000000Z-fixed-id.zjproject")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "sentinel")
	if err := os.WriteFile(sentinel, []byte("do not replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	_, err = manager.CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("CreateRecoveryCopy error = %v, want exists", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "do not replace" {
		t.Fatalf("existing target = %q, error %v", got, err)
	}
}

func TestRecoveryCopyRejectsSourceChangeWithoutPublishingPartialPackage(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	attachment := filepath.Join(source.Attachments, "story.txt")
	if err := os.WriteFile(attachment, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	changed := false
	manager := Manager{recoveryCopyHooks: &recoveryCopyHooks{
		copyPersistentFile: func(ctx context.Context, from, to string) error {
			if err := copyRecoveryCopyFile(ctx, from, to, recoveryCopyHooks{}); err != nil {
				return err
			}
			if !changed {
				changed = true
				return os.WriteFile(attachment, []byte("after"), 0o600)
			}
			return nil
		},
	}}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	_, err = manager.CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, ErrRecoveryCopySourceChanged) {
		t.Fatalf("CreateRecoveryCopy error = %v, want source changed", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial package published: %#v", entries)
	}
	if got, err := os.ReadFile(attachment); err != nil || string(got) != "after" {
		t.Fatalf("source attachment = %q, error %v", got, err)
	}
}

func TestRecoveryCopyRejectsABASourceMutationWhenCopiedBytesDoNotMatchManifest(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	attachment := filepath.Join(source.Attachments, "story.txt")
	if err := os.WriteFile(attachment, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	hooks := recoveryCopyHooks{copyPersistentFile: func(ctx context.Context, from, to string) error {
		if err := os.WriteFile(attachment, []byte("transient mutation"), 0o600); err != nil {
			return err
		}
		if err := copyRecoveryCopyFile(ctx, from, to, recoveryCopyHooks{}); err != nil {
			return err
		}
		return os.WriteFile(attachment, []byte("original"), 0o600)
	}}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	parent := t.TempDir()
	_, err = (Manager{recoveryCopyHooks: &hooks}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, ErrRecoveryCopySourceChanged) {
		t.Fatalf("CreateRecoveryCopy error = %v, want source changed", err)
	}
	if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
		t.Fatalf("ABA source mutation published a package: entries=%#v err=%v", entries, readErr)
	}
}

func TestRecoveryCopyValidationFailureCleansOnlyOwnedTemporaryPackage(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	parent := t.TempDir()
	validateErr := errors.New("validation failed")
	manager := Manager{recoveryCopyHooks: &recoveryCopyHooks{validate: func(context.Context, recoveryCopyLayout) error { return validateErr }}}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	_, err = manager.CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, validateErr) {
		t.Fatalf("CreateRecoveryCopy error = %v, want validation error", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation failure left temporary package: %#v", entries)
	}
	if _, err := os.Stat(source.Database); err != nil {
		t.Fatalf("source database removed: %v", err)
	}
}

func TestRecoveryCopyFaultsDoNotPublishPartialPackageOrDeleteSource(t *testing.T) {
	tests := []struct {
		name  string
		hooks func(error) recoveryCopyHooks
	}{
		{
			name: "copy",
			hooks: func(injected error) recoveryCopyHooks {
				return recoveryCopyHooks{copyPersistentFile: func(context.Context, string, string) error { return injected }}
			},
		},
		{
			name: "fsync temporary directory",
			hooks: func(injected error) recoveryCopyHooks {
				return recoveryCopyHooks{syncDirectory: func(string) error { return injected }}
			},
		},
		{
			name: "rename",
			hooks: func(injected error) recoveryCopyHooks {
				return recoveryCopyHooks{renameNoReplace: func(string, string) error { return injected }}
			},
		},
		{
			name: "validation",
			hooks: func(injected error) recoveryCopyHooks {
				return recoveryCopyHooks{validate: func(context.Context, recoveryCopyLayout) error { return injected }}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			source, sourceStore := recoveryCopySource(t)
			defer sourceStore.Close()
			sourceAttachment := filepath.Join(source.Attachments, "persistent.txt")
			if err := os.WriteFile(sourceAttachment, []byte("must remain"), 0o600); err != nil {
				t.Fatal(err)
			}
			readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
			if err != nil {
				t.Fatal(err)
			}
			defer readOnly.Close()
			parent := t.TempDir()
			injected := errors.New(test.name + " failure")
			hooks := test.hooks(injected)
			manager := Manager{recoveryCopyHooks: &hooks}
			_, err = manager.CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
			if !errors.Is(err, injected) {
				t.Fatalf("CreateRecoveryCopy error = %v, want injected %v", err, injected)
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("fault published or retained partial package: %#v", entries)
			}
			if got, err := os.ReadFile(sourceAttachment); err != nil || string(got) != "must remain" {
				t.Fatalf("source attachment = %q, error %v", got, err)
			}
			if _, err := os.Stat(source.Database); err != nil {
				t.Fatalf("source database removed: %v", err)
			}
		})
	}
}

func TestRecoveryCopySyncsParentAfterAtomicPublicationAndRetainsPublishedPathOnFailure(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	parent := t.TempDir()
	syncCalls := 0
	parentSyncErr := errors.New("parent sync failed")
	hooks := recoveryCopyHooks{syncDirectory: func(string) error {
		syncCalls++
		if syncCalls == 2 {
			return parentSyncErr
		}
		return nil
	}}
	published, err := (Manager{recoveryCopyHooks: &hooks}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, parentSyncErr) {
		t.Fatalf("CreateRecoveryCopy error = %v, want parent sync failure", err)
	}
	if syncCalls != 2 {
		t.Fatalf("directory sync calls = %d, want temporary and parent sync", syncCalls)
	}
	if published.Root == "" {
		t.Fatal("published recovery copy path lost after parent sync failure")
	}
	if info, statErr := os.Stat(published.Root); statErr != nil || !info.IsDir() {
		t.Fatalf("published package unavailable after parent sync failure: %v", statErr)
	}
}

func TestRecoveryCopyUsesFreshTargetForEachRequest(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	parent := t.TempDir()
	first, err := (Manager{}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Manager{}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if first.Root == second.Root {
		t.Fatalf("recovery copies reused target %q", first.Root)
	}
	for _, packageInfo := range []ports.ProjectPackage{first, second} {
		if info, err := os.Stat(packageInfo.Root); err != nil || !info.IsDir() {
			t.Fatalf("recovery copy %q missing: %v", packageInfo.Root, err)
		}
	}
}

func TestRecoveryCopyRejectsInterruptedRecoveryArtifactsWithoutPublishingOrReadingJournal(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	journal := filepath.Join(source.Branches, "operation.json")
	// Real journals can contain absolute source paths. The copier must only
	// stat this known artifact and must never copy or parse it.
	if err := os.WriteFile(journal, []byte(`{"database":"/private/source/project.db"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	snapshotCalled := false
	_, err := (Manager{}).CreateRecoveryCopy(ctx, source, parent, func(context.Context, string) error {
		snapshotCalled = true
		return nil
	})
	if !errors.Is(err, ErrProjectRecoveryRequired) {
		t.Fatalf("CreateRecoveryCopy error = %v, want recovery required", err)
	}
	if snapshotCalled {
		t.Fatal("recovery copy read source database after interrupted-operation preflight")
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("interrupted source published a copy: entries=%#v err=%v", entries, err)
	}
	if body, err := os.ReadFile(journal); err != nil || string(body) != `{"database":"/private/source/project.db"}` {
		t.Fatalf("source journal changed: %q, error %v", body, err)
	}
}

func TestRecoveryCopyRejectsRecoveryJournalIntroducedDuringCopy(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	if err := os.WriteFile(filepath.Join(source.Attachments, "persistent.txt"), []byte("copy me"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(source.Branches, "operation.json")
	createdJournal := false
	hooks := recoveryCopyHooks{copyPersistentFile: func(ctx context.Context, from, to string) error {
		if err := copyRecoveryCopyFile(ctx, from, to, recoveryCopyHooks{}); err != nil {
			return err
		}
		if !createdJournal {
			createdJournal = true
			return os.WriteFile(journal, []byte(`{"database":"/private/source/project.db"}`), 0o600)
		}
		return nil
	}}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	parent := t.TempDir()
	_, err = (Manager{recoveryCopyHooks: &hooks}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, ErrProjectRecoveryRequired) {
		t.Fatalf("CreateRecoveryCopy error = %v, want recovery required", err)
	}
	if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
		t.Fatalf("mid-copy journal published a package: entries=%#v err=%v", entries, readErr)
	}
}

func TestRecoveryCopyDoesNotDiscardPersistentAttachmentNamedTemporary(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	attachment := filepath.Join(source.Attachments, "author-kept.tmp")
	if err := os.WriteFile(attachment, []byte("important manuscript attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	copyPackage, err := (Manager{}).CreateRecoveryCopy(ctx, source, t.TempDir(), readOnly.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(copyPackage.Attachments, "author-kept.tmp")); err != nil || string(got) != "important manuscript attachment" {
		t.Fatalf("persistent .tmp attachment = %q, error %v", got, err)
	}
}

func TestRecoveryCopyHonorsContextCancellationDuringPersistentCopy(t *testing.T) {
	source, sourceStore := recoveryCopySource(t)
	defer sourceStore.Close()
	if err := os.WriteFile(filepath.Join(source.Attachments, "large.txt"), make([]byte, 512*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(context.Background(), source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooks := recoveryCopyHooks{copyPersistentFile: func(ctx context.Context, source, target string) error {
		cancel()
		return copyRecoveryCopyFile(ctx, source, target, recoveryCopyHooks{})
	}}
	parent := t.TempDir()
	_, err = (Manager{recoveryCopyHooks: &hooks}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateRecoveryCopy error = %v, want context cancellation", err)
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("cancelled copy left package: entries=%#v err=%v", entries, err)
	}
}

func TestRecoveryCopyRejectsMalformedSnapshotOrBranchArchivesBeforePublication(t *testing.T) {
	tests := []struct {
		name  string
		write func(t *testing.T, source ports.ProjectPackage)
	}{
		{
			name: "snapshot metadata",
			write: func(t *testing.T, source ports.ProjectPackage) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(source.Snapshots, "broken.json"), []byte("{not json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "branch catalog",
			write: func(t *testing.T, source ports.ProjectPackage) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(source.Branches, "index.json"), []byte("{not json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			source, sourceStore := recoveryCopySource(t)
			defer sourceStore.Close()
			test.write(t, source)
			readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
			if err != nil {
				t.Fatal(err)
			}
			defer readOnly.Close()
			parent := t.TempDir()
			_, err = (Manager{}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
			if err == nil {
				t.Fatal("CreateRecoveryCopy succeeded with malformed archive")
			}
			if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
				t.Fatalf("malformed archive published a package: entries=%#v err=%v", entries, readErr)
			}
		})
	}
}

func TestRecoveryCopyRejectsBranchHeadDigestMismatchBeforePublication(t *testing.T) {
	ctx := context.Background()
	source, sourceStore := recoveryCopySource(t)
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}
	archive := branchfs.Archive{}
	if _, err := archive.Initialize(source.Branches, "project-copy", "main", source.Database, time.Unix(101, 0)); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(source.Branches, "index.json")
	var catalog map[string]any
	body, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	branches := catalog["branches"].([]any)
	branches[0].(map[string]any)["headDigest"] = "tampered"
	body, err = json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := (sqlite.Factory{}).OpenReadOnly(ctx, source.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	parent := t.TempDir()
	_, err = (Manager{}).CreateRecoveryCopy(ctx, source, parent, readOnly.Backup)
	if !errors.Is(err, branchfs.ErrBranchHeadChanged) {
		t.Fatalf("CreateRecoveryCopy error = %v, want branch head changed", err)
	}
	if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
		t.Fatalf("invalid branch head published a package: entries=%#v err=%v", entries, readErr)
	}
}

func recoveryCopySource(t *testing.T) (ports.ProjectPackage, *sqlite.Store) {
	t.Helper()
	packageInfo, err := (Manager{}).Create(t.TempDir(), "恢复副本")
	if err != nil {
		t.Fatal(err)
	}
	project, err := projectdomain.NewProject("project-copy", "恢复副本", "奇幻", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	created, err := (sqlite.Factory{}).Create(context.Background(), packageInfo.Database, project)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := created.(*sqlite.Store)
	if !ok {
		t.Fatalf("store = %T", created)
	}
	return packageInfo, store
}

func mustRecoveryCopyChapter(t *testing.T, id, body string) projectdomain.Chapter {
	t.Helper()
	chapter, err := projectdomain.NewChapter(id, "project-copy", "第一章", 1, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := chapter.Commit(body, chapter.Revision, time.Unix(102, 0))
	if err != nil {
		t.Fatal(err)
	}
	return updated
}
