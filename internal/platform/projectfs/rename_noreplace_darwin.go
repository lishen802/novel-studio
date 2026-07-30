//go:build darwin

package projectfs

import (
	"errors"
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
	err := unix.Fclonefileat(
		int(temporary.Fd()),
		int(directory.Fd()),
		targetName,
		0,
	)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) {
		return errImportArchivePublicationUnsupported
	}
	return err
}
