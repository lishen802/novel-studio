package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

func TestSwitchBranchPreservesFilesystemHealthInWorkspaceAndView(t *testing.T) {
	ctx := context.Background()
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)},
		ids.UUIDGenerator{},
		snapshotfs.Archive{},
		branchfs.Archive{},
	)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "分支健康", "奇幻", ""); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateBranch(ctx, "健康分支", "")
	if err != nil {
		t.Fatal(err)
	}
	want := ports.ProjectFilesystemHealth{
		Level:    ports.ProjectFilesystemDegraded,
		Category: ports.ProjectFilesystemSharingViolation,
		Actions:  []ports.ProjectFilesystemAction{ports.ProjectFilesystemRetry},
	}
	service.mu.Lock()
	service.active.filesystemHealth = want
	service.mu.Unlock()

	result, err := service.SwitchBranch(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Project.FilesystemHealth, want) {
		t.Fatalf("switch branch view health = %#v, want %#v", result.Project.FilesystemHealth, want)
	}
	if !reflect.DeepEqual(activeFilesystemHealth(t, service), want) {
		t.Fatalf("active health = %#v, want %#v", activeFilesystemHealth(t, service), want)
	}
}

func TestProjectBranchesIsolateCompleteWorkspaceAndFastForwardMerge(t *testing.T) {
	ctx := context.Background()
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)},
		ids.UUIDGenerator{},
		snapshotfs.Archive{},
		branchfs.Archive{},
	)
	parentDir := t.TempDir()
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.CreateProject(ctx, parentDir, "凡人修仙", "修仙", "")
	if err != nil {
		t.Fatal(err)
	}
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}
	chapter, err = service.CommitChapter(ctx, chapter.ID, "主线正文", chapter.Revision)
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateBranch(ctx, "黑化线", "主角接受魔契")
	if err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}
	if created.Active || created.ParentID == "" {
		t.Fatalf("CreateBranch() = %#v", created)
	}
	switched, err := service.SwitchBranch(ctx, created.ID)
	if err != nil {
		t.Fatalf("SwitchBranch(child) error = %v", err)
	}
	if switched.ActiveBranchID != created.ID || switched.Project.Path != project.Path {
		t.Fatalf("SwitchBranch(child) = %#v", switched)
	}
	childChapter := switched.Chapters[0]
	childChapter, err = service.CommitChapter(
		ctx,
		childChapter.ID,
		"黑化线正文",
		childChapter.Revision,
	)
	if err != nil {
		t.Fatal(err)
	}

	branches, err := service.Branches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mainBranch := findBranchByName(t, branches, "主分支")
	mainWorkspace, err := service.SwitchBranch(ctx, mainBranch.ID)
	if err != nil {
		t.Fatalf("SwitchBranch(main) error = %v", err)
	}
	if got := mainWorkspace.Chapters[0].Body; got != "主线正文" {
		t.Fatalf("main chapter body = %q", got)
	}

	preview, err := service.PreviewBranchMerge(ctx, created.ID)
	if err != nil {
		t.Fatalf("PreviewBranchMerge() error = %v", err)
	}
	if preview.Token == "" || preview.Comparison.ModifiedChapters != 1 {
		t.Fatalf("PreviewBranchMerge() = %#v", preview)
	}
	merged, err := service.ConfirmBranchMerge(ctx, preview.Token)
	if err != nil {
		t.Fatalf("ConfirmBranchMerge() error = %v", err)
	}
	if got := merged.Chapters[0].Body; got != "黑化线正文" {
		t.Fatalf("merged chapter body = %q", got)
	}

	snapshots, err := service.Snapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].Kind != string(revisiondomain.SnapshotRecovery) {
		t.Fatalf("merge recovery snapshots = %#v", snapshots)
	}
}

func TestProjectBranchMergeBlocksWhenParentChangedAfterFork(t *testing.T) {
	ctx := context.Background()
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)},
		ids.UUIDGenerator{},
		snapshotfs.Archive{},
		branchfs.Archive{},
	)
	parentDir := t.TempDir()
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, parentDir, "凡人修仙", "修仙", ""); err != nil {
		t.Fatal(err)
	}
	chapter, _ := service.CreateChapter(ctx, "第一章")
	child, err := service.CreateBranch(ctx, "远行线", "")
	if err != nil {
		t.Fatal(err)
	}
	childWorkspace, err := service.SwitchBranch(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	childChapter := childWorkspace.Chapters[0]
	if _, err := service.CommitChapter(
		ctx,
		childChapter.ID,
		"子分支修改",
		childChapter.Revision,
	); err != nil {
		t.Fatal(err)
	}
	branches, err := service.Branches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mainBranch := findBranchByName(t, branches, "主分支")
	mainWorkspace, err := service.SwitchBranch(ctx, mainBranch.ID)
	if err != nil {
		t.Fatal(err)
	}
	chapter = mainWorkspace.Chapters[0]
	chapter, err = service.CommitChapter(ctx, chapter.ID, "主分支后来修改", chapter.Revision)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.PreviewBranchMerge(ctx, child.ID); !errors.Is(err, revisiondomain.ErrBranchDiverged) {
		t.Fatalf("PreviewBranchMerge() error = %v, want ErrBranchDiverged", err)
	}
}

func TestProjectBranchMergeRevalidatesPreviewBeforeCreatingRecoveryPoint(t *testing.T) {
	ctx := context.Background()
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)},
		ids.UUIDGenerator{},
		snapshotfs.Archive{},
		branchfs.Archive{},
	)
	parentDir := t.TempDir()
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, parentDir, "凡人修仙", "修仙", ""); err != nil {
		t.Fatal(err)
	}
	chapter, _ := service.CreateChapter(ctx, "第一章")
	child, _ := service.CreateBranch(ctx, "远行线", "")
	childWorkspace, _ := service.SwitchBranch(ctx, child.ID)
	childChapter := childWorkspace.Chapters[0]
	if _, err := service.CommitChapter(
		ctx,
		childChapter.ID,
		"子分支修改",
		childChapter.Revision,
	); err != nil {
		t.Fatal(err)
	}
	branches, _ := service.Branches(ctx)
	mainBranch := findBranchByName(t, branches, "主分支")
	mainWorkspace, _ := service.SwitchBranch(ctx, mainBranch.ID)
	preview, err := service.PreviewBranchMerge(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	chapter = mainWorkspace.Chapters[0]
	if _, err := service.CommitChapter(
		ctx,
		chapter.ID,
		"预览后的主分支修改",
		chapter.Revision,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ConfirmBranchMerge(ctx, preview.Token); !errors.Is(err, revisiondomain.ErrBranchDiverged) &&
		!errors.Is(err, revisiondomain.ErrStaleBranchMerge) {
		t.Fatalf("ConfirmBranchMerge() error = %v", err)
	}
	snapshots, err := service.Snapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("stale merge created recovery snapshot: %#v", snapshots)
	}
}

func findBranchByName(t *testing.T, branches []BranchView, name string) BranchView {
	t.Helper()
	for _, branch := range branches {
		if branch.Name == name {
			return branch
		}
	}
	t.Fatalf("branch %q not found in %#v", name, branches)
	return BranchView{}
}
