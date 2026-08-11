//go:build darwin

package projectfs

import (
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"golang.org/x/sys/unix"
)

func TestFilesystemRiskHintUsesNetworkMountCheckForVolumesPath(t *testing.T) {
	root := "/Volumes/team-share/project.zjproject"
	health := filesystemRiskHintWithNetworkVolumeCheck(root, func(path string) bool {
		return path == root
	})
	if health.Level != ports.ProjectFilesystemWarning || health.Category != ports.ProjectFilesystemSyncRisk {
		t.Fatalf("health = %#v", health)
	}
}

func TestIsMacOSNetworkMountDistinguishesLocalMounts(t *testing.T) {
	if !isMacOSNetworkMount(0) {
		t.Fatal("non-local mount was not classified as network")
	}
	if isMacOSNetworkMount(unix.MNT_LOCAL) {
		t.Fatal("local mount was classified as network")
	}
}
