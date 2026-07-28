package branchfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestInitializeCreatesExternalMainBranchRegistryAndFullHead(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	writeTestFile(t, databasePath, "main database")
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)

	catalog, err := (Archive{}).Initialize(
		filepath.Join(root, "branches"),
		"project-id",
		"main-id",
		databasePath,
		now,
	)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if catalog.ActiveBranchID != "main-id" || len(catalog.Branches) != 1 {
		t.Fatalf("Initialize() = %#v", catalog)
	}
	assertTestFile(t, filepath.Join(root, "branches", "heads", "main-id.sqlite"), "main database")
	if _, err := os.Stat(filepath.Join(root, "branches", "index.json")); err != nil {
		t.Fatalf("index.json: %v", err)
	}
}

func TestInitializeIsBackwardCompatibleAndIdempotent(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	writeTestFile(t, databasePath, "legacy database")
	archive := Archive{}
	directory := filepath.Join(root, "branches")
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)

	first, err := archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := archive.Initialize(directory, "project-id", "other-id", databasePath, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if second.ActiveBranchID != first.ActiveBranchID || len(second.Branches) != 1 {
		t.Fatalf("second Initialize() = %#v", second)
	}
}

func TestCheckpointAndForkKeepIndependentCompleteHeads(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	writeTestFile(t, databasePath, "baseline")
	directory := filepath.Join(root, "branches")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)

	child, err := archive.Fork(
		directory,
		"child-id",
		"黑化线",
		"主角接受魔契",
		"main-id",
		databasePath,
		now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}
	if child.ParentID != "main-id" || child.UpstreamDigest == "" {
		t.Fatalf("Fork() = %#v", child)
	}

	writeTestFile(t, databasePath, "child changed")
	updated, err := archive.Checkpoint(directory, "child-id", databasePath, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if updated.Generation != 2 {
		t.Fatalf("Checkpoint() generation = %d", updated.Generation)
	}
	assertTestFile(t, filepath.Join(directory, "heads", "main-id.sqlite"), "baseline")
	assertTestFile(t, filepath.Join(directory, "heads", "child-id.sqlite"), "child changed")
}

func TestSwitchCheckpointsCurrentDatabaseAndActivatesTarget(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	writeTestFile(t, databasePath, "main")
	directory := filepath.Join(root, "branches")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	_, _ = archive.Fork(directory, "child-id", "支线", "", "main-id", databasePath, now)

	writeTestFile(t, databasePath, "main changed")
	catalog, err := archive.Switch(
		directory,
		databasePath,
		"child-id",
		databasePath,
		now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}
	if catalog.ActiveBranchID != "child-id" {
		t.Fatalf("Switch() active = %q", catalog.ActiveBranchID)
	}
	assertTestFile(t, databasePath, "main")
	assertTestFile(t, filepath.Join(directory, "heads", "main-id.sqlite"), "main changed")
}

func TestSwitchRejectsAHeadChangedOutsideTheBranchArchive(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	writeTestFile(t, databasePath, "main")
	directory := filepath.Join(root, "branches")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	_, _ = archive.Fork(directory, "child-id", "支线", "", "main-id", databasePath, now)
	writeTestFile(t, archive.HeadPath(directory, "child-id"), "tampered")

	if _, err := archive.Switch(
		directory,
		databasePath,
		"child-id",
		databasePath,
		now,
	); !errors.Is(err, ErrBranchHeadChanged) {
		t.Fatalf("Switch() error = %v, want ErrBranchHeadChanged", err)
	}
	assertTestFile(t, databasePath, "main")
}

func TestRecoverInterruptedSwitchUsesCommittedCatalogAsDecisionPoint(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	directory := filepath.Join(root, "branches")
	writeTestFile(t, databasePath, "current")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)

	oldCatalog, err := archive.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	newCatalog := oldCatalog
	newCatalog.Revision++
	writeTestFile(t, databasePath+".branch-backup", "current")
	writeTestFile(t, databasePath, "target")
	if err := archive.writeJournal(directory, operationJournal{
		Version:     journalVersion,
		Database:    databasePath,
		OldRevision: oldCatalog.Revision,
		NewRevision: newCatalog.Revision,
	}); err != nil {
		t.Fatal(err)
	}

	if err := archive.RecoverInterruptedOperation(directory, databasePath); err != nil {
		t.Fatalf("RecoverInterruptedOperation() error = %v", err)
	}
	assertTestFile(t, databasePath, "current")
}

func TestRecoverInterruptedOperationKeepsDatabaseAndHeadWhenCatalogCommitted(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	directory := filepath.Join(root, "branches")
	writeTestFile(t, databasePath, "current")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	oldCatalog, _ := archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	headPath := archive.HeadPath(directory, "main-id")
	newCatalog := oldCatalog
	newCatalog.Revision++
	writeTestFile(t, databasePath+".branch-backup", "current")
	writeTestFile(t, databasePath, "merged")
	writeTestFile(t, headPath+".branch-backup", "current")
	writeTestFile(t, headPath, "merged")
	if err := archive.writeJournal(directory, operationJournal{
		Version:     journalVersion,
		Database:    databasePath,
		Head:        headPath,
		OldRevision: oldCatalog.Revision,
		NewRevision: newCatalog.Revision,
	}); err != nil {
		t.Fatal(err)
	}
	if err := archive.writeCatalog(directory, newCatalog); err != nil {
		t.Fatal(err)
	}

	if err := archive.RecoverInterruptedOperation(directory, databasePath); err != nil {
		t.Fatalf("RecoverInterruptedOperation() error = %v", err)
	}
	assertTestFile(t, databasePath, "merged")
	assertTestFile(t, headPath, "merged")
	for _, path := range []string{
		databasePath + ".branch-backup",
		headPath + ".branch-backup",
		journalPath(directory),
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists: %v", path, err)
		}
	}
}

func TestMergeRejectsDivergenceAndFastForwardsCompleteProject(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	directory := filepath.Join(root, "branches")
	writeTestFile(t, databasePath, "main")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	_, _ = archive.Fork(directory, "child-id", "支线", "", "main-id", databasePath, now)

	if _, err := archive.Switch(directory, databasePath, "child-id", databasePath, now); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, databasePath, "child changed")
	if _, err := archive.Checkpoint(directory, "child-id", databasePath, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Switch(directory, databasePath, "main-id", databasePath, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	plan, err := archive.PlanMerge(directory, "child-id", "main-id")
	if err != nil {
		t.Fatalf("PlanMerge() error = %v", err)
	}
	catalog, err := archive.Merge(
		directory,
		databasePath,
		plan,
		databasePath,
		now.Add(3*time.Hour),
	)
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	assertTestFile(t, databasePath, "child changed")
	target, _ := catalog.Find("main-id")
	source, _ := catalog.Find("child-id")
	if target.HeadDigest != source.HeadDigest || source.UpstreamDigest != target.HeadDigest {
		t.Fatalf("Merge() branches = source %#v target %#v", source, target)
	}

	writeTestFile(t, databasePath, "main diverged")
	if _, err := archive.Checkpoint(directory, "main-id", databasePath, now.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.PlanMerge(directory, "child-id", "main-id"); !errors.Is(err, revisiondomain.ErrBranchUnchanged) {
		t.Fatalf("PlanMerge() after merge error = %v", err)
	}
}

func TestDeleteProtectsMainActiveAndParentBranches(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "project.db")
	directory := filepath.Join(root, "branches")
	writeTestFile(t, databasePath, "main")
	archive := Archive{}
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	_, _ = archive.Initialize(directory, "project-id", "main-id", databasePath, now)
	_, _ = archive.Fork(directory, "child-id", "支线", "", "main-id", databasePath, now)
	_, _ = archive.Fork(directory, "grandchild-id", "支线二", "", "child-id", databasePath, now)

	if err := archive.Delete(directory, "main-id"); !errors.Is(err, ErrProtectedBranch) {
		t.Fatalf("Delete(main) error = %v", err)
	}
	if err := archive.Delete(directory, "child-id"); !errors.Is(err, ErrBranchHasChildren) {
		t.Fatalf("Delete(parent) error = %v", err)
	}
	if err := archive.Delete(directory, "grandchild-id"); err != nil {
		t.Fatalf("Delete(leaf) error = %v", err)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("%s = %q, want %q", path, body, want)
	}
}
