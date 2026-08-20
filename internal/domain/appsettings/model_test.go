package appsettings

import (
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestNewPreferencesRejectsInvalidTheme(t *testing.T) {
	if _, err := NewPreferences(Theme("neon"), "", time.Now()); !errors.Is(err, ErrInvalidTheme) {
		t.Fatalf("err = %v, want ErrInvalidTheme", err)
	}
}

func TestNewPreferencesAcceptsSystemTheme(t *testing.T) {
	prefs, err := NewPreferences(ThemeSystem, "", time.Now())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if prefs.Theme != ThemeSystem {
		t.Fatalf("theme = %q, want system", prefs.Theme)
	}
}

func TestNewProviderConnectionRejectsBlankFields(t *testing.T) {
	cases := []struct {
		name          string
		id            string
		displayName   string
		credentialRef string
	}{
		{"blank id", "", "OpenAI", "ref-1"},
		{"blank name", "p1", "", "ref-1"},
		{"blank credential", "p1", "OpenAI", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewProviderConnection(tc.id, tc.displayName, agent.ProviderOpenAI, "https://api.openai.com/v1", tc.credentialRef, time.Now()); !errors.Is(err, ErrBlankValue) {
				t.Fatalf("err = %v, want ErrBlankValue", err)
			}
		})
	}
}

func TestNewProviderConnectionRejectsInvalidKind(t *testing.T) {
	if _, err := NewProviderConnection("p1", "OpenAI", agent.ProviderKind("grok"), "https://api.openai.com/v1", "ref-1", time.Now()); !errors.Is(err, agent.ErrInvalidProvider) {
		t.Fatalf("err = %v, want ErrInvalidProvider", err)
	}
}

func TestNewCreativeProfileVersionRejectsDuplicateWriterReviewer(t *testing.T) {
	roles := []RoleBinding{
		{Role: agent.RolePlanner, ConnectionID: "c1", Model: "gpt-4", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		{Role: agent.RoleWriter, ConnectionID: "c1", Model: "same-model", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
		{Role: agent.RoleContinuityReviewer, ConnectionID: "c1", Model: "same-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
	}
	if _, err := NewCreativeProfileVersion("profile-1", 1, "方案", roles, 30000, 120000, time.Now()); !errors.Is(err, agent.ErrReviewerNotIndependent) {
		t.Fatalf("err = %v, want ErrReviewerNotIndependent", err)
	}
}

func TestNewCreativeProfileVersionRejectsMissingRole(t *testing.T) {
	roles := []RoleBinding{
		{Role: agent.RolePlanner, ConnectionID: "c1", Model: "gpt-4", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		{Role: agent.RoleWriter, ConnectionID: "c1", Model: "writer-model", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
	}
	if _, err := NewCreativeProfileVersion("profile-1", 1, "方案", roles, 30000, 120000, time.Now()); !errors.Is(err, ErrInvalidRoleCount) {
		t.Fatalf("err = %v, want ErrInvalidRoleCount", err)
	}
}

// TestNewCreativeProfileVersionRejectsDuplicateRoles 覆盖审查指出的重复角色
// 场景：输入 [planner, planner, writer] 共 3 项，Map 去重后缺失 reviewer，
// 必须被拦截，不能写入空 ConnectionID/Model 的 reviewer 角色绑定。
func TestNewCreativeProfileVersionRejectsDuplicateRoles(t *testing.T) {
	roles := []RoleBinding{
		{Role: agent.RolePlanner, ConnectionID: "c1", Model: "gpt-4", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		{Role: agent.RolePlanner, ConnectionID: "c2", Model: "gpt-4", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		{Role: agent.RoleWriter, ConnectionID: "c1", Model: "writer-model", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
	}
	if _, err := NewCreativeProfileVersion("profile-1", 1, "方案", roles, 30000, 120000, time.Now()); !errors.Is(err, ErrInvalidRoleCount) {
		t.Fatalf("err = %v, want ErrInvalidRoleCount", err)
	}
}

// TestNewCreativeProfileVersionRejectsInvalidGenerationParams 验证 maxOutputTokens
// 和 temperature 的边界校验，避免非法生成参数被写入不可变版本。
func TestNewCreativeProfileVersionRejectsInvalidGenerationParams(t *testing.T) {
	cases := []struct {
		name        string
		output      int
		temperature float64
	}{
		{"zero output tokens", 0, 0.7},
		{"negative output tokens", -1, 0.7},
		{"temperature below zero", 1000, -0.1},
		{"temperature above two", 1000, 2.1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roles := []RoleBinding{
				{Role: agent.RolePlanner, ConnectionID: "c1", Model: "gpt-4", MaxOutputTokens: tc.output, Temperature: tc.temperature, Enabled: true},
				{Role: agent.RoleWriter, ConnectionID: "c1", Model: "writer-model", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
				{Role: agent.RoleContinuityReviewer, ConnectionID: "c2", Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
			}
			if _, err := NewCreativeProfileVersion("profile-1", 1, "方案", roles, 30000, 120000, time.Now()); !errors.Is(err, ErrInvalidGenerationParams) {
				t.Fatalf("err = %v, want ErrInvalidGenerationParams", err)
			}
		})
	}
}

// TestNewCreativeProfileVersionRejectsBudgetExceeded 验证三个角色最大输出之和
// 不超过运行预算，避免运行中途因超限失败。
func TestNewCreativeProfileVersionRejectsBudgetExceeded(t *testing.T) {
	roles := []RoleBinding{
		{Role: agent.RolePlanner, ConnectionID: "c1", Model: "gpt-4", MaxOutputTokens: 15000, Temperature: 0.7, Enabled: true},
		{Role: agent.RoleWriter, ConnectionID: "c1", Model: "writer-model", MaxOutputTokens: 10000, Temperature: 0.8, Enabled: true},
		{Role: agent.RoleContinuityReviewer, ConnectionID: "c2", Model: "reviewer-model", MaxOutputTokens: 6000, Temperature: 0.2, Enabled: true},
	}
	// 15000 + 10000 + 6000 = 31000 > 30000
	if _, err := NewCreativeProfileVersion("profile-1", 1, "方案", roles, 30000, 120000, time.Now()); !errors.Is(err, agent.ErrInvalidBudget) {
		t.Fatalf("err = %v, want ErrInvalidBudget", err)
	}
}

func TestNewRecentProjectRejectsBlankFields(t *testing.T) {
	if _, err := NewRecentProject("", "title", "/path", time.Now()); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("err = %v, want ErrBlankValue", err)
	}
}
