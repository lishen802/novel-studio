//go:build darwin || linux

package projectfs

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func lockProjectLease(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
		return errProjectLeaseContended
	}
	return err
}

func unlockProjectLease(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

func isProjectLeaseOpenContention(error) bool {
	return false
}
