package appsettings

import (
	"errors"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/agent"
)

var (
	ErrBlankValue              = errors.New("required value must not be blank")
	ErrInvalidTheme            = errors.New("invalid theme")
	ErrInvalidVersion          = errors.New("invalid creative profile version")
	ErrProfileInUse            = errors.New("provider is referenced by a creative profile")
	ErrInvalidRoleCount        = errors.New("creative profile must have exactly planner, writer, and continuity_reviewer roles")
	ErrInvalidGenerationParams = errors.New("creative profile role has invalid generation parameters")
)

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

func (t Theme) Valid() bool {
	return t == ThemeSystem || t == ThemeLight || t == ThemeDark
}

type Preferences struct {
	Theme                    Theme
	DefaultCreativeProfileID string
	UpdatedAt                time.Time
}

type ProviderConnection struct {
	ID            string
	Name          string
	Kind          agent.ProviderKind
	BaseURL       string
	CredentialRef string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RoleBinding struct {
	Role            agent.AgentRole
	ConnectionID    string
	Model           string
	MaxOutputTokens int
	Temperature     float64
	Enabled         bool
}

type CreativeProfileVersion struct {
	ProfileID       string
	Version         int64
	Name            string
	Roles           []RoleBinding
	MaxRunTokens    int
	MaxContextChars int
	CreatedAt       time.Time
}

type RecentProject struct {
	ProjectID     string
	Title         string
	Path          string
	LastWorkspace string
	LastOpenedAt  time.Time
	Missing       bool
}

func NewPreferences(theme Theme, defaultProfileID string, now time.Time) (Preferences, error) {
	if !theme.Valid() {
		return Preferences{}, ErrInvalidTheme
	}
	now = now.UTC()
	return Preferences{
		Theme:                    theme,
		DefaultCreativeProfileID: strings.TrimSpace(defaultProfileID),
		UpdatedAt:                now,
	}, nil
}

func NewProviderConnection(id, name string, kind agent.ProviderKind, baseURL, credentialRef string, now time.Time) (ProviderConnection, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	credentialRef = strings.TrimSpace(credentialRef)
	if id == "" || name == "" || credentialRef == "" {
		return ProviderConnection{}, ErrBlankValue
	}
	if kind != agent.ProviderOpenAI && kind != agent.ProviderAnthropic {
		return ProviderConnection{}, agent.ErrInvalidProvider
	}
	// 复用 agent 领域的安全规则：远程地址必须 HTTPS，仅本机地址允许 HTTP，
	// 防止 API Key 通过明文 HTTP 连接发送。
	normalizedURL, err := agent.NormalizeEndpoint(baseURL)
	if err != nil {
		return ProviderConnection{}, err
	}
	now = now.UTC()
	return ProviderConnection{
		ID:            id,
		Name:          name,
		Kind:          kind,
		BaseURL:       normalizedURL,
		CredentialRef: credentialRef,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func NewCreativeProfileVersion(
	profileID string,
	version int64,
	name string,
	roles []RoleBinding,
	maxRunTokens, maxContextChars int,
	now time.Time,
) (CreativeProfileVersion, error) {
	profileID = strings.TrimSpace(profileID)
	name = strings.TrimSpace(name)
	if profileID == "" || name == "" {
		return CreativeProfileVersion{}, ErrBlankValue
	}
	if version < 1 {
		return CreativeProfileVersion{}, ErrInvalidVersion
	}
	if maxRunTokens < 1 || maxContextChars < 1 {
		return CreativeProfileVersion{}, agent.ErrInvalidBudget
	}
	if len(roles) != 3 {
		return CreativeProfileVersion{}, ErrInvalidRoleCount
	}
	byRole := make(map[agent.AgentRole]RoleBinding, 3)
	for _, r := range roles {
		if r.Role != agent.RolePlanner && r.Role != agent.RoleWriter && r.Role != agent.RoleContinuityReviewer {
			return CreativeProfileVersion{}, agent.ErrInvalidRole
		}
		if r.ConnectionID == "" || r.Model == "" {
			return CreativeProfileVersion{}, ErrBlankValue
		}
		// 生成参数边界：maxOutputTokens ≥ 1，temperature ∈ [0, 2]。
		// 非法值会让模型调用失败或行为异常，必须在不可变版本创建时拦截。
		if r.MaxOutputTokens < 1 {
			return CreativeProfileVersion{}, ErrInvalidGenerationParams
		}
		if r.Temperature < 0 || r.Temperature > 2 {
			return CreativeProfileVersion{}, ErrInvalidGenerationParams
		}
		byRole[r.Role] = r
	}
	// 三个必需角色必须齐全：重复输入（如 planner, planner, writer）会让 Map 去重后
	// 缺失 reviewer，此时 byRole[reviewer] 是零值，必须拦截避免写入空角色绑定。
	orderedRoles := make([]RoleBinding, 0, 3)
	var sumMaxOutput int
	for _, required := range []agent.AgentRole{agent.RolePlanner, agent.RoleWriter, agent.RoleContinuityReviewer} {
		r, ok := byRole[required]
		if !ok || r.ConnectionID == "" {
			return CreativeProfileVersion{}, ErrInvalidRoleCount
		}
		sumMaxOutput += r.MaxOutputTokens
		orderedRoles = append(orderedRoles, r)
	}
	// 三个角色最大输出之和不能超过运行预算，否则运行中途会因超限失败。
	if sumMaxOutput > maxRunTokens {
		return CreativeProfileVersion{}, agent.ErrInvalidBudget
	}
	writer := byRole[agent.RoleWriter]
	reviewer := byRole[agent.RoleContinuityReviewer]
	if writer.ConnectionID == reviewer.ConnectionID && writer.Model == reviewer.Model {
		return CreativeProfileVersion{}, agent.ErrReviewerNotIndependent
	}
	now = now.UTC()
	return CreativeProfileVersion{
		ProfileID:       profileID,
		Version:         version,
		Name:            name,
		Roles:           orderedRoles,
		MaxRunTokens:    maxRunTokens,
		MaxContextChars: maxContextChars,
		CreatedAt:       now,
	}, nil
}

func NewRecentProject(projectID, title, path string, now time.Time) (RecentProject, error) {
	projectID = strings.TrimSpace(projectID)
	title = strings.TrimSpace(title)
	path = strings.TrimSpace(path)
	if projectID == "" || title == "" || path == "" {
		return RecentProject{}, ErrBlankValue
	}
	return RecentProject{
		ProjectID:    projectID,
		Title:        title,
		Path:         path,
		LastOpenedAt: now.UTC(),
	}, nil
}
