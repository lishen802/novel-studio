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

func (s *Store) SaveAgentRuntimeSettings(ctx context.Context, settings agentdomain.RuntimeSettings) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_runtime_settings(project_id, max_run_tokens, max_context_chars, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			max_run_tokens = excluded.max_run_tokens,
			max_context_chars = excluded.max_context_chars,
			updated_at = excluded.updated_at
	`, settings.ProjectID, settings.MaxRunTokens, settings.MaxContextChars, formatTime(settings.UpdatedAt))
	if err != nil {
		return databaseOperationError("save agent runtime settings", err)
	}
	return nil
}

func (s *Store) AgentRuntimeSettings(ctx context.Context, projectID string) (agentdomain.RuntimeSettings, error) {
	var settings agentdomain.RuntimeSettings
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT project_id, max_run_tokens, max_context_chars, updated_at
		FROM agent_runtime_settings WHERE project_id = ?
	`, projectID).Scan(
		&settings.ProjectID,
		&settings.MaxRunTokens,
		&settings.MaxContextChars,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		project, projectErr := s.Project(ctx)
		if projectErr != nil {
			return agentdomain.RuntimeSettings{}, projectErr
		}
		return agentdomain.NewRuntimeSettings(
			project.ID,
			agentdomain.DefaultMaxRunTokens,
			agentdomain.DefaultMaxContextChars,
			project.UpdatedAt,
		)
	}
	if err != nil {
		return agentdomain.RuntimeSettings{}, databaseOperationError("read agent runtime settings", err)
	}
	settings.UpdatedAt, err = parseTime(updatedAt)
	return settings, err
}

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
	return s.CreateAgentRunBundle(ctx, agentdomain.AgentRunBundle{Run: run})
}

func (s *Store) CreateAgentRunBundle(ctx context.Context, bundle agentdomain.AgentRunBundle) error {
	run := bundle.Run
	if err := validateAgentRunBundle(bundle); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin create agent run", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO agent_runs(
			id, project_id, chapter_id, chapter_base_revision, prompt, status,
			error_code, error_message, change_set_id, retry_of, created_at, updated_at,
			started_at, completed_at, token_budget, context_char_budget,
			input_tokens, output_tokens, context_chars, context_truncated,
			task_kind, workflow_id, workflow_version, workflow_hash, canonical_head,
			input_digest, quality_status, budget_policy_version, quality_policy_version,
			max_revision_attempts
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		run.ID, run.ProjectID, run.ChapterID, run.ChapterBaseRevision, run.Prompt, run.Status,
		run.ErrorCode, run.ErrorMessage, run.ChangeSetID, run.RetryOf,
		formatTime(run.CreatedAt), formatTime(run.UpdatedAt), nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
		run.TokenBudget, run.ContextCharBudget, run.InputTokens, run.OutputTokens,
		run.ContextChars, run.ContextTruncated,
		run.TaskKind, run.WorkflowBinding.ID, run.WorkflowBinding.Version, run.WorkflowBinding.Hash,
		run.CanonicalHead, run.InputDigest, run.QualityStatus, run.BudgetPolicyVersion,
		run.QualityPolicyVersion, run.MaxRevisionAttempts,
	)
	if err != nil {
		return databaseOperationError("create agent run", err)
	}
	for _, step := range run.Steps {
		if err := insertAgentRunStep(ctx, tx, step); err != nil {
			return err
		}
	}
	for _, artifact := range bundle.Artifacts {
		if err := insertAgentArtifact(ctx, tx, artifact); err != nil {
			return err
		}
	}
	for _, manifest := range bundle.Manifests {
		if err := insertContextManifest(ctx, tx, manifest); err != nil {
			return err
		}
	}
	for _, finding := range bundle.Findings {
		if err := insertAgentReviewFinding(ctx, tx, finding); err != nil {
			return err
		}
	}
	queuedDraft, err := agentdomain.NewJobEventDraft(
		run.ID, agentdomain.JobKindAgent, run.ProjectID, agentdomain.EventJobQueued,
		"", "", run.CreatedAt,
	)
	if err != nil {
		return err
	}
	maxSequence, err := appendJobEventDrafts(ctx, tx, []agentdomain.JobEventDraft{queuedDraft})
	if err != nil {
		return err
	}
	if err := upsertAgentJobSnapshot(ctx, tx, run.ID, maxSequence, formatTime(run.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit create agent run", err)
	}
	return nil
}

func (s *Store) UpdateAgentRun(ctx context.Context, run agentdomain.AgentRun, drafts ...agentdomain.JobEventDraft) error {
	if len(drafts) == 0 {
		result, err := execUpdateAgentRun(ctx, s.db, run)
		if err != nil {
			return databaseOperationError("update agent run", err)
		}
		return requireAffected(result, "agent run", run.ID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin update agent run", err)
	}
	defer tx.Rollback()
	result, err := execUpdateAgentRun(ctx, tx, run)
	if err != nil {
		return databaseOperationError("update agent run", err)
	}
	if err := requireAffected(result, "agent run", run.ID); err != nil {
		return err
	}
	maxSequence, err := appendJobEventDrafts(ctx, tx, drafts)
	if err != nil {
		return err
	}
	if err := upsertAgentJobSnapshot(ctx, tx, run.ID, maxSequence, formatTime(run.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit update agent run", err)
	}
	return nil
}

// execUpdateAgentRun runs the run-row UPDATE against exec (a *sql.DB or *sql.Tx)
// so UpdateAgentRun can reuse it both for the legacy single-statement path and
// for the transactional path that also appends events and refreshes the
// snapshot.
func execUpdateAgentRun(ctx context.Context, exec txExecutor, run agentdomain.AgentRun) (sql.Result, error) {
	return exec.ExecContext(ctx, `
		UPDATE agent_runs SET
			status = ?, error_code = ?, error_message = ?,
			change_set_id = NULLIF(?, ''), retry_of = NULLIF(?, ''),
			updated_at = ?, started_at = ?, completed_at = ?,
			token_budget = ?, context_char_budget = ?,
			input_tokens = ?, output_tokens = ?,
			context_chars = ?, context_truncated = ?,
			task_kind = ?, workflow_id = ?, workflow_version = ?, workflow_hash = ?,
			canonical_head = ?, input_digest = ?, quality_status = ?,
			budget_policy_version = ?, quality_policy_version = ?, max_revision_attempts = ?
		WHERE id = ?
	`,
		run.Status, run.ErrorCode, run.ErrorMessage, run.ChangeSetID, run.RetryOf,
		formatTime(run.UpdatedAt), nullableTime(run.StartedAt), nullableTime(run.CompletedAt),
		run.TokenBudget, run.ContextCharBudget, run.InputTokens, run.OutputTokens,
		run.ContextChars, run.ContextTruncated,
		run.TaskKind, run.WorkflowBinding.ID, run.WorkflowBinding.Version, run.WorkflowBinding.Hash,
		run.CanonicalHead, run.InputDigest, run.QualityStatus, run.BudgetPolicyVersion,
		run.QualityPolicyVersion, run.MaxRevisionAttempts, run.ID,
	)
}

func (s *Store) AgentRun(ctx context.Context, id string) (agentdomain.AgentRun, error) {
	run, err := scanAgentRun(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, chapter_id, chapter_base_revision, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			created_at, updated_at, started_at, completed_at,
			token_budget, context_char_budget, input_tokens, output_tokens,
			context_chars, context_truncated, task_kind, workflow_id, workflow_version,
			workflow_hash, canonical_head, input_digest, quality_status,
			budget_policy_version, quality_policy_version, max_revision_attempts
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
			created_at, updated_at, started_at, completed_at,
			token_budget, context_char_budget, input_tokens, output_tokens,
			context_chars, context_truncated, task_kind, workflow_id, workflow_version,
			workflow_hash, canonical_head, input_digest, quality_status,
			budget_policy_version, quality_policy_version, max_revision_attempts
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

func (s *Store) UpdateAgentRunStep(ctx context.Context, step agentdomain.AgentRunStep, drafts ...agentdomain.JobEventDraft) error {
	if len(drafts) == 0 {
		result, err := execUpdateAgentRunStep(ctx, s.db, step)
		if err != nil {
			return databaseOperationError("update agent run step", err)
		}
		return requireAffected(result, "agent run step", step.ID)
	}
	return s.SaveAgentStepBundle(ctx, agentdomain.AgentStepBundle{Step: step}, drafts...)
}

func (s *Store) SaveAgentStepBundle(ctx context.Context, bundle agentdomain.AgentStepBundle, drafts ...agentdomain.JobEventDraft) error {
	step := bundle.Step
	if err := validateAgentStepBundle(bundle); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin update agent run step", err)
	}
	defer tx.Rollback()
	for _, artifact := range bundle.Artifacts {
		if err := insertAgentArtifact(ctx, tx, artifact); err != nil {
			return err
		}
	}
	result, err := execUpdateAgentRunStep(ctx, tx, step)
	if err != nil {
		return databaseOperationError("update agent run step", err)
	}
	if err := requireAffected(result, "agent run step", step.ID); err != nil {
		return err
	}
	for _, manifest := range bundle.Manifests {
		if err := insertContextManifest(ctx, tx, manifest); err != nil {
			return err
		}
	}
	for _, finding := range bundle.Findings {
		if err := insertAgentReviewFinding(ctx, tx, finding); err != nil {
			return err
		}
	}
	if err := refreshAgentRunUsageFromSteps(ctx, tx, step.RunID, formatTime(step.UpdatedAt)); err != nil {
		return err
	}
	maxSequence := uint64(0)
	if len(drafts) > 0 {
		maxSequence, err = appendJobEventDrafts(ctx, tx, drafts)
		if err != nil {
			return err
		}
	} else if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence_number), 0) FROM job_events WHERE job_id = ?`, step.RunID).Scan(&maxSequence); err != nil {
		return databaseOperationError("read agent event cursor", err)
	}
	// The snapshot projects from the run row, which the step update does not
	// touch; the subsequent UpdateAgentRun (called by the workspace for terminal
	// and token-refresh transitions) refreshes status/tokens. This advance of
	// max_sequence keeps cursor replay aligned with the newly appended events.
	if err := upsertAgentJobSnapshot(ctx, tx, step.RunID, maxSequence, formatTime(step.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit update agent run step", err)
	}
	return nil
}

// refreshAgentRunUsageFromSteps keeps the parent run's cumulative usage in
// sync with its step rows before the JobSnapshot is projected. This makes the
// event and snapshot atomic even when the caller persists the parent run's
// other fields in a later transition.
func refreshAgentRunUsageFromSteps(ctx context.Context, exec txExecutor, runID, updatedAt string) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE agent_runs
		SET input_tokens = COALESCE((SELECT SUM(input_tokens) FROM agent_run_steps WHERE run_id = ?), 0),
			output_tokens = COALESCE((SELECT SUM(output_tokens) FROM agent_run_steps WHERE run_id = ?), 0),
			updated_at = ?
		WHERE id = ?
	`, runID, runID, updatedAt, runID)
	if err != nil {
		return databaseOperationError("refresh agent run usage", err)
	}
	return nil
}

// execUpdateAgentRunStep runs the step-row UPDATE against exec (a *sql.DB or
// *sql.Tx) so UpdateAgentRunStep can reuse it for both code paths.
func execUpdateAgentRunStep(ctx context.Context, exec txExecutor, step agentdomain.AgentRunStep) (sql.Result, error) {
	if step.Capability != "" {
		step.Output = agentdomain.BoundedStepPreview(step.Output)
	}
	return exec.ExecContext(ctx, `
		UPDATE agent_run_steps SET
			status = ?, output = ?, error_code = ?, error_message = ?,
			started_at = ?, completed_at = ?, updated_at = ?,
			provider_request_id = ?, input_tokens = ?, output_tokens = ?,
			node_id = ?, capability = ?, attempt = ?, prompt_id = ?, prompt_version = ?,
			prompt_hash = ?, schema_id = ?, schema_version = ?, schema_hash = ?,
			assembler_id = ?, assembler_version = ?, input_digest = ?, output_digest = ?,
			primary_artifact_id = NULLIF(?, ''), quality_status = ?
		WHERE id = ?
	`,
		step.Status, step.Output, step.ErrorCode, step.ErrorMessage,
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt), formatTime(step.UpdatedAt),
		step.ProviderRequestID, step.InputTokens, step.OutputTokens,
		normalizedNodeID(step), step.Capability, normalizedAttempt(step),
		step.PromptBinding.ID, step.PromptBinding.Version, step.PromptBinding.Hash,
		step.SchemaBinding.ID, step.SchemaBinding.Version, step.SchemaBinding.Hash,
		step.AssemblerID, step.AssemblerVersion, step.InputDigest, step.OutputDigest,
		step.PrimaryArtifactID, step.QualityStatus, step.ID,
	)
}

func (s *Store) CreateAgentArtifact(ctx context.Context, artifact agentdomain.AgentArtifact) error {
	if err := artifact.Validate(); err != nil {
		return err
	}
	return insertAgentArtifact(ctx, s.db, artifact)
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
		return 0, databaseOperationError("begin recover interrupted agent runs", err)
	}
	defer tx.Rollback()
	formatted := formatTime(recoveredAt)

	// Capture the running run IDs before the UPDATE so a job_recovered event
	// and a refreshed snapshot can be appended for each recovered run in the
	// same transaction. The event feed must reflect the recovery so the UI can
	// replay it on remount; otherwise the snapshot would say recoverable while
	// the event feed stops at job_started.
	recoveryRows, err := tx.QueryContext(ctx, `
		SELECT id, project_id FROM agent_runs WHERE status = 'running'
	`)
	if err != nil {
		return 0, databaseOperationError("list running agent runs for recovery", err)
	}
	type recoveredJob struct {
		runID     string
		projectID string
	}
	var recovered []recoveredJob
	for recoveryRows.Next() {
		var job recoveredJob
		if err := recoveryRows.Scan(&job.runID, &job.projectID); err != nil {
			recoveryRows.Close()
			return 0, err
		}
		recovered = append(recovered, job)
	}
	if err := recoveryRows.Err(); err != nil {
		recoveryRows.Close()
		return 0, err
	}
	recoveryRows.Close()

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

	// Append a job_recovered event and refresh the snapshot for each recovered
	// run. The sequence is assigned atomically within this transaction; the
	// snapshot projects the now-recoverable status from the just-updated
	// agent_runs row so the feed and the authoritative row commit together.
	for _, job := range recovered {
		draft, draftErr := agentdomain.NewJobEventDraft(
			job.runID, agentdomain.JobKindAgent, job.projectID,
			agentdomain.EventJobRecovered, "",
			recoveryEventPayload("app_interrupted", "应用上次退出时创作任务仍在运行，可重试该任务。"),
			recoveredAt,
		)
		if draftErr != nil {
			return 0, draftErr
		}
		maxSequence, err := appendJobEventDrafts(ctx, tx, []agentdomain.JobEventDraft{draft})
		if err != nil {
			return 0, err
		}
		if err := upsertAgentJobSnapshot(ctx, tx, job.runID, maxSequence, formatted); err != nil {
			return 0, err
		}
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, databaseOperationError("commit recover interrupted agent runs", err)
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
		&run.TokenBudget, &run.ContextCharBudget, &run.InputTokens, &run.OutputTokens,
		&run.ContextChars, &run.ContextTruncated,
		&run.TaskKind, &run.WorkflowBinding.ID, &run.WorkflowBinding.Version,
		&run.WorkflowBinding.Hash, &run.CanonicalHead, &run.InputDigest, &run.QualityStatus,
		&run.BudgetPolicyVersion, &run.QualityPolicyVersion, &run.MaxRevisionAttempts,
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
		SELECT id, run_id, role, COALESCE(profile_id, ''), position, status, output,
			error_code, error_message, started_at, completed_at, created_at, updated_at,
			provider_request_id, input_tokens, output_tokens,
			creative_profile_id, creative_profile_version, provider_connection_id,
			provider_name, provider_kind, base_url, credential_ref, model,
			max_output_tokens, temperature,
			node_id, capability, attempt, prompt_id, prompt_version, prompt_hash,
			schema_id, schema_version, schema_hash, assembler_id, assembler_version,
			input_digest, output_digest, COALESCE(primary_artifact_id, ''), quality_status
		FROM agent_run_steps WHERE run_id = ? ORDER BY position, attempt
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
			&step.ProviderRequestID, &step.InputTokens, &step.OutputTokens,
			&step.CreativeProfileID, &step.CreativeProfileVersion, &step.ProviderConnectionID,
			&step.ProviderName, &step.ProviderKind, &step.BaseURL, &step.CredentialRef, &step.Model,
			&step.MaxOutputTokens, &step.Temperature,
			&step.NodeID, &step.Capability, &step.Attempt,
			&step.PromptBinding.ID, &step.PromptBinding.Version, &step.PromptBinding.Hash,
			&step.SchemaBinding.ID, &step.SchemaBinding.Version, &step.SchemaBinding.Hash,
			&step.AssemblerID, &step.AssemblerVersion, &step.InputDigest, &step.OutputDigest,
			&step.PrimaryArtifactID, &step.QualityStatus,
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
	if step.Capability != "" {
		step.Output = agentdomain.BoundedStepPreview(step.Output)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO agent_run_steps(
			id, run_id, role, profile_id, position, status, output,
			error_code, error_message, started_at, completed_at, created_at, updated_at,
			provider_request_id, input_tokens, output_tokens,
			creative_profile_id, creative_profile_version, provider_connection_id,
			provider_name, provider_kind, base_url, credential_ref, model,
			max_output_tokens, temperature, node_id, capability, attempt,
			prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
			assembler_id, assembler_version, input_digest, output_digest,
			primary_artifact_id, quality_status
		) VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
	`,
		step.ID, step.RunID, step.Role, step.ProfileID, step.Position, step.Status,
		step.Output, step.ErrorCode, step.ErrorMessage,
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt),
		formatTime(step.CreatedAt), formatTime(step.UpdatedAt),
		step.ProviderRequestID, step.InputTokens, step.OutputTokens,
		step.CreativeProfileID, step.CreativeProfileVersion, step.ProviderConnectionID,
		step.ProviderName, step.ProviderKind, step.BaseURL, step.CredentialRef, step.Model,
		step.MaxOutputTokens, step.Temperature,
		normalizedNodeID(step), step.Capability, normalizedAttempt(step),
		step.PromptBinding.ID, step.PromptBinding.Version, step.PromptBinding.Hash,
		step.SchemaBinding.ID, step.SchemaBinding.Version, step.SchemaBinding.Hash,
		step.AssemblerID, step.AssemblerVersion, step.InputDigest, step.OutputDigest,
		step.PrimaryArtifactID, step.QualityStatus,
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
