//go:build windows

package sqlite

// syncDirectory is a no-op on Windows. The Go runtime and Windows file
// systems do not expose a portable directory handle sync that compares to
// fsync on Unix. Each staged file is Synced before the atomic rename, and
// rename ordering guarantees the activation sequence is crash-safe. Any
// real I/O error from file Sync or rename still surfaces to the caller.
func syncDirectory(_ string) error { return nil }
