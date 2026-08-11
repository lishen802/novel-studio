//go:build windows

package projectfs

// Windows does not expose a portable equivalent of fsync for directory
// handles. All recovery-copy files are synced before their atomic rename;
// attempting FlushFileBuffers on a directory returns ERROR_ACCESS_DENIED on
// supported Windows filesystems and prevents a valid copy from publishing.
func syncRecoveryCopyDirectory(_ string) error { return nil }
