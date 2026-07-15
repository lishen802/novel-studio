package license

import "context"

type Capability string

const (
	CreateProject    Capability = "create_project"
	EditManuscript   Capability = "edit_manuscript"
	ImportManuscript Capability = "import_manuscript"
	RunAI            Capability = "run_ai"
)

type Service interface {
	Require(ctx context.Context, capability Capability) error
}
