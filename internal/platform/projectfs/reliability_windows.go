//go:build windows

package projectfs

import (
	"errors"
	"path/filepath"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"golang.org/x/sys/windows"
)

func classifyPlatformFilesystemError(err error) *ports.ProjectFilesystemFailure {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemSharingViolation}
	}
	return nil
}

func isNetworkFilesystem(root string) bool {
	return isWindowsNetworkVolume(root, windowsDriveType)
}

func isWindowsNetworkVolume(path string, driveType func(string) uint32) bool {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return false
	}
	return driveType(volume+"\\") == windows.DRIVE_REMOTE
}

func windowsDriveType(root string) uint32 {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return windows.DRIVE_UNKNOWN
	}
	return windows.GetDriveType(path)
}
