package workspace

import "errors"

var ErrNoActiveProject = errors.New("no active project")

var (
	ErrAgentRuntimeUnavailable  = errors.New("agent runtime is unavailable")
	ErrAgentStoreUnavailable    = errors.New("active project does not support agent records")
	ErrAgentRunActive           = errors.New("an agent run is already active")
	ErrTokenBudgetExceeded      = errors.New("agent run token budget exceeded")
	ErrImportSourceArchive      = errors.New("manuscript import source could not be prepared")
	ErrImportStoreUnavailable   = errors.New("active project does not support manuscript imports")
	ErrImportJobProjectMismatch = errors.New("import job belongs to a different project")
	ErrImportOperationActive    = errors.New("a manuscript import operation is already active")
	ErrImportWorkspaceRead      = errors.New("manuscript import workspace data could not be read")
	ErrImportAppliedRefresh     = errors.New("manuscript import was applied but workspace refresh failed")
)
