package projectfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

const projectLeaseFileName = "project.lease"

var errProjectLeaseContended = errors.New("project lease OS lock is held")

// LeaseManager acquires the fixed project lease file for editable access.
// Lock ownership is determined exclusively by the operating system; metadata
// is only a best-effort diagnostic for a conflicting process.
type LeaseManager struct{}

func (LeaseManager) Acquire(packageInfo ports.ProjectPackage, metadata ports.ProjectLeaseMetadata) (ports.ProjectLease, error) {
	leasePath := filepath.Join(filepath.Clean(packageInfo.Root), projectLeaseFileName)
	file, err := os.OpenFile(leasePath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		if isProjectLeaseOpenContention(err) {
			return nil, &ports.ProjectLeaseConflict{PackagePath: packageInfo.Root, Retryable: true}
		}
		return nil, fmt.Errorf("open project lease: %w", err)
	}

	if err := lockProjectLease(file); err != nil {
		if errors.Is(err, errProjectLeaseContended) {
			holder := readProjectLeaseMetadata(file)
			closeErr := file.Close()
			if closeErr != nil {
				return nil, errors.Join(
					&ports.ProjectLeaseConflict{PackagePath: packageInfo.Root, Holder: holder, Retryable: true},
					fmt.Errorf("close contended project lease: %w", closeErr),
				)
			}
			return nil, &ports.ProjectLeaseConflict{PackagePath: packageInfo.Root, Holder: holder, Retryable: true}
		}
		_ = file.Close()
		return nil, fmt.Errorf("lock project lease: %w", err)
	}

	lease := &projectLease{file: file}
	if err := lease.UpdateMetadata(metadata); err != nil {
		releaseErr := lease.Release()
		return nil, errors.Join(fmt.Errorf("write project lease metadata: %w", err), releaseErr)
	}
	return lease, nil
}

type projectLease struct {
	mu   sync.Mutex
	file *os.File
}

func (lease *projectLease) UpdateMetadata(metadata ports.ProjectLeaseMetadata) error {
	lease.mu.Lock()
	defer lease.mu.Unlock()

	if lease.file == nil {
		return os.ErrClosed
	}
	return writeProjectLeaseMetadata(lease.file, metadata)
}

func (lease *projectLease) Release() error {
	lease.mu.Lock()
	file := lease.file
	lease.file = nil
	lease.mu.Unlock()

	if file == nil {
		return nil
	}
	unlockErr := unlockProjectLease(file)
	closeErr := file.Close()
	return errors.Join(unlockErr, closeErr)
}

func writeProjectLeaseMetadata(file *os.File, metadata ports.ProjectLeaseMetadata) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	if err := json.NewEncoder(file).Encode(metadata); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}

func readProjectLeaseMetadata(file *os.File) *ports.ProjectLeaseMetadata {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(file, 64*1024))
	if err != nil {
		return nil
	}
	var metadata ports.ProjectLeaseMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil
	}
	return &metadata
}

var _ ports.ProjectLeaseManager = LeaseManager{}
