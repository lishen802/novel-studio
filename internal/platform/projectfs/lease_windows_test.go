//go:build windows

package projectfs

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProjectLeaseOpenSharingViolationIsContention(t *testing.T) {
	if !isProjectLeaseOpenContention(windows.ERROR_SHARING_VIOLATION) {
		t.Fatal("sharing violation while opening the lease must be treated as contention")
	}
	if isProjectLeaseOpenContention(windows.ERROR_ACCESS_DENIED) {
		t.Fatal("access denied must retain its original cause")
	}
	if isProjectLeaseOpenContention(errors.New("unrelated open failure")) {
		t.Fatal("unrelated open failure must retain its original cause")
	}
}
