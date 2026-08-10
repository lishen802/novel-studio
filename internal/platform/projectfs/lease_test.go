package projectfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

func TestProjectLeaseAcquiresAfterResidualMetadata(t *testing.T) {
	tests := []struct {
		name     string
		contents []byte
	}{
		{
			name:     "valid metadata",
			contents: []byte(`{"pid":123,"hostname":"previous-host","applicationVersion":"0.9.0","startedAt":"2026-08-10T00:00:00Z","projectId":"project-1"}`),
		},
		{name: "malformed metadata", contents: []byte(`not json`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			packageInfo := leaseTestPackage(t)
			if err := os.WriteFile(packageInfo.Lease, test.contents, 0o600); err != nil {
				t.Fatal(err)
			}

			lease, err := (LeaseManager{}).Acquire(packageInfo, leaseTestMetadata("new-holder"))
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			if err := lease.Release(); err != nil {
				t.Fatalf("Release: %v", err)
			}

			if _, err := os.Stat(packageInfo.Lease); err != nil {
				t.Fatalf("lease file was removed: %v", err)
			}
		})
	}
}

func TestProjectLeaseUsesFixedPathAndReleaseIsIdempotent(t *testing.T) {
	packageInfo := leaseTestPackage(t)
	packageInfo.Lease = filepath.Join(t.TempDir(), "redirected.lease")

	lease, err := (LeaseManager{}).Acquire(packageInfo, leaseTestMetadata("holder"))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}

	if _, err := os.Stat(filepath.Join(packageInfo.Root, "project.lease")); err != nil {
		t.Fatalf("fixed lease path was not retained: %v", err)
	}
	if _, err := os.Stat(packageInfo.Lease); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("redirected lease path was used: %v", err)
	}
}

func leaseTestPackage(t *testing.T) ports.ProjectPackage {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project.zjproject")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return ports.ProjectPackage{Root: root, Lease: filepath.Join(root, "project.lease")}
}

func leaseTestMetadata(hostname string) ports.ProjectLeaseMetadata {
	return ports.ProjectLeaseMetadata{
		PID:                1234,
		Hostname:           hostname,
		ApplicationVersion: "test-version",
		StartedAt:          time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
		ProjectID:          "project-1",
	}
}
