//go:build !windows

package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
)

// syncDirectory flushes directory metadata for the directory containing path
// on Unix-like systems. It opens the parent directory and calls fsync so that
// renames and file creations are durable after a crash. macOS and Linux both
// support fsync on directory handles.
func syncDirectory(path string) error {
	dir := filepath.Dir(path)
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
