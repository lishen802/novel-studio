package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

func TestReadOnlyWorkspaceRejectsEveryWriteWithoutChangingProjectFiles(t *testing.T) {
	ctx := context.Background()
	editable := NewService(
		projectfs.Manager{}, sqlite.Factory{}, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{},
	)
	project, err := editable.CreateProject(ctx, t.TempDir(), "只读项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := editable.Close(); err != nil {
		t.Fatal(err)
	}

	readOnly := NewService(
		projectfs.Manager{}, sqlite.Factory{}, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{},
	)
	t.Cleanup(func() { _ = readOnly.Close() })
	if _, err := readOnly.OpenProjectReadOnly(ctx, project.Path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func() error
	}{
		{"draft", func() error { return readOnly.SaveDraft(ctx, "chapter", "正文", 1) }},
		{"discard draft", func() error { return readOnly.DiscardDraft(ctx, "chapter", 1) }},
		{"chapter text", func() error { _, err := readOnly.CommitChapter(ctx, "chapter", "正文", 1); return err }},
		{"chapter", func() error { _, err := readOnly.CreateChapter(ctx, "第一章"); return err }},
		{"change set", func() error { _, err := readOnly.ProposeChapterChange(ctx, ChapterChangeInput{}); return err }},
		{"story asset", func() error { _, err := readOnly.CreateOutlineNode(ctx, CreateOutlineNodeInput{}); return err }},
		{"world asset", func() error { _, err := readOnly.CreateWorldRule(ctx, WorldRuleInput{}); return err }},
		{"fact", func() error { _, err := readOnly.CreateFact(ctx, FactInput{}); return err }},
		{"agent generation", func() error { _, err := readOnly.ExecuteAgentRun(ctx, "run"); return err }},
		{"agent cancellation", func() error { return readOnly.CancelAgentRun(ctx, "run") }},
		{"agent runtime settings", func() error {
			_, err := readOnly.SaveAgentRuntimeSettings(ctx, AgentRuntimeSettingsInput{})
			return err
		}},
		{"provider connection save", func() error { _, err := readOnly.SaveProviderConnection(ctx, ProviderConnectionInput{}); return err }},
		{"provider connection delete", func() error { return readOnly.DeleteProviderConnection(ctx, "connection") }},
		{"agent profile", func() error { _, err := readOnly.SaveAgentProfile(ctx, AgentProfileInput{}); return err }},
		{"agent start", func() error { _, err := readOnly.CreateChapterAgentRun(ctx, "chapter", "prompt"); return err }},
		{"agent retry", func() error { _, err := readOnly.RetryAgentRun(ctx, "run"); return err }},
		{"extraction", func() error { _, err := readOnly.ExecuteExtractionRun(ctx, "task"); return err }},
		{"extraction start", func() error { _, err := readOnly.StartExtractionRun(ctx, ExtractionRunInput{}); return err }},
		{"extraction retry", func() error { _, err := readOnly.RetryExtractionRun(ctx, "task"); return err }},
		{"import apply", func() error { _, err := readOnly.ApplyImportJob(ctx, "job", nil); return err }},
		{"import prepare", func() error { _, err := readOnly.PrepareManuscriptImport(ctx, "source.txt"); return err }},
		{"import delete", func() error { return readOnly.DeleteImportJob(ctx, "job") }},
		{"chapter revision restore", func() error { _, err := readOnly.RestoreChapterRevision(ctx, "chapter", 1, 1); return err }},
		{"snapshot create", func() error { _, err := readOnly.CreateSnapshot(ctx, "snapshot"); return err }},
		{"snapshot restore", func() error { _, err := readOnly.RestoreSnapshot(ctx, "snapshot"); return err }},
		{"branch create", func() error { _, err := readOnly.CreateBranch(ctx, "branch", ""); return err }},
		{"branch preview", func() error { _, err := readOnly.PreviewBranchMerge(ctx, "branch"); return err }},
		{"branch confirm", func() error { _, err := readOnly.ConfirmBranchMerge(ctx, "token"); return err }},
		{"branch delete", func() error { return readOnly.DeleteBranch(ctx, "branch") }},
		{"branch switch", func() error { _, err := readOnly.SwitchBranch(ctx, "branch"); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := projectFileHashes(t, project.Path)
			err := test.mutate()
			if !errors.Is(err, ErrWorkspaceReadOnly) {
				t.Fatalf("mutation error = %v, want ErrWorkspaceReadOnly", err)
			}
			after := projectFileHashes(t, project.Path)
			if !equalProjectHashes(before, after) {
				t.Fatalf("project files changed: before=%v after=%v", before, after)
			}
		})
	}
}

func TestReadOnlyWorkspaceCompareBranchesDoesNotWriteBranchHeads(t *testing.T) {
	ctx := context.Background()
	editable := NewService(
		projectfs.Manager{}, sqlite.Factory{}, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{},
	)
	project, err := editable.CreateProject(ctx, t.TempDir(), "分支只读项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := editable.CreateBranch(ctx, "分支", ""); err != nil {
		t.Fatal(err)
	}
	branches, err := editable.Branches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	main := findBranchByName(t, branches, "主分支")
	child := findBranchByName(t, branches, "分支")
	if err := editable.Close(); err != nil {
		t.Fatal(err)
	}

	stores := &readOnlyTrackingStoreFactory{}
	readOnly := NewService(
		projectfs.Manager{}, stores, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{},
	)
	t.Cleanup(func() { _ = readOnly.Close() })
	if _, err := readOnly.OpenProjectReadOnly(ctx, project.Path); err != nil {
		t.Fatal(err)
	}
	stores.openCalls, stores.readOnlyCalls, stores.immutableReadOnlyCalls = 0, 0, 0

	before := projectFileHashes(t, project.Path)
	if _, err := readOnly.CompareBranches(ctx, child.ID, main.ID); err != nil {
		t.Fatalf("CompareBranches() error = %v", err)
	}
	if after := projectFileHashes(t, project.Path); !equalProjectHashes(before, after) {
		t.Fatalf("read-only branch comparison changed branch heads/WAL/SHM: before=%v after=%v", before, after)
	}
	if stores.openCalls != 0 || stores.readOnlyCalls != 0 || stores.immutableReadOnlyCalls != 2 {
		t.Fatalf("branch comparison opens = editable:%d read-only:%d immutable:%d, want 0/0/2", stores.openCalls, stores.readOnlyCalls, stores.immutableReadOnlyCalls)
	}
}

type readOnlyTrackingStoreFactory struct {
	openCalls              int
	readOnlyCalls          int
	immutableReadOnlyCalls int
}

func (f *readOnlyTrackingStoreFactory) Create(ctx context.Context, databasePath string, project projectdomain.Project) (ports.ProjectStore, error) {
	return sqlite.Factory{}.Create(ctx, databasePath, project)
}

func (f *readOnlyTrackingStoreFactory) Open(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	f.openCalls++
	return sqlite.Factory{}.Open(ctx, databasePath)
}

func (f *readOnlyTrackingStoreFactory) OpenReadOnly(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	f.readOnlyCalls++
	return sqlite.Factory{}.OpenReadOnly(ctx, databasePath)
}

func (f *readOnlyTrackingStoreFactory) OpenReadOnlyImmutable(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	f.immutableReadOnlyCalls++
	return sqlite.Factory{}.OpenReadOnlyImmutable(ctx, databasePath)
}

func (f *readOnlyTrackingStoreFactory) RecoveryRequired(databasePath string) (bool, error) {
	return sqlite.Factory{}.RecoveryRequired(databasePath)
}

func TestReadOnlyWorkspaceCancellationFallbackRechecksAfterTransition(t *testing.T) {
	ctx := context.Background()
	service := NewService(
		projectfs.Manager{}, sqlite.Factory{}, license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{},
	)
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.CreateProject(ctx, t.TempDir(), "切换中的项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.requireEditable(); err != nil {
		t.Fatalf("initial editable check: %v", err)
	}

	// This models the boundary after Cancel* passes its initial check but
	// before it takes the fallback read lock to persist a queued job. A
	// workspace replacement can install a read-only active workspace there.
	service.mu.Lock()
	service.active.mode = ports.ProjectAccessReadOnly
	service.mu.Unlock()

	tests := []struct {
		name   string
		cancel func() error
	}{
		{"agent", func() error { return service.cancelAgentRunFallback(ctx, "agent-run") }},
		{"extraction", func() error { return service.cancelExtractionRunFallback(ctx, "extraction-task") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := projectFileHashes(t, project.Path)
			err := test.cancel()
			if !errors.Is(err, ErrWorkspaceReadOnly) {
				t.Fatalf("cancel fallback error = %v, want ErrWorkspaceReadOnly", err)
			}
			if after := projectFileHashes(t, project.Path); !equalProjectHashes(before, after) {
				t.Fatalf("project files changed: before=%v after=%v", before, after)
			}
		})
	}
}

func projectFileHashes(t *testing.T, root string) map[string]string {
	t.Helper()
	hashes := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(contents)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hashes[relative] = hex.EncodeToString(digest[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return hashes
}

func equalProjectHashes(first, second map[string]string) bool {
	if len(first) != len(second) {
		return false
	}
	keys := make([]string, 0, len(first))
	for key := range first {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if first[key] != second[key] {
			return false
		}
	}
	return true
}
