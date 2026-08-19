package appsettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/agent"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
)

var (
	ErrProviderInUse       = errors.New("provider is referenced by a creative profile")
	ErrProfileNotFound     = errors.New("creative profile not found")
	ErrProviderNotFound    = errors.New("provider not found")
	ErrInvalidProfileInput = errors.New("invalid creative profile input")
	// ErrCredentialRollback 表示元数据写入失败后旧凭据未能自动恢复；此时连接
	// 显示为"凭据需要重新输入"，需要作者重新保存 API Key。
	ErrCredentialRollback = errors.New("credential rollback failed")
	// ErrCredentialMissing 表示 API Key 未保存或凭据库不可用。
	ErrCredentialMissing = errors.New("provider credential missing")
)

// ProviderInput carries the data needed to create or update a provider
// connection. The APIKey is never persisted to the settings database; it is
// stored only in the OS credential vault.
type ProviderInput struct {
	Name    string
	Kind    agent.ProviderKind
	BaseURL string
	APIKey  string
}

// ProfileInput carries the data needed to publish a new creative profile
// version.
type ProfileInput struct {
	ProfileID       string
	Name            string
	Roles           []RoleInput
	MaxRunTokens    int
	MaxContextChars int
}

// RoleInput is a single role binding in a profile input.
type RoleInput struct {
	Role            agent.AgentRole
	ConnectionID    string
	Model           string
	MaxOutputTokens int
	Temperature     float64
	Enabled         bool
}

// Service is the application service for application-level settings. It
// coordinates the settings repository, credential vault, and profile versioning.
type Service struct {
	repo        ports.AppSettingsRepository
	credentials ports.CredentialStore
	providers   ports.ModelProviderFactory
	clock       ports.Clock
	ids         ports.IDGenerator
}

func NewService(repo ports.AppSettingsRepository, credentials ports.CredentialStore, clock ports.Clock, ids ports.IDGenerator) *Service {
	return &Service{repo: repo, credentials: credentials, clock: clock, ids: ids}
}

// ConfigureProviders installs the model provider factory used for real
// connection tests. It is separate from NewService so existing callers keep
// their construction contract.
func (s *Service) ConfigureProviders(providers ports.ModelProviderFactory) {
	s.providers = providers
}

func (s *Service) Preferences(ctx context.Context) (appdomain.Preferences, error) {
	return s.repo.Preferences(ctx)
}

func (s *Service) SavePreferences(ctx context.Context, prefs appdomain.Preferences) error {
	return s.repo.SavePreferences(ctx, prefs)
}

func (s *Service) Providers(ctx context.Context) ([]appdomain.ProviderConnection, error) {
	return s.repo.Providers(ctx)
}

func (s *Service) SaveProvider(ctx context.Context, input ProviderInput) (appdomain.ProviderConnection, error) {
	now := s.clock.Now()
	id := s.ids.NewID()
	credentialRef := "app-provider:" + id

	if err := s.credentials.Save(ctx, credentialRef, input.APIKey); err != nil {
		return appdomain.ProviderConnection{}, fmt.Errorf("save credential: %w", err)
	}

	conn, err := appdomain.NewProviderConnection(id, input.Name, input.Kind, input.BaseURL, credentialRef, now)
	if err != nil {
		_ = s.credentials.Delete(ctx, credentialRef)
		return appdomain.ProviderConnection{}, err
	}

	if err := s.repo.UpsertProvider(ctx, conn); err != nil {
		_ = s.credentials.Delete(ctx, credentialRef)
		return appdomain.ProviderConnection{}, fmt.Errorf("save provider metadata: %w", err)
	}

	return conn, nil
}

func (s *Service) UpdateProvider(ctx context.Context, providerID string, input ProviderInput) (appdomain.ProviderConnection, error) {
	existing, err := s.repo.Provider(ctx, providerID)
	if err != nil {
		return appdomain.ProviderConnection{}, ErrProviderNotFound
	}

	// 读取旧凭据，供元数据写入失败时恢复。
	oldSecret, oldErr := s.credentials.Get(ctx, existing.CredentialRef)

	now := s.clock.Now()
	conn, err := appdomain.NewProviderConnection(existing.ID, input.Name, input.Kind, input.BaseURL, existing.CredentialRef, now)
	if err != nil {
		return appdomain.ProviderConnection{}, err
	}
	conn.CreatedAt = existing.CreatedAt

	secretChanged := false
	if input.APIKey != "" {
		if err := s.credentials.Save(ctx, existing.CredentialRef, input.APIKey); err != nil {
			return appdomain.ProviderConnection{}, fmt.Errorf("update credential: %w", err)
		}
		secretChanged = true
	}

	if err := s.repo.UpsertProvider(ctx, conn); err != nil {
		// 元数据写入失败时恢复旧凭据；恢复失败返回独立的凭据回滚错误。
		if secretChanged {
			if rollbackErr := s.restoreCredential(ctx, existing.CredentialRef, oldSecret, oldErr); rollbackErr != nil {
				return appdomain.ProviderConnection{}, fmt.Errorf("%w: %v", ErrCredentialRollback, rollbackErr)
			}
		}
		return appdomain.ProviderConnection{}, fmt.Errorf("update provider metadata: %w", err)
	}

	return conn, nil
}

func (s *Service) DeleteProvider(ctx context.Context, providerID string) error {
	conn, err := s.repo.Provider(ctx, providerID)
	if err != nil {
		return ErrProviderNotFound
	}

	inUse, err := s.repo.ProviderInUse(ctx, providerID)
	if err != nil {
		return fmt.Errorf("check provider in use: %w", err)
	}
	if inUse {
		return ErrProviderInUse
	}

	// 读取旧凭据用于删除失败时的恢复，随后先删凭据、再删元数据。
	oldSecret, oldErr := s.credentials.Get(ctx, conn.CredentialRef)
	if err := s.credentials.Delete(ctx, conn.CredentialRef); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}

	if err := s.repo.DeleteProvider(ctx, providerID); err != nil {
		if rollbackErr := s.restoreCredential(ctx, conn.CredentialRef, oldSecret, oldErr); rollbackErr != nil {
			return fmt.Errorf("%w: %v", ErrCredentialRollback, rollbackErr)
		}
		return fmt.Errorf("delete provider: %w", err)
	}

	return nil
}

// restoreCredential 把凭据恢复为旧值。旧凭据存在时重新写入；旧凭据缺失时删除
// 新写入的凭据，保证凭据库与元数据保持一致。
func (s *Service) restoreCredential(ctx context.Context, ref, oldSecret string, oldErr error) error {
	if oldErr == nil {
		return s.credentials.Save(ctx, ref, oldSecret)
	}
	return s.credentials.Delete(ctx, ref)
}

func (s *Service) PublishCreativeProfile(ctx context.Context, input ProfileInput) (appdomain.CreativeProfileVersion, error) {
	if len(input.Roles) != 3 {
		return appdomain.CreativeProfileVersion{}, ErrInvalidProfileInput
	}

	// 发布前校验每个角色引用的连接存在且凭据已配置，避免发布一个指向
	// 已删除连接或无凭据的方案；作者在发布时就能发现问题，而不是等到运行时。
	providers, err := s.repo.Providers(ctx)
	if err != nil {
		return appdomain.CreativeProfileVersion{}, fmt.Errorf("load providers: %w", err)
	}
	providerByID := make(map[string]appdomain.ProviderConnection, len(providers))
	for _, p := range providers {
		providerByID[p.ID] = p
	}
	for _, r := range input.Roles {
		conn, ok := providerByID[r.ConnectionID]
		if !ok {
			return appdomain.CreativeProfileVersion{}, fmt.Errorf("%w: connection %s", ErrProviderNotFound, r.ConnectionID)
		}
		if _, err := s.credentials.Get(ctx, conn.CredentialRef); err != nil {
			return appdomain.CreativeProfileVersion{}, fmt.Errorf("%w: role %s", ErrCredentialMissing, r.Role)
		}
	}

	// Determine the next version number.
	var nextVersion int64 = 1
	if latest, err := s.repo.LatestProfileVersion(ctx, input.ProfileID); err == nil {
		nextVersion = latest.Version + 1
	}

	roles := make([]appdomain.RoleBinding, len(input.Roles))
	for i, r := range input.Roles {
		roles[i] = appdomain.RoleBinding{
			Role:            r.Role,
			ConnectionID:    r.ConnectionID,
			Model:           r.Model,
			MaxOutputTokens: r.MaxOutputTokens,
			Temperature:     r.Temperature,
			Enabled:         r.Enabled,
		}
	}

	v, err := appdomain.NewCreativeProfileVersion(
		input.ProfileID, nextVersion, input.Name,
		roles, input.MaxRunTokens, input.MaxContextChars, s.clock.Now(),
	)
	if err != nil {
		// 方案领域校验错误（角色缺失、预算非法、审阅不独立等）统一包装为
		// ErrInvalidProfileInput，桌面层映射为 CREATIVE_PROFILE_INVALID。
		return appdomain.CreativeProfileVersion{}, fmt.Errorf("%w: %w", ErrInvalidProfileInput, err)
	}

	if err := s.repo.PublishProfileVersion(ctx, v); err != nil {
		return appdomain.CreativeProfileVersion{}, fmt.Errorf("publish profile version: %w", err)
	}

	return v, nil
}

func (s *Service) ProfileVersion(ctx context.Context, profileID string, version int64) (appdomain.CreativeProfileVersion, error) {
	return s.repo.ProfileVersion(ctx, profileID, version)
}

func (s *Service) LatestProfileVersion(ctx context.Context, profileID string) (appdomain.CreativeProfileVersion, error) {
	return s.repo.LatestProfileVersion(ctx, profileID)
}

// LatestProfiles returns the newest version of every creative profile in a
// single repository query, so the settings page can list all profiles without
// looping at the Wails layer.
func (s *Service) LatestProfiles(ctx context.Context) ([]appdomain.CreativeProfileVersion, error) {
	return s.repo.ListLatestProfiles(ctx)
}

// ResolveCreativeProfile implements ports.CreativeProfileResolver. It reads a
// profile version by exact ID and version, then joins each role binding to its
// non-sensitive connection metadata. It verifies every role's credential exists
// in the OS vault without reading the secret value, so a project can never pin
// a profile that points at a missing or unusable connection.
func (s *Service) ResolveCreativeProfile(ctx context.Context, profileID string, version int64) (agent.ResolvedCreativeProfile, error) {
	profile, err := s.repo.ProfileVersion(ctx, strings.TrimSpace(profileID), version)
	if err != nil {
		return agent.ResolvedCreativeProfile{}, ErrProfileNotFound
	}

	resolved := agent.ResolvedCreativeProfile{
		ProfileID:       profile.ProfileID,
		Version:         profile.Version,
		MaxRunTokens:    profile.MaxRunTokens,
		MaxContextChars: profile.MaxContextChars,
		Roles:           make([]agent.ResolvedRole, 0, len(profile.Roles)),
	}
	// 持久化层按 role ASC 排序会得到 continuity_reviewer → planner → writer，
	// 但 AgentRun 严格按位置校验 planner → writer → continuity_reviewer。
	// 此处按规范顺序重排，保证无论 DB 读取顺序如何，下游都能正确创建运行。
	resolvedByRole := make(map[agent.AgentRole]agent.ResolvedRole, len(profile.Roles))
	for _, role := range profile.Roles {
		conn, err := s.repo.Provider(ctx, role.ConnectionID)
		if err != nil {
			return agent.ResolvedCreativeProfile{}, fmt.Errorf("%w: connection %s", ErrProviderNotFound, role.ConnectionID)
		}
		if _, err := s.credentials.Get(ctx, conn.CredentialRef); err != nil {
			return agent.ResolvedCreativeProfile{}, fmt.Errorf("%w: role %s", ErrCredentialMissing, role.Role)
		}
		resolvedByRole[role.Role] = agent.ResolvedRole{
			Role:            role.Role,
			ConnectionID:    conn.ID,
			ProviderName:    conn.Name,
			ProviderKind:    conn.Kind,
			BaseURL:         conn.BaseURL,
			CredentialRef:   conn.CredentialRef,
			Model:           role.Model,
			MaxOutputTokens: role.MaxOutputTokens,
			Temperature:     role.Temperature,
		}
	}
	for _, required := range []agent.AgentRole{agent.RolePlanner, agent.RoleWriter, agent.RoleContinuityReviewer} {
		r, ok := resolvedByRole[required]
		if !ok {
			return agent.ResolvedCreativeProfile{}, fmt.Errorf("%w: role %s", ErrInvalidProfileInput, required)
		}
		resolved.Roles = append(resolved.Roles, r)
	}
	return resolved, nil
}

func (s *Service) SetDefaultProfile(ctx context.Context, profileID string) error {
	profileID = strings.TrimSpace(profileID)
	// 非空默认方案必须对应一个已发布的最新版本，否则后续创建新项目时
	// 解析默认方案会读不到版本并失败。空值表示清除默认，允许通过。
	if profileID != "" {
		if _, err := s.repo.LatestProfileVersion(ctx, profileID); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidProfileInput, profileID)
		}
	}
	prefs, err := s.repo.Preferences(ctx)
	if err != nil {
		return fmt.Errorf("read preferences: %w", err)
	}
	prefs.DefaultCreativeProfileID = profileID
	prefs.UpdatedAt = s.clock.Now()
	return s.repo.SavePreferences(ctx, prefs)
}

func (s *Service) RecentProjects(ctx context.Context, limit int) ([]appdomain.RecentProject, error) {
	return s.repo.RecentProjects(ctx, limit)
}

func (s *Service) RecordRecentProject(ctx context.Context, rp appdomain.RecentProject) error {
	return s.repo.RecordRecentProject(ctx, rp)
}

func (s *Service) MarkProjectMissing(ctx context.Context, projectID string, missing bool) error {
	return s.repo.MarkProjectMissing(ctx, projectID, missing)
}

// DefaultProfileSelection implements ports.ProjectDefaults so the workspace
// service can pin the current default version into new projects.
func (s *Service) DefaultProfileSelection(ctx context.Context) (ports.ProjectProfileSelection, error) {
	prefs, err := s.repo.Preferences(ctx)
	if err != nil {
		return ports.ProjectProfileSelection{}, err
	}
	if prefs.DefaultCreativeProfileID == "" {
		// 未配置默认方案不是创建项目的错误；作者仍可先手写，再在应用设置中配置模型。
		return ports.ProjectProfileSelection{}, nil
	}
	latest, err := s.repo.LatestProfileVersion(ctx, prefs.DefaultCreativeProfileID)
	if err != nil {
		return ports.ProjectProfileSelection{}, err
	}
	return ports.ProjectProfileSelection{
		ProfileID: latest.ProfileID,
		Version:   latest.Version,
	}, nil
}

// HasCredential checks whether the OS credential vault has a secret for the
// given provider. It does not return the secret value.
func (s *Service) HasCredential(ctx context.Context, provider appdomain.ProviderConnection) bool {
	_, err := s.credentials.Get(ctx, provider.CredentialRef)
	return err == nil
}

// probeTimeout 是真实连接测试的默认超时。
const probeTimeout = 20 * time.Second

// ProbeProvider 对已保存连接发起一次真实、最小费用的模型请求，验证鉴权、
// 模型名称与调用协议可用。它不保存响应正文，也不自动重试到其他连接或模型。
func (s *Service) ProbeProvider(ctx context.Context, providerID, model string) (ports.ProviderProbeResult, error) {
	if s.providers == nil {
		return ports.ProviderProbeResult{}, fmt.Errorf("model provider factory unavailable")
	}
	conn, err := s.repo.Provider(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return ports.ProviderProbeResult{}, ErrProviderNotFound
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return ports.ProviderProbeResult{}, fmt.Errorf("%w: model is required", ErrInvalidProfileInput)
	}

	apiKey, err := s.credentials.Get(ctx, conn.CredentialRef)
	if err != nil {
		return ports.ProviderProbeResult{}, ErrCredentialMissing
	}

	provider, err := s.providers.New(agent.ProviderConnection{Kind: conn.Kind, BaseURL: conn.BaseURL})
	if err != nil {
		return ports.ProviderProbeResult{}, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	started := s.clock.Now()
	result, err := provider.Generate(probeCtx, apiKey, ports.ModelGenerateRequest{
		Model:           model,
		System:          "Return a concise acknowledgement.",
		Input:           "Connection test.",
		MaxOutputTokens: 16,
		Temperature:     0,
	}, nil)
	if err != nil {
		return ports.ProviderProbeResult{}, err
	}

	return ports.ProviderProbeResult{
		ProviderID:        conn.ID,
		ProviderName:      conn.Name,
		Kind:              string(conn.Kind),
		Model:             model,
		ProviderRequestID: result.ProviderRequestID,
		InputTokens:       result.InputTokens,
		OutputTokens:      result.OutputTokens,
		Latency:           s.clock.Now().Sub(started),
	}, nil
}

// now is a convenience for tests that need to assert timestamps.
func (s *Service) now() time.Time {
	return s.clock.Now()
}
