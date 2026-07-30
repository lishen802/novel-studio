//go:build linux

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
	return unix.Renameat2(
		directoryFD,
		sourceName,
		directoryFD,
		targetName,
		unix.RENAME_NOREPLACE,
	)
}
