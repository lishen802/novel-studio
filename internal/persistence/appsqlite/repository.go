package appsqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/domain/appsettings"
)

var ErrSettingsMissing = errors.New("settings record does not exist")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func (s *Store) Preferences(ctx context.Context) (appsettings.Preferences, error) {
	var (
		theme            string
		defaultProfileID string
		updatedAt        string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT theme, default_creative_profile_id, updated_at
		FROM app_preferences WHERE id = 1
	`).Scan(&theme, &defaultProfileID, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appsettings.Preferences{}, ErrSettingsMissing
	}
	if err != nil {
		return appsettings.Preferences{}, fmt.Errorf("read preferences: %w", err)
	}
	parsed, err := parseTime(updatedAt)
	if err != nil {
		return appsettings.Preferences{}, fmt.Errorf("parse preferences updated_at: %w", err)
	}
	return appsettings.Preferences{
		Theme:                    appsettings.Theme(theme),
		DefaultCreativeProfileID: defaultProfileID,
		UpdatedAt:                parsed,
	}, nil
}

func (s *Store) SavePreferences(ctx context.Context, prefs appsettings.Preferences) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE app_preferences
		SET theme = ?, default_creative_profile_id = ?, updated_at = ?
		WHERE id = 1
	`, string(prefs.Theme), prefs.DefaultCreativeProfileID, formatTime(prefs.UpdatedAt))
	if err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}
	return nil
}

func (s *Store) UpsertProvider(ctx context.Context, p appsettings.ProviderConnection) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_connections(id, name, kind, base_url, credential_ref, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			kind = excluded.kind,
			base_url = excluded.base_url,
			credential_ref = excluded.credential_ref,
			updated_at = excluded.updated_at
	`, p.ID, p.Name, string(p.Kind), p.BaseURL, p.CredentialRef,
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return fmt.Errorf("upsert provider: %w", err)
	}
	return nil
}

func (s *Store) DeleteProvider(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM provider_connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}

func (s *Store) ProviderInUse(ctx context.Context, providerID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM creative_profile_roles WHERE connection_id = ?
	`, providerID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check provider in use: %w", err)
	}
	return count > 0, nil
}

func (s *Store) AllProfileIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT profile_id FROM creative_profile_versions ORDER BY profile_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list profile ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan profile id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profile ids: %w", err)
	}
	return ids, nil
}

func (s *Store) Provider(ctx context.Context, id string) (appsettings.ProviderConnection, error) {
	var (
		kind, createdAt, updatedAt string
		p                          appsettings.ProviderConnection
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, kind, base_url, credential_ref, created_at, updated_at
		FROM provider_connections WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &kind, &p.BaseURL, &p.CredentialRef, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return appsettings.ProviderConnection{}, ErrSettingsMissing
	}
	if err != nil {
		return appsettings.ProviderConnection{}, fmt.Errorf("read provider: %w", err)
	}
	p.Kind = agent.ProviderKind(kind)
	p.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return appsettings.ProviderConnection{}, fmt.Errorf("parse provider created_at: %w", err)
	}
	p.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return appsettings.ProviderConnection{}, fmt.Errorf("parse provider updated_at: %w", err)
	}
	return p, nil
}

func (s *Store) Providers(ctx context.Context) ([]appsettings.ProviderConnection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, kind, base_url, credential_ref, created_at, updated_at
		FROM provider_connections ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	var providers []appsettings.ProviderConnection
	for rows.Next() {
		var (
			p         appsettings.ProviderConnection
			kind      string
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&p.ID, &p.Name, &kind, &p.BaseURL, &p.CredentialRef, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		p.Kind = agent.ProviderKind(kind)
		if p.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, fmt.Errorf("parse provider created_at: %w", err)
		}
		if p.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, fmt.Errorf("parse provider updated_at: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}
	return providers, nil
}

func (s *Store) PublishProfileVersion(ctx context.Context, v appsettings.CreativeProfileVersion) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin publish profile transaction: %w", err)
	}
	closeWithError := func(err error) error {
		_ = tx.Rollback()
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO creative_profile_versions(profile_id, version, name, max_run_tokens, max_context_chars, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, version) DO UPDATE SET
			name = excluded.name,
			max_run_tokens = excluded.max_run_tokens,
			max_context_chars = excluded.max_context_chars,
			created_at = excluded.created_at
	`, v.ProfileID, v.Version, v.Name, v.MaxRunTokens, v.MaxContextChars, formatTime(v.CreatedAt))
	if err != nil {
		return closeWithError(fmt.Errorf("upsert profile version: %w", err))
	}

	// Clear existing roles for this version so the insert is idempotent.
	_, err = tx.ExecContext(ctx, `
		DELETE FROM creative_profile_roles WHERE profile_id = ? AND version = ?
	`, v.ProfileID, v.Version)
	if err != nil {
		return closeWithError(fmt.Errorf("clear profile roles: %w", err))
	}

	for _, role := range v.Roles {
		enabled := 0
		if role.Enabled {
			enabled = 1
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO creative_profile_roles(profile_id, version, role, connection_id, model, max_output_tokens, temperature, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, v.ProfileID, v.Version, string(role.Role), role.ConnectionID, role.Model,
			role.MaxOutputTokens, role.Temperature, enabled)
		if err != nil {
			return closeWithError(fmt.Errorf("insert profile role: %w", err))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit publish profile: %w", err)
	}
	return nil
}

func (s *Store) ProfileVersion(ctx context.Context, profileID string, version int64) (appsettings.CreativeProfileVersion, error) {
	var (
		v         appsettings.CreativeProfileVersion
		createdAt string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT profile_id, version, name, max_run_tokens, max_context_chars, created_at
		FROM creative_profile_versions WHERE profile_id = ? AND version = ?
	`, profileID, version).Scan(
		&v.ProfileID, &v.Version, &v.Name, &v.MaxRunTokens, &v.MaxContextChars, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return appsettings.CreativeProfileVersion{}, ErrSettingsMissing
	}
	if err != nil {
		return appsettings.CreativeProfileVersion{}, fmt.Errorf("read profile version: %w", err)
	}
	v.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return appsettings.CreativeProfileVersion{}, fmt.Errorf("parse profile created_at: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT role, connection_id, model, max_output_tokens, temperature, enabled
		FROM creative_profile_roles
		WHERE profile_id = ? AND version = ?
		ORDER BY role ASC
	`, profileID, version)
	if err != nil {
		return appsettings.CreativeProfileVersion{}, fmt.Errorf("read profile roles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			roleStr string
			enabled int
			r       appsettings.RoleBinding
		)
		if err := rows.Scan(&roleStr, &r.ConnectionID, &r.Model, &r.MaxOutputTokens, &r.Temperature, &enabled); err != nil {
			return appsettings.CreativeProfileVersion{}, fmt.Errorf("scan profile role: %w", err)
		}
		r.Role = agent.AgentRole(roleStr)
		r.Enabled = enabled == 1
		v.Roles = append(v.Roles, r)
	}
	if err := rows.Err(); err != nil {
		return appsettings.CreativeProfileVersion{}, fmt.Errorf("iterate profile roles: %w", err)
	}
	return v, nil
}

func (s *Store) LatestProfileVersion(ctx context.Context, profileID string) (appsettings.CreativeProfileVersion, error) {
	var version int64
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(version) FROM creative_profile_versions WHERE profile_id = ?
	`, profileID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) || version == 0 {
		return appsettings.CreativeProfileVersion{}, ErrSettingsMissing
	}
	if err != nil {
		return appsettings.CreativeProfileVersion{}, fmt.Errorf("read latest profile version: %w", err)
	}
	return s.ProfileVersion(ctx, profileID, version)
}

func (s *Store) RecordRecentProject(ctx context.Context, rp appsettings.RecentProject) error {
	missing := 0
	if rp.Missing {
		missing = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO recent_projects(project_id, title, path, last_workspace, last_opened_at, missing)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			project_id = excluded.project_id,
			title = excluded.title,
			last_workspace = excluded.last_workspace,
			last_opened_at = excluded.last_opened_at,
			missing = excluded.missing
	`, rp.ProjectID, rp.Title, rp.Path, rp.LastWorkspace, formatTime(rp.LastOpenedAt), missing)
	if err != nil {
		return fmt.Errorf("record recent project: %w", err)
	}
	return nil
}

func (s *Store) RecentProjects(ctx context.Context, limit int) ([]appsettings.RecentProject, error) {
	if limit < 1 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, title, path, last_workspace, last_opened_at, missing
		FROM recent_projects ORDER BY last_opened_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent projects: %w", err)
	}
	defer rows.Close()
	var projects []appsettings.RecentProject
	for rows.Next() {
		var (
			rp           appsettings.RecentProject
			lastOpenedAt string
			missing      int
		)
		if err := rows.Scan(&rp.ProjectID, &rp.Title, &rp.Path, &rp.LastWorkspace, &lastOpenedAt, &missing); err != nil {
			return nil, fmt.Errorf("scan recent project: %w", err)
		}
		rp.LastOpenedAt, err = parseTime(lastOpenedAt)
		if err != nil {
			return nil, fmt.Errorf("parse recent project last_opened_at: %w", err)
		}
		rp.Missing = missing == 1
		projects = append(projects, rp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent projects: %w", err)
	}
	return projects, nil
}

func (s *Store) MarkProjectMissing(ctx context.Context, projectID string, missing bool) error {
	flag := 0
	if missing {
		flag = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE recent_projects SET missing = ? WHERE project_id = ?
	`, flag, projectID)
	if err != nil {
		return fmt.Errorf("mark recent project missing: %w", err)
	}
	return nil
}

func (s *Store) RecordLegacyImport(ctx context.Context, projectID, sourceDigest, profileID string, profileVersion int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO legacy_imports(project_id, source_digest, profile_id, profile_version, imported_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, source_digest) DO UPDATE SET
			profile_id = excluded.profile_id,
			profile_version = excluded.profile_version,
			imported_at = excluded.imported_at
	`, projectID, sourceDigest, profileID, profileVersion, formatTime(now))
	if err != nil {
		return fmt.Errorf("record legacy import: %w", err)
	}
	return nil
}

func (s *Store) LegacyImport(ctx context.Context, projectID, sourceDigest string) (profileID string, profileVersion int64, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT profile_id, profile_version FROM legacy_imports
		WHERE project_id = ? AND source_digest = ?
	`, projectID, sourceDigest).Scan(&profileID, &profileVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrSettingsMissing
	}
	if err != nil {
		return "", 0, fmt.Errorf("read legacy import: %w", err)
	}
	return profileID, profileVersion, nil
}
