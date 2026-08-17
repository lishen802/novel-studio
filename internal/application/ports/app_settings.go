package ports

import (
	"context"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
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
	// ListLatestProfiles 一次查询返回每个方案的当前最新版本，避免上层循环
	// 逐方案拼装事实。
	ListLatestProfiles(ctx context.Context) ([]appsettings.CreativeProfileVersion, error)

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

// ProviderProbeResult 是一次真实连接测试的结果，只包含安全字段，不保存响应正文。
type ProviderProbeResult struct {
	ProviderID        string
	ProviderName      string
	Kind              string
	Model             string
	ProviderRequestID string
	InputTokens       int
	OutputTokens      int
	Latency           time.Duration
}

// ProviderProber 对已保存的连接发起一次真实、最小费用的模型请求，用于验证
// 鉴权、模型名称和当前调用协议可用。它不自动重试到其他连接或模型。
type ProviderProber interface {
	ProbeProvider(ctx context.Context, providerID, model string) (ProviderProbeResult, error)
}

// CreativeProfileResolver 按确切版本解析一个创作方案，返回预算与三个角色实际
// 使用的非敏感连接元数据、模型和生成参数，不包含 API Key。项目创建 AgentRun
// 时据此生成运行冻结快照。
type CreativeProfileResolver interface {
	ResolveCreativeProfile(ctx context.Context, profileID string, version int64) (agentdomain.ResolvedCreativeProfile, error)
}
