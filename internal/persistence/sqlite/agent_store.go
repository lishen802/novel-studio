package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var ErrAgentRecordMissing = errors.New("agent record does not exist")

func (s *Store) UpsertProviderConnection(ctx context.Context, connection agentdomain.ProviderConnection) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_connections(
			id, project_id, name, kind, base_url, credential_ref, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			kind = excluded.kind,
			base_url = excluded.base_url,
			credential_ref = excluded.credential_ref,
			updated_at = excluded.updated_at
	`,
		connection.ID, connection.ProjectID, connection.Name, connection.Kind,
		connection.BaseURL, connection.CredentialRef,
		formatTime(connection.CreatedAt), formatTime(connection.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save provider connection", err)
	}
	return nil
}

func (s *Store) ListProviderConnections(ctx context.Context) ([]agentdomain.ProviderConnection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, kind, base_url, credential_ref, created_at, updated_at
		FROM provider_connections ORDER BY name, id
	`)
	if err != nil {
		return nil, databaseOperationError("list provider connections", err)
	}
	defer rows.Close()
	result := make([]agentdomain.ProviderConnection, 0)
	for rows.Next() {
		connection, err := scanProviderConnection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, connection)
	}
	return result, rows.Err()
}

func (s *Store) ProviderConnection(ctx context.Context, id string) (agentdomain.ProviderConnection, error) {
	connection, err := scanProviderConnection(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, kind, base_url, credential_ref, created_at, updated_at
		FROM provider_connections WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return agentdomain.ProviderConnection{}, fmt.Errorf("%w: provider connection %s", ErrAgentRecordMissing, id)
	}
	return connection, err
}

func (s *Store) DeleteProviderConnection(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM provider_connections WHERE id = ?", id)
	if err != nil {
		return databaseOperationError("delete provider connection", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%w: provider connection %s", ErrAgentRecordMissing, id)
	}
	return nil
}

func (s *Store) UpsertAgentProfile(ctx context.Context, profile agentdomain.AgentProfile) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_profiles(
			id, project_id, role, connection_id, model, max_output_tokens,
			temperature, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, role) DO UPDATE SET
			connection_id = excluded.connection_id,
			model = excluded.model,
			max_output_tokens = excluded.max_output_tokens,
			temperature = excluded.temperature,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`,
		profile.ID, profile.ProjectID, profile.Role, profile.ConnectionID, profile.Model,
		profile.MaxOutputTokens, profile.Temperature, profile.Enabled,
		formatTime(profile.CreatedAt), formatTime(profile.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("save agent profile", err)
	}
	return nil
}

func (s *Store) ListAgentProfiles(ctx context.Context) ([]agentdomain.AgentProfile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, role, connection_id, model, max_output_tokens,
			temperature, enabled, created_at, updated_at
		FROM agent_profiles
		ORDER BY CASE role
			WHEN 'planner' THEN 1 WHEN 'writer' THEN 2 ELSE 3 END
	`)
	if err != nil {
		return nil, databaseOperationError("list agent profiles", err)
	}
	defer rows.Close()
	result := make([]agentdomain.AgentProfile, 0)
	for rows.Next() {
		profile, err := scanAgentProfile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, profile)
	}
	return result, rows.Err()
}

func (s *Store) AgentProfile(ctx context.Context, id string) (agentdomain.AgentProfile, error) {
	profile, err := scanAgentProfile(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, role, connection_id, model, max_output_tokens,
			temperature, enabled, created_at, updated_at
		FROM agent_profiles WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return agentdomain.AgentProfile{}, fmt.Errorf("%w: agent profile %s", ErrAgentRecordMissing, id)
	}
	return profile, err
}

func (s *Store) CreateAgentRun(ctx context.Context, run agentdomain.AgentRun) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin create agent run", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO agent_runs(
			id, project_id, chapter_id, chapter_base_revision, prompt, status,
			error_code, error_message, change_set_id, retry_of, created_at, updated_at,
			started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)
	`,
		run.ID, run.ProjectID, run.ChapterID, run.ChapterBaseRevision, run.Prompt, run.Status,
		run.ErrorCode, run.ErrorMessage, run.ChangeSetID, run.RetryOf,
		formatTime(run.CreatedAt), formatTime(run.UpdatedAt), nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
	)
	if err != nil {
		return databaseOperationError("create agent run", err)
	}
	for _, step := range run.Steps {
		if err := insertAgentRunStep(ctx, tx, step); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit create agent run", err)
	}
	return nil
}

func (s *Store) UpdateAgentRun(ctx context.Context, run agentdomain.AgentRun) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs SET
			status = ?, error_code = ?, error_message = ?,
			change_set_id = NULLIF(?, ''), retry_of = NULLIF(?, ''),
			updated_at = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`,
		run.Status, run.ErrorCode, run.ErrorMessage, run.ChangeSetID, run.RetryOf,
		formatTime(run.UpdatedAt), nullableTime(run.StartedAt), nullableTime(run.CompletedAt), run.ID,
	)
	if err != nil {
		return databaseOperationError("update agent run", err)
	}
	return requireAffected(result, "agent run", run.ID)
}

func (s *Store) AgentRun(ctx context.Context, id string) (agentdomain.AgentRun, error) {
	run, err := scanAgentRun(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, chapter_id, chapter_base_revision, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			created_at, updated_at, started_at, completed_at
		FROM agent_runs WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return agentdomain.AgentRun{}, fmt.Errorf("%w: agent run %s", ErrAgentRecordMissing, id)
	}
	if err != nil {
		return agentdomain.AgentRun{}, err
	}
	run.Steps, err = s.listAgentRunSteps(ctx, id)
	return run, err
}

func (s *Store) ListAgentRuns(ctx context.Context, limit int) ([]agentdomain.AgentRun, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, chapter_id, chapter_base_revision, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			created_at, updated_at, started_at, completed_at
		FROM agent_runs ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, databaseOperationError("list agent runs", err)
	}
	defer rows.Close()
	result := make([]agentdomain.AgentRun, 0)
	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Steps, err = s.listAgentRunSteps(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) UpdateAgentRunStep(ctx context.Context, step agentdomain.AgentRunStep) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_run_steps SET
			status = ?, output = ?, error_code = ?, error_message = ?,
			started_at = ?, completed_at = ?, updated_at = ?
		WHERE id = ?
	`,
		step.Status, step.Output, step.ErrorCode, step.ErrorMessage,
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt), formatTime(step.UpdatedAt), step.ID,
	)
	if err != nil {
		return databaseOperationError("update agent run step", err)
	}
	return requireAffected(result, "agent run step", step.ID)
}

func (s *Store) CreateAgentArtifact(ctx context.Context, artifact agentdomain.AgentArtifact) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_artifacts(id, run_id, step_id, kind, content, sha256, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`, artifact.ID, artifact.RunID, artifact.StepID, artifact.Kind, artifact.Content, artifact.SHA256, formatTime(artifact.CreatedAt))
	if err != nil {
		return databaseOperationError("create agent artifact", err)
	}
	return nil
}

func (s *Store) ListAgentArtifacts(ctx context.Context, runID string) ([]agentdomain.AgentArtifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, COALESCE(step_id, ''), kind, content, sha256, created_at
		FROM agent_artifacts WHERE run_id = ? ORDER BY created_at, id
	`, runID)
	if err != nil {
		return nil, databaseOperationError("list agent artifacts", err)
	}
	defer rows.Close()
	result := make([]agentdomain.AgentArtifact, 0)
	for rows.Next() {
		var artifact agentdomain.AgentArtifact
		var createdAt string
		if err := rows.Scan(
			&artifact.ID, &artifact.RunID, &artifact.StepID, &artifact.Kind,
			&artifact.Content, &artifact.SHA256, &createdAt,
		); err != nil {
			return nil, err
		}
		artifact.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	return result, rows.Err()
}

func (s *Store) RecoverInterruptedAgentRuns(ctx context.Context, recoveredAt time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	formatted := formatTime(recoveredAt)
	result, err := tx.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = 'recoverable',
			error_code = 'app_interrupted',
			error_message = '应用上次退出时创作任务仍在运行，可重试该任务。',
			updated_at = ?
		WHERE status = 'running'
	`, formatted)
	if err != nil {
		return 0, databaseOperationError("recover interrupted agent runs", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_run_steps
		SET status = 'failed',
			error_code = 'app_interrupted',
			error_message = '应用退出导致步骤中断。',
			completed_at = ?,
			updated_at = ?
		WHERE status = 'running'
	`, formatted, formatted); err != nil {
		return 0, databaseOperationError("recover interrupted agent run steps", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProviderConnection(scanner rowScanner) (agentdomain.ProviderConnection, error) {
	var connection agentdomain.ProviderConnection
	var createdAt, updatedAt string
	err := scanner.Scan(
		&connection.ID, &connection.ProjectID, &connection.Name, &connection.Kind,
		&connection.BaseURL, &connection.CredentialRef, &createdAt, &updatedAt,
	)
	if err != nil {
		return connection, err
	}
	connection.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		connection.UpdatedAt, err = parseTime(updatedAt)
	}
	return connection, err
}

func scanAgentProfile(scanner rowScanner) (agentdomain.AgentProfile, error) {
	var profile agentdomain.AgentProfile
	var createdAt, updatedAt string
	err := scanner.Scan(
		&profile.ID, &profile.ProjectID, &profile.Role, &profile.ConnectionID, &profile.Model,
		&profile.MaxOutputTokens, &profile.Temperature, &profile.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		return profile, err
	}
	profile.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		profile.UpdatedAt, err = parseTime(updatedAt)
	}
	return profile, err
}

func scanAgentRun(scanner rowScanner) (agentdomain.AgentRun, error) {
	var run agentdomain.AgentRun
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	err := scanner.Scan(
		&run.ID, &run.ProjectID, &run.ChapterID, &run.ChapterBaseRevision, &run.Prompt,
		&run.Status, &run.ErrorCode, &run.ErrorMessage, &run.ChangeSetID, &run.RetryOf,
		&createdAt, &updatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return run, err
	}
	run.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		run.UpdatedAt, err = parseTime(updatedAt)
	}
	if err == nil {
		run.StartedAt, err = parseNullableTime(startedAt)
	}
	if err == nil {
		run.CompletedAt, err = parseNullableTime(completedAt)
	}
	return run, err
}

func (s *Store) listAgentRunSteps(ctx context.Context, runID string) ([]agentdomain.AgentRunStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, role, profile_id, position, status, output,
			error_code, error_message, started_at, completed_at, created_at, updated_at
		FROM agent_run_steps WHERE run_id = ? ORDER BY position
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]agentdomain.AgentRunStep, 0)
	for rows.Next() {
		var step agentdomain.AgentRunStep
		var startedAt, completedAt sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(
			&step.ID, &step.RunID, &step.Role, &step.ProfileID, &step.Position,
			&step.Status, &step.Output, &step.ErrorCode, &step.ErrorMessage,
			&startedAt, &completedAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		step.CreatedAt, err = parseTime(createdAt)
		if err == nil {
			step.UpdatedAt, err = parseTime(updatedAt)
		}
		if err == nil {
			step.StartedAt, err = parseNullableTime(startedAt)
		}
		if err == nil {
			step.CompletedAt, err = parseNullableTime(completedAt)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, step)
	}
	return result, rows.Err()
}

func insertAgentRunStep(ctx context.Context, tx *sql.Tx, step agentdomain.AgentRunStep) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO agent_run_steps(
			id, run_id, role, profile_id, position, status, output,
			error_code, error_message, started_at, completed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		step.ID, step.RunID, step.Role, step.ProfileID, step.Position, step.Status,
		step.Output, step.ErrorCode, step.ErrorMessage,
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt),
		formatTime(step.CreatedAt), formatTime(step.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("create agent run step", err)
	}
	return nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func parseNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func requireAffected(result sql.Result, kind, id string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%w: %s %s", ErrAgentRecordMissing, kind, id)
	}
	return nil
}
