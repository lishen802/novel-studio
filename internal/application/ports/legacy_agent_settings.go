package ports

import (
	"context"
)

// LegacyProvider is a non-sensitive provider connection from a project database.
// It carries only the metadata and the existing credential reference; the API
// key value never leaves the OS credential vault.
type LegacyProvider struct {
	ID            string
	Name          string
	Kind          string
	BaseURL       string
	CredentialRef string
}

// LegacyRoleBinding is a role binding from a project database.
type LegacyRoleBinding struct {
	Role            string
	ConnectionID    string
	Model           string
	MaxOutputTokens int
	Temperature     float64
	Enabled         bool
}

// LegacyAgentSettingsSnapshot is the export of a project's agent settings,
// used for one-way migration to the application settings database. It contains
// only non-sensitive metadata and existing credential references.
type LegacyAgentSettingsSnapshot struct {
	ProjectID       string
	ProjectTitle    string
	Connections     []LegacyProvider
	Profiles        []LegacyRoleBinding
	MaxRunTokens    int
	MaxContextChars int
	SourceDigest    string
}

// LegacyAgentSettingsSource is implemented by the workspace service to export
// a project's agent settings without reading credential values.
type LegacyAgentSettingsSource interface {
	ExportLegacyAgentSettings(ctx context.Context, projectID string) (LegacyAgentSettingsSnapshot, error)
}

// LegacyImportResult is the outcome of a one-way migration of a project's agent
// settings into the application settings database.
type LegacyImportResult struct {
	ProfileID      string
	ProfileVersion int64
	ProviderIDs    []string
}

// LegacySettingsImporter is implemented by the application settings service to
// import a project's legacy agent settings into the application settings
// database. The import is idempotent on (project_id, source_digest).
type LegacySettingsImporter interface {
	ImportLegacyProject(ctx context.Context, snapshot LegacyAgentSettingsSnapshot) (LegacyImportResult, error)
}
