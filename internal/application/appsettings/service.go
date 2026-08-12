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
	clock       ports.Clock
	ids         ports.IDGenerator
}

func NewService(repo ports.AppSettingsRepository, credentials ports.CredentialStore, clock ports.Clock, ids ports.IDGenerator) *Service {
	return &Service{repo: repo, credentials: credentials, clock: clock, ids: ids}
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

	now := s.clock.Now()
	conn, err := appdomain.NewProviderConnection(existing.ID, input.Name, input.Kind, input.BaseURL, existing.CredentialRef, now)
	if err != nil {
		return appdomain.ProviderConnection{}, err
	}
	conn.CreatedAt = existing.CreatedAt

	if input.APIKey != "" {
		if err := s.credentials.Save(ctx, existing.CredentialRef, input.APIKey); err != nil {
			return appdomain.ProviderConnection{}, fmt.Errorf("update credential: %w", err)
		}
	}

	if err := s.repo.UpsertProvider(ctx, conn); err != nil {
		return appdomain.ProviderConnection{}, fmt.Errorf("update provider metadata: %w", err)
	}

	return conn, nil
}

func (s *Service) DeleteProvider(ctx context.Context, providerID string) error {
	_, err := s.repo.Provider(ctx, providerID)
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

	if err := s.repo.DeleteProvider(ctx, providerID); err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}

	return nil
}

func (s *Service) PublishCreativeProfile(ctx context.Context, input ProfileInput) (appdomain.CreativeProfileVersion, error) {
	if len(input.Roles) != 3 {
		return appdomain.CreativeProfileVersion{}, ErrInvalidProfileInput
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
		return appdomain.CreativeProfileVersion{}, err
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

func (s *Service) SetDefaultProfile(ctx context.Context, profileID string) error {
	prefs, err := s.repo.Preferences(ctx)
	if err != nil {
		return fmt.Errorf("read preferences: %w", err)
	}
	prefs.DefaultCreativeProfileID = strings.TrimSpace(profileID)
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
		return ports.ProjectProfileSelection{}, ErrProfileNotFound
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

// now is a convenience for tests that need to assert timestamps.
func (s *Service) now() time.Time {
	return s.clock.Now()
}
