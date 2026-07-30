//go:build darwin

package projectfs

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameImportArchiveNoReplace(
	directory *os.File,
	sourceName string,
	targetName string,
) error {
	directoryFD := int(directory.Fd())
	return unix.RenameatxNp(
		directoryFD,
		sourceName,
		directoryFD,
		targetName,
		unix.RENAME_EXCL,
	)
}
