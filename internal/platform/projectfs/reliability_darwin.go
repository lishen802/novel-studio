//go:build darwin

package projectfs

import (
	"github.com/lishen802/novel-studio/internal/application/ports"
	"golang.org/x/sys/unix"
)

func classifyPlatformFilesystemError(error) *ports.ProjectFilesystemFailure {
	return nil
}

func isNetworkFilesystem(root string) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(root, &stat); err != nil {
		return false
	}
	return isMacOSNetworkMount(stat.Flags)
}

func isMacOSNetworkMount(flags uint32) bool {
	return flags&unix.MNT_LOCAL == 0
}
