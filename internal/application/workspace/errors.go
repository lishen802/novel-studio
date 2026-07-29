package workspace

import "errors"

var ErrNoActiveProject = errors.New("no active project")

var (
	ErrAgentRuntimeUnavailable = errors.New("agent runtime is unavailable")
	ErrAgentStoreUnavailable   = errors.New("active project does not support agent records")
	ErrAgentRunActive          = errors.New("an agent run is already active")
)
