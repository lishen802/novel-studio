package projectfs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

func TestNormalProjectFilesystemHealthHasNoActions(t *testing.T) {
	got := ports.NormalProjectFilesystemHealth()
	if got.Level != ports.ProjectFilesystemNormal || len(got.Actions) != 0 {
		t.Fatalf("health = %#v", got)
	}
}

func TestProjectFilesystemFailureMatchesStableSentinel(t *testing.T) {
	err := &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}
	if !errors.Is(err, ports.ErrProjectFilesystemFailure) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateCreatePathRejectsLongWindowsCompatibleRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), strings.Repeat("segment-", 35))
	err := (Manager{}).ValidateCreatePath(parent, "作品")
	failure := (Manager{}).ClassifyRuntimeError(err)
	if failure == nil || failure.Category != ports.ProjectFilesystemPathTooLong {
		t.Fatalf("error = %v, failure = %#v", err, failure)
	}
}

func TestPortableProjectNameTrimsTrailingWindowsComponents(t *testing.T) {
	got, err := portableProjectName(" 长夜.  ")
	if err != nil || got != "长夜" {
		t.Fatalf("name = %q, error = %v", got, err)
	}
}

func TestPreflightEditableLeavesCanonicalFilesUnchanged(t *testing.T) {
	manager := Manager{}
	pkg, err := manager.Create(t.TempDir(), "预检作品")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pkg.Database, []byte("canonical database bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := projectFileHashes(t, pkg.Root)

	health, err := manager.PreflightEditable(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if health.Level != ports.ProjectFilesystemNormal {
		t.Fatalf("health = %#v", health)
	}
	if after := projectFileHashes(t, pkg.Root); !sameProjectFileHashes(before, after) {
		t.Fatalf("preflight changed project files\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestPreflightEditableWarnsForKnownSyncRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "OneDrive")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	pkg, err := (Manager{}).Create(parent, "同步作品")
	if err != nil {
		t.Fatal(err)
	}

	health, err := (Manager{}).PreflightEditable(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if health.Level != ports.ProjectFilesystemWarning || health.Category != ports.ProjectFilesystemSyncRisk {
		t.Fatalf("health = %#v", health)
	}
}

func TestClassifyFilesystemErrorMapsRuntimeWriteFailures(t *testing.T) {
	for _, test := range []struct {
		err  error
		want ports.ProjectFilesystemCategory
	}{
		{errors.New("database is locked"), ports.ProjectFilesystemBusy},
		{errors.New("disk is full"), ports.ProjectFilesystemDiskFull},
		{errors.New("readonly database"), ports.ProjectFilesystemReadOnly},
		{errors.New("disk I/O error"), ports.ProjectFilesystemIO},
	} {
		got := ClassifyFilesystemError(test.err)
		if got == nil || got.Category != test.want {
			t.Fatalf("%v => %#v, want %q", test.err, got, test.want)
		}
	}
}

func TestClassifyFilesystemErrorMapsWrappedRuntimeWriteFailure(t *testing.T) {
	err := &os.PathError{
		Op:   "write",
		Path: "project.sqlite",
		Err:  fmt.Errorf("commit: %w", errors.New("database is locked")),
	}
	got := ClassifyFilesystemError(err)
	if got == nil || got.Category != ports.ProjectFilesystemBusy {
		t.Fatalf("error = %v, failure = %#v", err, got)
	}
}

func TestFilesystemRiskHintWarnsForRawUNCPath(t *testing.T) {
	health := filesystemRiskHint("//server/share/project.zjproject")
	if health.Level != ports.ProjectFilesystemWarning || health.Category != ports.ProjectFilesystemSyncRisk {
		t.Fatalf("health = %#v", health)
	}
}

func TestFilesystemRiskHintUsesNetworkVolumeCheck(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project.zjproject")
	called := false
	health := filesystemRiskHintWithNetworkVolumeCheck(root, func(path string) bool {
		called = true
		return path == filepath.Clean(root)
	})
	if !called {
		t.Fatal("network volume check was not called")
	}
	if health.Level != ports.ProjectFilesystemWarning || health.Category != ports.ProjectFilesystemSyncRisk {
		t.Fatalf("health = %#v", health)
	}
}

func TestFilesystemRiskHintKeepsLocalPathNormal(t *testing.T) {
	health := filesystemRiskHintWithNetworkVolumeCheck(filepath.Join(t.TempDir(), "project.zjproject"), func(string) bool {
		return false
	})
	if health.Level != ports.ProjectFilesystemNormal || health.Category != "" || len(health.Actions) != 0 {
		t.Fatalf("health = %#v", health)
	}
}

func projectFileHashes(t *testing.T, root string) map[string][sha256.Size]byte {
	t.Helper()
	hashes := make(map[string][sha256.Size]byte)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hashes[filepath.ToSlash(relative)] = sha256.Sum256(body)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return hashes
}

func sameProjectFileHashes(first, second map[string][sha256.Size]byte) bool {
	if len(first) != len(second) {
		return false
	}
	for path, firstHash := range first {
		if second[path] != firstHash {
			return false
		}
	}
	return true
}
