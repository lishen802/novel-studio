//go:build linux

package projectfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openImportArchiveTemporary(
	importsDirectory *importArchiveDirectory,
	name string,
) (*os.File, error) {
	return importsDirectory.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
}

func commitOpenImportArchiveNoReplace(
	directory *os.File,
	temporary *os.File,
	_ string,
	targetName string,
) error {
	err := unix.Linkat(
		int(temporary.Fd()),
		"",
		int(directory.Fd()),
		targetName,
		unix.AT_EMPTY_PATH,
	)
	if err == nil || !errors.Is(err, unix.ENOENT) {
		return err
	}

	return unix.Linkat(
		unix.AT_FDCWD,
		fmt.Sprintf("/proc/self/fd/%d", temporary.Fd()),
		int(directory.Fd()),
		targetName,
		unix.AT_SYMLINK_FOLLOW,
	)
}
