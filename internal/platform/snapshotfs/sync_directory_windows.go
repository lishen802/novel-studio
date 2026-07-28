//go:build windows

package snapshotfs

// Windows does not support syncing a directory handle through os.File.Sync.
// The snapshot files themselves are synced before each atomic rename.
func syncDirectory(string) error {
	return nil
}
