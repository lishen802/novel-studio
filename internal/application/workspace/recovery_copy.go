package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

// RecoveryCopyResult always returns the published package path once a copy is
// validated and atomically published. Project is nil only when the subsequent
// editable open fails; callers can then show the durable path rather than
// deleting the user's only good recovery copy.
type RecoveryCopyResult struct {
	Path    string       `json:"path"`
	Project *ProjectView `json:"project,omitempty"`
}

// CreateProjectRecoveryCopy makes a new package from a strictly read-only
// source. The source store is opened mode=ro exclusively for SQLite's online
// backup API, then closed before the published copy is opened normally with a
// fresh editable OS lease.
func (s *Service) CreateProjectRecoveryCopy(
	ctx context.Context,
	sourcePath, parentDirectory string,
) (RecoveryCopyResult, error) {
	if err := s.beginLifecycleOperation(); err != nil {
		return RecoveryCopyResult{}, err
	}
	defer s.endLifecycleOperation()
	if err := ctx.Err(); err != nil {
		return RecoveryCopyResult{}, err
	}

	source, err := s.packages.Inspect(sourcePath)
	if err != nil {
		return RecoveryCopyResult{}, fmt.Errorf("inspect recovery copy source: %w", err)
	}
	if err := s.validateRecoveryCopyPath(source, parentDirectory); err != nil {
		return RecoveryCopyResult{}, fmt.Errorf("validate recovery copy path: %w", err)
	}
	if err := s.requireRecoveryCopySourceStable(source); err != nil {
		return RecoveryCopyResult{}, err
	}
	readOnly, err := s.stores.OpenReadOnly(ctx, source.Database)
	if err != nil {
		return RecoveryCopyResult{}, fmt.Errorf("open recovery copy source read-only: %w", err)
	}
	published, copyErr := s.packages.CreateRecoveryCopy(ctx, source, parentDirectory, readOnly.Backup)
	closeErr := readOnly.Close()
	if copyErr != nil {
		result := RecoveryCopyResult{Path: published.Root}
		return result, s.handleFilesystemWriteFailureForPackage(
			source.Root,
			joinRecoveryCopyErrors(copyErr, closeErr),
		)
	}

	result := RecoveryCopyResult{Path: published.Root}
	if closeErr != nil {
		return result, joinRecoveryCopyErrors(nil, closeErr)
	}
	project, err := s.openProjectEditable(ctx, published.Root)
	if err != nil {
		return result, fmt.Errorf("open published recovery copy: %w", err)
	}
	result.Project = &project
	return result, nil
}

func (s *Service) validateRecoveryCopyPath(source ports.ProjectPackage, parentDirectory string) error {
	s.mu.RLock()
	guard := s.filesystem
	s.mu.RUnlock()
	if guard == nil {
		return nil
	}
	return guard.ValidateRecoveryCopyPath(source, parentDirectory)
}

// requireRecoveryCopySourceStable is a pure source preflight. In particular,
// it does not reuse any recovery routine: those rename/delete journals and
// are forbidden while creating a copy from a potentially contested project.
func (s *Service) requireRecoveryCopySourceStable(source ports.ProjectPackage) error {
	recoveryRequired, err := s.snapshots.RecoveryRequired(source.Database)
	if err != nil {
		return fmt.Errorf("inspect recovery copy snapshot state: %w", err)
	}
	if recoveryRequired {
		return ErrProjectRecoveryRequired
	}
	recoveryRequired, err = s.stores.RecoveryRequired(source.Database)
	if err != nil {
		return fmt.Errorf("inspect recovery copy database state: %w", err)
	}
	if recoveryRequired {
		return ErrProjectRecoveryRequired
	}
	if s.branches == nil {
		return nil
	}
	recoveryRequired, err = s.branches.RecoveryRequired(source.Branches, source.Database)
	if err != nil {
		return fmt.Errorf("inspect recovery copy branch state: %w", err)
	}
	if recoveryRequired {
		return ErrProjectRecoveryRequired
	}
	return nil
}

func joinRecoveryCopyErrors(copyErr, closeErr error) error {
	if copyErr == nil {
		return fmt.Errorf("close recovery copy source: %w", closeErr)
	}
	if closeErr == nil {
		return fmt.Errorf("create recovery copy: %w", copyErr)
	}
	return fmt.Errorf("create recovery copy: %w", errors.Join(copyErr, closeErr))
}
