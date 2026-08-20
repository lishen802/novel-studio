package appsettings

import (
	"context"
	"fmt"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/agent"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
)

// Compile-time check that Service implements ports.LegacySettingsImporter.
var _ ports.LegacySettingsImporter = (*Service)(nil)

// legacyProfileIDPrefix is the deterministic prefix for profile IDs created by
// legacy migration. The profile ID is derived from the project ID so that a
// re-import with a changed source digest publishes a new version of the same
// profile instead of creating an unrelated one. A dash (not a colon) is used
// as the separator so the ID remains parseable by test fakes that store it
// alongside the version number in a colon-delimited string.
const legacyProfileIDPrefix = "legacy-"

// ImportLegacyProject imports a project's legacy agent settings into the
// application settings database. The import is idempotent on
// (project_id, source_digest): calling it twice with the same digest returns
// the existing result without creating duplicate providers or profile versions.
//
// When the source digest changes (the project's agent settings were modified
// after the first import), a new version of the same profile is published so
// the user can review the evolution of their migrated settings.
//
// Only non-sensitive metadata and existing credential references are stored.
// API key values never enter the settings database; they remain in the OS
// credential vault under the reference carried over from the project.
func (s *Service) ImportLegacyProject(
	ctx context.Context,
	snapshot ports.LegacyAgentSettingsSnapshot,
) (ports.LegacyImportResult, error) {
	if strings.TrimSpace(snapshot.ProjectID) == "" {
		return ports.LegacyImportResult{}, ErrInvalidProfileInput
	}

	// Idempotency: if this (project_id, source_digest) pair was already
	// imported, return the recorded result without writing anything.
	if profileID, version, err := s.repo.LegacyImport(ctx, snapshot.ProjectID, snapshot.SourceDigest); err == nil {
		return ports.LegacyImportResult{
			ProfileID:      profileID,
			ProfileVersion: version,
			ProviderIDs:    s.providerIDsForProfile(ctx, profileID, version),
		}, nil
	}

	profileID := legacyProfileIDPrefix + snapshot.ProjectID

	existingProviders, err := s.repo.Providers(ctx)
	if err != nil {
		return ports.LegacyImportResult{}, fmt.Errorf("list existing providers for legacy import: %w", err)
	}

	// Deduplicate providers by (kind, base_url, credential_ref). Providers
	// created by a previous import (with a different digest) are reused so the
	// settings database does not accumulate duplicates across re-imports.
	providerIDByLegacyID := make(map[string]string, len(snapshot.Connections))
	createdProviderIDs := make([]string, 0, len(snapshot.Connections))
	for _, conn := range snapshot.Connections {
		matchedID := matchLegacyProvider(existingProviders, conn)
		if matchedID != "" {
			providerIDByLegacyID[conn.ID] = matchedID
			continue
		}
		newID := s.ids.NewID()
		credentialRef := strings.TrimSpace(conn.CredentialRef)
		if credentialRef == "" {
			credentialRef = "legacy-provider:" + newID
		}
		now := s.clock.Now()
		newProvider, err := appdomain.NewProviderConnection(
			newID, conn.Name, agent.ProviderKind(conn.Kind), conn.BaseURL, credentialRef, now,
		)
		if err != nil {
			return ports.LegacyImportResult{}, fmt.Errorf("create legacy provider connection: %w", err)
		}
		if err := s.repo.UpsertProvider(ctx, newProvider); err != nil {
			return ports.LegacyImportResult{}, fmt.Errorf("upsert legacy provider: %w", err)
		}
		existingProviders = append(existingProviders, newProvider)
		providerIDByLegacyID[conn.ID] = newID
		createdProviderIDs = append(createdProviderIDs, newID)
	}

	roles := make([]appdomain.RoleBinding, 0, len(snapshot.Profiles))
	for _, p := range snapshot.Profiles {
		providerID, ok := providerIDByLegacyID[p.ConnectionID]
		if !ok {
			continue
		}
		roles = append(roles, appdomain.RoleBinding{
			Role:            agent.Role(p.Role),
			ConnectionID:    providerID,
			Model:           p.Model,
			MaxOutputTokens: p.MaxOutputTokens,
			Temperature:     p.Temperature,
			Enabled:         p.Enabled,
		})
	}

	nextVersion := int64(1)
	if latest, err := s.repo.LatestProfileVersion(ctx, profileID); err == nil {
		nextVersion = latest.Version + 1
	}

	name := legacyProfileName(snapshot.ProjectTitle, nextVersion)
	version, err := appdomain.NewCreativeProfileVersion(
		profileID, nextVersion, name,
		roles, snapshot.MaxRunTokens, snapshot.MaxContextChars, s.clock.Now(),
	)
	if err != nil {
		return ports.LegacyImportResult{}, fmt.Errorf("create legacy profile version: %w", err)
	}
	if err := s.repo.PublishProfileVersion(ctx, version); err != nil {
		return ports.LegacyImportResult{}, fmt.Errorf("publish legacy profile version: %w", err)
	}

	if err := s.repo.RecordLegacyImport(
		ctx, snapshot.ProjectID, snapshot.SourceDigest, profileID, nextVersion, s.clock.Now(),
	); err != nil {
		return ports.LegacyImportResult{}, fmt.Errorf("record legacy import: %w", err)
	}

	return ports.LegacyImportResult{
		ProfileID:      profileID,
		ProfileVersion: nextVersion,
		ProviderIDs:    createdProviderIDs,
	}, nil
}

// providerIDsForProfile returns the distinct provider IDs referenced by the
// roles of a profile version. Used to populate LegacyImportResult when an
// idempotent return skips provider creation.
func (s *Service) providerIDsForProfile(ctx context.Context, profileID string, version int64) []string {
	v, err := s.repo.ProfileVersion(ctx, profileID, version)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool, len(v.Roles))
	ids := make([]string, 0, len(v.Roles))
	for _, r := range v.Roles {
		if !seen[r.ConnectionID] {
			seen[r.ConnectionID] = true
			ids = append(ids, r.ConnectionID)
		}
	}
	return ids
}

// matchLegacyProvider returns the ID of an existing provider connection that
// matches the legacy connection by (kind, base_url, credential_ref), or "" if
// no match is found. This deduplicates providers across imports and within a
// single import.
func matchLegacyProvider(existing []appdomain.ProviderConnection, conn ports.LegacyProvider) string {
	for _, p := range existing {
		if string(p.Kind) != conn.Kind {
			continue
		}
		if p.BaseURL != conn.BaseURL {
			continue
		}
		credentialRef := strings.TrimSpace(conn.CredentialRef)
		if credentialRef == "" {
			continue
		}
		if p.CredentialRef == credentialRef {
			return p.ID
		}
	}
	return ""
}

// legacyProfileName builds the human-readable name for a migrated profile
// version. When the project title is known the name follows the pattern
// "从《作品名》迁移"; otherwise a generic fallback is used.
func legacyProfileName(projectTitle string, version int64) string {
	title := strings.TrimSpace(projectTitle)
	if title != "" {
		return fmt.Sprintf("从《%s》迁移", title)
	}
	return fmt.Sprintf("从旧项目迁移（版本 %d）", version)
}
