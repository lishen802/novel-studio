package ports

import (
	"context"
	"errors"
)

// ErrRecoveryCopySourceChanged marks a stable, retryable failure: a source
// persistent file changed between the two manifests used to assemble a
// recovery copy.
var ErrRecoveryCopySourceChanged = errors.New("project changed while recovery copy was being created")

// ErrProjectRecoveryRequired means an interrupted operation is present and
// must be repaired through the editable recovery workflow before any recovery
// copy can safely be assembled.
var ErrProjectRecoveryRequired = errors.New("project recovery is required")

type ProjectPackage struct {
	Root        string
	Database    string
	Attachments string
	Exports     string
	Imports     string
	Recovery    string
	Snapshots   string
	Branches    string
	Lease       string
}

type ProjectPackageManager interface {
	Create(parentDirectory, projectTitle string) (ProjectPackage, error)
	Inspect(packagePath string) (ProjectPackage, error)
	PrepareEditable(packageInfo ProjectPackage) error
	CreateRecoveryCopy(ctx context.Context, source ProjectPackage, parentDirectory string, snapshot DatabaseSnapshot) (ProjectPackage, error)
}

// DatabaseSnapshot creates a consistent SQLite image at targetPath. Callers
// must provide an implementation backed by a read-only source connection.
// The target must not exist before it is called.
type DatabaseSnapshot func(ctx context.Context, targetPath string) error
