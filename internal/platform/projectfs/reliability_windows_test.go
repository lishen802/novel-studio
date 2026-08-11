//go:build windows

package projectfs

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"golang.org/x/sys/windows"
)

func TestClassifyFilesystemErrorMapsWrappedWindowsContention(t *testing.T) {
	for _, err := range []error{windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION} {
		err := &os.PathError{
			Op:   "rename",
			Path: "project.sqlite",
			Err:  fmt.Errorf("replace: %w", err),
		}
		failure := ClassifyFilesystemError(err)
		if failure == nil || failure.Category != ports.ProjectFilesystemSharingViolation {
			t.Fatalf("error = %v, failure = %#v", err, failure)
		}
	}
}

func TestIsWindowsNetworkVolumeRecognizesMappedRemoteDrive(t *testing.T) {
	calledWith := ""
	if !isWindowsNetworkVolume("Z:\\novels\\project.zjproject", func(root string) uint32 {
		calledWith = root
		return windows.DRIVE_REMOTE
	}) {
		t.Fatal("mapped remote drive was not classified as network")
	}
	if calledWith != "Z:\\" {
		t.Fatalf("GetDriveType root = %q", calledWith)
	}
}

func TestIsWindowsNetworkVolumeRejectsFixedDrive(t *testing.T) {
	if isWindowsNetworkVolume("C:\\novels\\project.zjproject", func(string) uint32 {
		return windows.DRIVE_FIXED
	}) {
		t.Fatal("fixed drive was classified as network")
	}
}

func TestWindowsContentionErrorsRemainComparable(t *testing.T) {
	if !errors.Is(fmt.Errorf("wrapped: %w", windows.ERROR_SHARING_VIOLATION), windows.ERROR_SHARING_VIOLATION) {
		t.Fatal("sharing violation no longer participates in error chains")
	}
}
