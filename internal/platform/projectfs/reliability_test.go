package projectfs

import (
	"errors"
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
