//go:build windows

package projectfs

import (
	"errors"
	"io/fs"
	"syscall"

	"golang.org/x/sys/windows"
)

func renameRecoveryCopyNoReplace(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, 0)
	if errors.Is(err, syscall.ERROR_ALREADY_EXISTS) || errors.Is(err, syscall.ERROR_FILE_EXISTS) {
		return fs.ErrExist
	}
	return err
}
