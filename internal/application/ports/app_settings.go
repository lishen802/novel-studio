package ports

import (
	"context"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/appsettings"
)

// AppSettingsRepository is the persistence port for the application settings
// database. It stores only non-sensitive metadata; API keys never enter this
// repository.
type AppSettingsRepository interface {
	Preferences(ctx context.Context) (appsettings.Preferences, error)
	SavePreferences(ctx context.Context, prefs appsettings.Preferences) error

	UpsertProvider(ctx context.Context, p appsettings.ProviderConnection) error
	Provider(ctx context.Context, id string) (appsettings.ProviderConnection, error)
	Providers(ctx context.Context) ([]appsettings.ProviderConnection, error)
	DeleteProvider(ctx context.Context, id string) error
	ProviderInUse(ctx context.Context, providerID string) (bool, error)

	PublishProfileVersion(ctx context.Context, v appsettings.CreativeProfileVersion) error
	ProfileVersion(ctx context.Context, profileID string, version int64) (appsettings.CreativeProfileVersion, error)
	LatestProfileVersion(ctx context.Context, profileID string) (appsettings.CreativeProfileVersion, error)

	RecordRecentProject(ctx context.Context, rp appsettings.RecentProject) error
	RecentProjects(ctx context.Context, limit int) ([]appsettings.RecentProject, error)
	MarkProjectMissing(ctx context.Context, projectID string, missing bool) error

	RecordLegacyImport(ctx context.Context, projectID, sourceDigest, profileID string, profileVersion int64, now time.Time) error
	LegacyImport(ctx context.Context, projectID, sourceDigest string) (profileID string, profileVersion int64, err error)
}

// CredentialStore is already defined in agent_runtime.go and reused here.
// The settings service depends on the same port so credentials stay in the
// OS vault, not in the settings database.

// ProjectDefaults provides read-only access to the current application default
// profile selection. Projects pin a specific version at creation time and are
// not implicitly upgraded when the default changes.
type ProjectDefaults interface {
	DefaultProfileSelection(ctx context.Context) (ProjectProfileSelection, error)
}

// ProjectProfileSelection is the pinned profile version and optional role
// overrides for a project.
type ProjectProfileSelection struct {
	ProfileID string `json:"profileId"`
	Version   int64  `json:"version"`
}
