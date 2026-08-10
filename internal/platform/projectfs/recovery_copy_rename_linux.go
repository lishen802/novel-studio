//go:build linux

package projectfs

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/unix"
)

func renameRecoveryCopyNoReplace(source, target string) error {
	err := unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, target, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EEXIST) {
		return fs.ErrExist
	}
	return err
}
