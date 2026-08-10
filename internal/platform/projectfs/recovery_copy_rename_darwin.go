//go:build darwin

package projectfs

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/unix"
)

func renameRecoveryCopyNoReplace(source, target string) error {
	err := unix.RenamexNp(source, target, unix.RENAME_EXCL)
	if errors.Is(err, unix.EEXIST) {
		return fs.ErrExist
	}
	return err
}
