package ports

import "errors"

var ErrProjectFilesystemFailure = errors.New("project filesystem operation failed")

type ProjectFilesystemLevel string

const (
	ProjectFilesystemNormal   ProjectFilesystemLevel = "normal"
	ProjectFilesystemWarning  ProjectFilesystemLevel = "warning"
	ProjectFilesystemDegraded ProjectFilesystemLevel = "degraded"
)

type ProjectFilesystemCategory string

const (
	ProjectFilesystemSyncRisk         ProjectFilesystemCategory = "sync_risk"
	ProjectFilesystemPathTooLong      ProjectFilesystemCategory = "path_too_long"
	ProjectFilesystemSharingViolation ProjectFilesystemCategory = "sharing_violation"
	ProjectFilesystemBusy             ProjectFilesystemCategory = "busy"
	ProjectFilesystemReadOnly         ProjectFilesystemCategory = "read_only"
	ProjectFilesystemDiskFull         ProjectFilesystemCategory = "disk_full"
	ProjectFilesystemIO               ProjectFilesystemCategory = "io"
	ProjectFilesystemCheckpoint       ProjectFilesystemCategory = "checkpoint"
	ProjectFilesystemAtomicReplace    ProjectFilesystemCategory = "atomic_replace"
	ProjectFilesystemPreflight        ProjectFilesystemCategory = "preflight"
)

type ProjectFilesystemAction string

const (
	ProjectFilesystemRetry              ProjectFilesystemAction = "retry"
	ProjectFilesystemCopyText           ProjectFilesystemAction = "copy_text"
	ProjectFilesystemOpenReadOnly       ProjectFilesystemAction = "open_read_only"
	ProjectFilesystemCreateRecoveryCopy ProjectFilesystemAction = "create_recovery_copy"
)

type ProjectFilesystemHealth struct {
	Level    ProjectFilesystemLevel    `json:"level"`
	Category ProjectFilesystemCategory `json:"category,omitempty"`
	Actions  []ProjectFilesystemAction `json:"actions,omitempty"`
}

func NormalProjectFilesystemHealth() ProjectFilesystemHealth {
	return ProjectFilesystemHealth{Level: ProjectFilesystemNormal}
}

type ProjectFilesystemFailure struct {
	Category ProjectFilesystemCategory
}

func (failure *ProjectFilesystemFailure) Error() string {
	return ErrProjectFilesystemFailure.Error()
}

func (failure *ProjectFilesystemFailure) Is(target error) bool {
	return target == ErrProjectFilesystemFailure
}

type ProjectFilesystemGuard interface {
	PreflightEditable(ProjectPackage) (ProjectFilesystemHealth, error)
	ValidateCreatePath(parentDirectory, projectTitle string) error
	ValidateRecoveryCopyPath(source ProjectPackage, parentDirectory string) error
	ClassifyRuntimeError(error) *ProjectFilesystemFailure
}
