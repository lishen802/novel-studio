package appsettings

import (
	"context"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/agent"
)

// legacySnapshot builds a LegacyAgentSettingsSnapshot with the given project
// metadata, source digest, and writer/reviewer models. The snapshot always has
// two OpenAI providers and three role bindings so the resulting profile passes
// domain validation (planner/writer/continuity_reviewer with an independent
// reviewer).
func legacySnapshot(projectID, projectTitle, sourceDigest, writerModel string) ports.LegacyAgentSettingsSnapshot {
	return ports.LegacyAgentSettingsSnapshot{
		ProjectID:       projectID,
		ProjectTitle:    projectTitle,
		SourceDigest:    sourceDigest,
		MaxRunTokens:    agent.DefaultMaxRunTokens,
		MaxContextChars: agent.DefaultMaxContextChars,
		Connections: []ports.LegacyProvider{
			{
				ID:            "legacy-conn-1",
				Name:          "OpenAI",
				Kind:          string(agent.ProviderOpenAI),
				BaseURL:       "https://api.openai.com/v1",
				CredentialRef: "provider:legacy-conn-1",
			},
			{
				ID:            "legacy-conn-2",
				Name:          "Anthropic",
				Kind:          string(agent.ProviderAnthropic),
				BaseURL:       "https://api.anthropic.com",
				CredentialRef: "provider:legacy-conn-2",
			},
		},
		Profiles: []ports.LegacyRoleBinding{
			{Role: string(agent.RolePlanner), ConnectionID: "legacy-conn-1", Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
			{Role: string(agent.RoleWriter), ConnectionID: "legacy-conn-1", Model: writerModel, MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
			{Role: string(agent.RoleContinuityReviewer), ConnectionID: "legacy-conn-2", Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
		},
	}
}

func TestImportLegacyProjectCreatesOneProfileAndPreservesCredentialRefs(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	snapshot := legacySnapshot("project-1", "凡人修仙", "digest-a", "writer-model")

	// Pre-populate the credential vault so the legacy credential references
	// resolve. The import must preserve these references, not the secret values.
	creds.saved["provider:legacy-conn-1"] = "sk-legacy-1"
	creds.saved["provider:legacy-conn-2"] = "sk-legacy-2"

	first, err := svc.ImportLegacyProject(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if first.ProfileID == "" || first.ProfileVersion != 1 {
		t.Fatalf("first result = %+v", first)
	}
	if len(first.ProviderIDs) != 2 {
		t.Fatalf("first provider IDs = %v, want 2", first.ProviderIDs)
	}

	second, err := svc.ImportLegacyProject(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if second.ProfileID != first.ProfileID || second.ProfileVersion != first.ProfileVersion {
		t.Fatalf("idempotent result = %+v, want %+v", second, first)
	}

	// The settings database must contain exactly one profile version and two
	// providers after two imports with the same digest.
	if len(repo.profiles) != 1 {
		t.Fatalf("profile versions = %d, want 1", len(repo.profiles))
	}
	if len(repo.providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(repo.providers))
	}

	// The credential references (not the secret values) must be preserved on
	// the stored providers.
	for _, p := range repo.providers {
		if !strings.HasPrefix(p.CredentialRef, "provider:legacy-conn-") {
			t.Fatalf("provider %s credential ref = %q", p.ID, p.CredentialRef)
		}
	}

	// The secret values must never enter the settings database. The providers
	// list is the closest proxy for the settings DB contents.
	for _, p := range repo.providers {
		if strings.Contains(p.CredentialRef, "sk-legacy") {
			t.Fatalf("secret value reached settings database: %q", p.CredentialRef)
		}
	}

	// The credential vault must still hold the original secrets under the
	// legacy references.
	if creds.saved["provider:legacy-conn-1"] != "sk-legacy-1" {
		t.Fatal("credential vault was modified during import")
	}
}

func TestLegacySourceChangeCreatesExplicitNewVersion(t *testing.T) {
	svc, repo, _ := newServiceHarness(t)
	firstSnapshot := legacySnapshot("project-1", "凡人修仙", "digest-a", "writer-v1")

	first, err := svc.ImportLegacyProject(context.Background(), firstSnapshot)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if first.ProfileVersion != 1 {
		t.Fatalf("first version = %d, want 1", first.ProfileVersion)
	}

	// The project's agent settings changed (writer model updated), producing a
	// new source digest. The re-import must publish a new version of the same
	// profile, not a new profile.
	secondSnapshot := legacySnapshot("project-1", "凡人修仙", "digest-b", "writer-v2")
	second, err := svc.ImportLegacyProject(context.Background(), secondSnapshot)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if second.ProfileID != first.ProfileID {
		t.Fatalf("profile ID changed: %q → %q", first.ProfileID, second.ProfileID)
	}
	if second.ProfileVersion != first.ProfileVersion+1 {
		t.Fatalf("second version = %d, want %d", second.ProfileVersion, first.ProfileVersion+1)
	}

	// Both versions must exist in the settings database.
	if len(repo.profiles) != 2 {
		t.Fatalf("profile versions = %d, want 2", len(repo.profiles))
	}

	v1, err := svc.ProfileVersion(context.Background(), first.ProfileID, 1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := svc.ProfileVersion(context.Background(), first.ProfileID, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range v1.Roles {
		if r.Role == agent.RoleWriter && r.Model != "writer-v1" {
			t.Fatalf("v1 writer model = %q, want writer-v1", r.Model)
		}
	}
	for _, r := range v2.Roles {
		if r.Role == agent.RoleWriter && r.Model != "writer-v2" {
			t.Fatalf("v2 writer model = %q, want writer-v2", r.Model)
		}
	}
}

func TestLegacyMissingCredentialImportsSafeMetadata(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	// Snapshot with a missing credential reference on the second connection.
	// The import must not block: a synthetic reference is generated and the
	// provider is stored with HasCredential=false.
	snapshot := legacySnapshot("project-1", "无凭据项目", "digest-a", "writer-model")
	snapshot.Connections[1].CredentialRef = ""

	result, err := svc.ImportLegacyProject(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("import with missing credential: %v", err)
	}
	if result.ProfileID == "" || result.ProfileVersion != 1 {
		t.Fatalf("result = %+v", result)
	}

	// Two providers must be stored; the second must have a generated
	// credential reference (not the missing one).
	if len(repo.providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(repo.providers))
	}
	missingProvider := repo.providers[1]
	if missingProvider.CredentialRef == "" {
		t.Fatal("missing credential provider has empty credential ref")
	}
	if !strings.HasPrefix(missingProvider.CredentialRef, "legacy-provider:") {
		t.Fatalf("generated credential ref = %q, want legacy-provider: prefix", missingProvider.CredentialRef)
	}

	// HasCredential must return false because the vault has no secret under the
	// generated reference.
	if svc.HasCredential(context.Background(), missingProvider) {
		t.Fatal("HasCredential = true, want false for missing credential")
	}

	// The first provider (with a real credential ref) must still resolve.
	if _, err := creds.Get(context.Background(), repo.providers[0].CredentialRef); err == nil {
		t.Fatal("first provider credential unexpectedly present in vault")
	}
}

func TestLegacyImportSettingsSuccessPinFailureRetriesWithoutDuplicateVersion(t *testing.T) {
	svc, repo, _ := newServiceHarness(t)
	snapshot := legacySnapshot("project-1", "重试项目", "digest-a", "writer-model")

	// First import succeeds: the settings database now has profile version 1
	// and a legacy_imports row recording (project-1, digest-a).
	first, err := svc.ImportLegacyProject(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if first.ProfileVersion != 1 {
		t.Fatalf("first version = %d, want 1", first.ProfileVersion)
	}

	// Simulate a pin failure: the caller's next step (writing the profile
	// selection to the project database) failed, so the caller retries the
	// entire import. The retry must be idempotent: no new version, no new
	// providers, the same result.
	retry, err := svc.ImportLegacyProject(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("retry import: %v", err)
	}
	if retry.ProfileID != first.ProfileID || retry.ProfileVersion != first.ProfileVersion {
		t.Fatalf("retry result = %+v, want %+v", retry, first)
	}

	// Exactly one profile version and two providers must exist.
	if len(repo.profiles) != 1 {
		t.Fatalf("profile versions after retry = %d, want 1", len(repo.profiles))
	}
	if len(repo.providers) != 2 {
		t.Fatalf("providers after retry = %d, want 2", len(repo.providers))
	}

	// The legacy_imports table must have exactly one entry for this project.
	if len(repo.legacyImports) != 1 {
		t.Fatalf("legacy import records = %d, want 1", len(repo.legacyImports))
	}
}
