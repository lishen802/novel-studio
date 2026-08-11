//go:build windows

package projectfs

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Keep the Windows byte-range lock far beyond the compact JSON metadata. A
// lock over byte zero prevents a contending process from safely reading that
// metadata to explain who holds the project lease.
const projectLeaseLockOffset = 1 << 30

func lockProjectLease(file *os.File) error {
	overlapped := windows.Overlapped{Offset: projectLeaseLockOffset}
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return errProjectLeaseContended
	}
	return err
}

func unlockProjectLease(file *os.File) error {
	overlapped := windows.Overlapped{Offset: projectLeaseLockOffset}
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
}

func isProjectLeaseOpenContention(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
