package projectfs

import (
	"errors"
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
