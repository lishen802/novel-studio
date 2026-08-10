package ports

import (
	"errors"
	"fmt"
	"time"
)

var ErrProjectLeaseHeld = errors.New("project lease is held by another process")

type ProjectAccessMode string

const (
	ProjectAccessEditable ProjectAccessMode = "editable"
	ProjectAccessReadOnly ProjectAccessMode = "read_only"
)

type ProjectLeaseMetadata struct {
	PID                int       `json:"pid"`
	Hostname           string    `json:"hostname"`
	ApplicationVersion string    `json:"applicationVersion"`
	StartedAt          time.Time `json:"startedAt"`
	ProjectID          string    `json:"projectId,omitempty"`
}

type ProjectLeaseConflict struct {
	PackagePath string                `json:"packagePath"`
	Holder      *ProjectLeaseMetadata `json:"holder,omitempty"`
	Retryable   bool                  `json:"retryable"`
}

func (c *ProjectLeaseConflict) Error() string {
	if c == nil {
		return ErrProjectLeaseHeld.Error()
	}
	return fmt.Sprintf("%s: %s", ErrProjectLeaseHeld, c.PackagePath)
}

func (c *ProjectLeaseConflict) Is(target error) bool {
	return target == ErrProjectLeaseHeld
}

type ProjectLease interface {
	UpdateMetadata(ProjectLeaseMetadata) error
	Release() error
}

type ProjectLeaseManager interface {
	Acquire(packageInfo ProjectPackage, metadata ProjectLeaseMetadata) (ProjectLease, error)
}
