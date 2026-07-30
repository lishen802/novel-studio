//go:build darwin

package projectfs

import (
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
	return unix.Fclonefileat(
		int(temporary.Fd()),
		int(directory.Fd()),
		targetName,
		0,
	)
}
