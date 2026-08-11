//go:build !windows && !darwin

package projectfs

import (
	"github.com/lishen802/novel-studio/internal/application/ports"
)

func classifyPlatformFilesystemError(error) *ports.ProjectFilesystemFailure {
	return nil
}

func isNetworkFilesystem(string) bool {
	return false
}
