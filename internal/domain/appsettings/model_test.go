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

func TestNewRecentProjectRejectsBlankFields(t *testing.T) {
	if _, err := NewRecentProject("", "title", "/path", time.Now()); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("err = %v, want ErrBlankValue", err)
	}
}
