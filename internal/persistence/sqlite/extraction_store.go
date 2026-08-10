package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
)

var _ ports.ExtractionStore = (*Store)(nil)

func (s *Store) CreateTask(ctx context.Context, task extractiondomain.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin create extraction task", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO extraction_tasks(
			id, project_id, import_job_id, connection_id, model, prompt, status,
			error_code, error_message, change_set_id, retry_of,
			token_budget, context_char_budget, input_tokens, output_tokens,
			created_at, updated_at, started_at, completed_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''),
			?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		task.ID, task.ProjectID, task.ImportJobID, task.ConnectionID, task.Model, task.Prompt,
		task.Status, task.ErrorCode, task.ErrorMessage, task.ChangeSetID, task.RetryOf,
		task.TokenBudget, task.ContextCharBudget, task.InputTokens, task.OutputTokens,
		formatTime(task.CreatedAt), formatTime(task.UpdatedAt),
		nullableTime(task.StartedAt), nullableTime(task.CompletedAt),
	)
	if err != nil {
		return databaseOperationError("create extraction task", err)
	}
	for _, step := range task.Steps {
		if err := insertExtractionStep(ctx, tx, step); err != nil {
			return err
		}
	}
	queuedDraft, err := agentdomain.NewJobEventDraft(
		task.ID, agentdomain.JobKindExtraction, task.ProjectID, agentdomain.EventJobQueued,
		"", "", task.CreatedAt,
	)
	if err != nil {
		return err
	}
	maxSequence, err := appendJobEventDrafts(ctx, tx, []agentdomain.JobEventDraft{queuedDraft})
	if err != nil {
		return err
	}
	if err := upsertExtractionJobSnapshot(ctx, tx, task.ID, maxSequence, formatTime(task.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit create extraction task", err)
	}
	return nil
}

func insertExtractionStep(ctx context.Context, tx *sql.Tx, step extractiondomain.Step) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO extraction_steps(
			id, task_id, import_chapter_id, created_chapter_id, chapter_title, position,
			source_start_line, source_end_line,
			status, output, error_code, error_message,
			input_tokens, output_tokens, provider_request_id,
			created_at, updated_at, started_at, completed_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		step.ID, step.TaskID, step.ImportChapterID, step.CreatedChapterID, step.ChapterTitle, step.Position,
		step.SourceStartLine, step.SourceEndLine,
		step.Status, step.Output, step.ErrorCode, step.ErrorMessage,
		step.InputTokens, step.OutputTokens, step.ProviderRequestID,
		formatTime(step.CreatedAt), formatTime(step.UpdatedAt),
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt),
	)
	if err != nil {
		return databaseOperationError("create extraction step", err)
	}
	return nil
}

func (s *Store) UpdateTask(ctx context.Context, task extractiondomain.Task, drafts ...agentdomain.JobEventDraft) error {
	if len(drafts) == 0 {
		result, err := execUpdateTask(ctx, s.db, task)
		if err != nil {
			return databaseOperationError("update extraction task", err)
		}
		return requireAffected(result, "extraction task", task.ID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin update extraction task", err)
	}
	defer tx.Rollback()
	result, err := execUpdateTask(ctx, tx, task)
	if err != nil {
		return databaseOperationError("update extraction task", err)
	}
	if err := requireAffected(result, "extraction task", task.ID); err != nil {
		return err
	}
	maxSequence, err := appendJobEventDrafts(ctx, tx, drafts)
	if err != nil {
		return err
	}
	if err := upsertExtractionJobSnapshot(ctx, tx, task.ID, maxSequence, formatTime(task.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit update extraction task", err)
	}
	return nil
}

// execUpdateTask runs the task-row UPDATE against exec (a *sql.DB or *sql.Tx)
// so UpdateTask can reuse it for both code paths.
func execUpdateTask(ctx context.Context, exec txExecutor, task extractiondomain.Task) (sql.Result, error) {
	return exec.ExecContext(ctx, `
		UPDATE extraction_tasks SET
			status = ?, error_code = ?, error_message = ?,
			change_set_id = NULLIF(?, ''), retry_of = NULLIF(?, ''),
			updated_at = ?, started_at = ?, completed_at = ?,
			token_budget = ?, context_char_budget = ?,
			input_tokens = ?, output_tokens = ?
		WHERE id = ?
	`,
		task.Status, task.ErrorCode, task.ErrorMessage,
		task.ChangeSetID, task.RetryOf,
		formatTime(task.UpdatedAt), nullableTime(task.StartedAt), nullableTime(task.CompletedAt),
		task.TokenBudget, task.ContextCharBudget,
		task.InputTokens, task.OutputTokens,
		task.ID,
	)
}

func (s *Store) UpdateStep(ctx context.Context, step extractiondomain.Step, drafts ...agentdomain.JobEventDraft) error {
	if len(drafts) == 0 {
		result, err := execUpdateStep(ctx, s.db, step)
		if err != nil {
			return databaseOperationError("update extraction step", err)
		}
		return requireAffected(result, "extraction step", step.ID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin update extraction step", err)
	}
	defer tx.Rollback()
	result, err := execUpdateStep(ctx, tx, step)
	if err != nil {
		return databaseOperationError("update extraction step", err)
	}
	if err := requireAffected(result, "extraction step", step.ID); err != nil {
		return err
	}
	if err := refreshExtractionTaskUsageFromSteps(ctx, tx, step.TaskID, formatTime(step.UpdatedAt)); err != nil {
		return err
	}
	maxSequence, err := appendJobEventDrafts(ctx, tx, drafts)
	if err != nil {
		return err
	}
	// See UpdateAgentRunStep: the snapshot projects from the task row and the
	// subsequent UpdateTask refreshes status/tokens; this only advances
	// max_sequence to keep cursor replay aligned with the new events.
	if err := upsertExtractionJobSnapshot(ctx, tx, step.TaskID, maxSequence, formatTime(step.UpdatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit update extraction step", err)
	}
	return nil
}

// refreshExtractionTaskUsageFromSteps mirrors the agent-side aggregation so a
// step event and its authoritative snapshot always carry the same usage.
func refreshExtractionTaskUsageFromSteps(ctx context.Context, exec txExecutor, taskID, updatedAt string) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE extraction_tasks
		SET input_tokens = COALESCE((SELECT SUM(input_tokens) FROM extraction_steps WHERE task_id = ?), 0),
			output_tokens = COALESCE((SELECT SUM(output_tokens) FROM extraction_steps WHERE task_id = ?), 0),
			updated_at = ?
		WHERE id = ?
	`, taskID, taskID, updatedAt, taskID)
	if err != nil {
		return databaseOperationError("refresh extraction task usage", err)
	}
	return nil
}

// execUpdateStep runs the step-row UPDATE against exec (a *sql.DB or *sql.Tx)
// so UpdateStep can reuse it for both code paths.
func execUpdateStep(ctx context.Context, exec txExecutor, step extractiondomain.Step) (sql.Result, error) {
	return exec.ExecContext(ctx, `
		UPDATE extraction_steps SET
			status = ?, output = ?, error_code = ?, error_message = ?,
			started_at = ?, completed_at = ?, updated_at = ?,
			provider_request_id = ?, input_tokens = ?, output_tokens = ?
		WHERE id = ?
	`,
		step.Status, step.Output, step.ErrorCode, step.ErrorMessage,
		nullableTime(step.StartedAt), nullableTime(step.CompletedAt), formatTime(step.UpdatedAt),
		step.ProviderRequestID, step.InputTokens, step.OutputTokens, step.ID,
	)
}

func (s *Store) Task(ctx context.Context, id string) (extractiondomain.Task, error) {
	task, err := scanExtractionTask(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, import_job_id, connection_id, model, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			token_budget, context_char_budget, input_tokens, output_tokens,
			created_at, updated_at, started_at, completed_at
		FROM extraction_tasks WHERE id = ?
	`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return extractiondomain.Task{}, fmt.Errorf("%w: %s", ports.ErrExtractionTaskMissing, id)
	}
	if err != nil {
		return extractiondomain.Task{}, databaseOperationError("read extraction task", err)
	}
	task.Steps, err = s.listExtractionSteps(ctx, id)
	return task, err
}

func (s *Store) ListTasks(ctx context.Context, projectID string, limit int) ([]extractiondomain.Task, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, import_job_id, connection_id, model, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			token_budget, context_char_budget, input_tokens, output_tokens,
			created_at, updated_at, started_at, completed_at
		FROM extraction_tasks
		WHERE project_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, projectID, limit)
	if err != nil {
		return nil, databaseOperationError("list extraction tasks", err)
	}
	defer rows.Close()
	tasks := make([]extractiondomain.Task, 0)
	for rows.Next() {
		task, err := scanExtractionTask(rows)
		if err != nil {
			return nil, databaseOperationError("scan extraction task", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate extraction tasks", err)
	}
	for index := range tasks {
		tasks[index].Steps, err = s.listExtractionSteps(ctx, tasks[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (s *Store) TasksForImport(ctx context.Context, importJobID string) ([]extractiondomain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, import_job_id, connection_id, model, prompt, status,
			error_code, error_message, COALESCE(change_set_id, ''), COALESCE(retry_of, ''),
			token_budget, context_char_budget, input_tokens, output_tokens,
			created_at, updated_at, started_at, completed_at
		FROM extraction_tasks
		WHERE import_job_id = ?
		ORDER BY created_at DESC, id DESC
	`, importJobID)
	if err != nil {
		return nil, databaseOperationError("list extraction tasks for import", err)
	}
	defer rows.Close()
	tasks := make([]extractiondomain.Task, 0)
	for rows.Next() {
		task, err := scanExtractionTask(rows)
		if err != nil {
			return nil, databaseOperationError("scan extraction task", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate extraction tasks for import", err)
	}
	for index := range tasks {
		tasks[index].Steps, err = s.listExtractionSteps(ctx, tasks[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (s *Store) listExtractionSteps(ctx context.Context, taskID string) ([]extractiondomain.Step, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, import_chapter_id, created_chapter_id, chapter_title, position,
			source_start_line, source_end_line,
			status, output, error_code, error_message,
			input_tokens, output_tokens, provider_request_id,
			created_at, updated_at, started_at, completed_at
		FROM extraction_steps
		WHERE task_id = ?
		ORDER BY position, id
	`, taskID)
	if err != nil {
		return nil, databaseOperationError("list extraction steps", err)
	}
	defer rows.Close()
	steps := make([]extractiondomain.Step, 0)
	for rows.Next() {
		step, err := scanExtractionStep(rows)
		if err != nil {
			return nil, databaseOperationError("scan extraction step", err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate extraction steps", err)
	}
	return steps, nil
}

func (s *Store) CreateArtifact(ctx context.Context, artifact ports.ExtractionArtifact) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO extraction_artifacts(id, task_id, step_id, kind, content, sha256, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`, artifact.ID, artifact.TaskID, artifact.StepID, artifact.Kind, artifact.Content,
		artifact.SHA256, formatTime(artifact.CreatedAt))
	if err != nil {
		return databaseOperationError("create extraction artifact", err)
	}
	return nil
}

func (s *Store) ListArtifacts(ctx context.Context, taskID string) ([]ports.ExtractionArtifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, COALESCE(step_id, ''), kind, content, sha256, created_at
		FROM extraction_artifacts
		WHERE task_id = ?
		ORDER BY created_at, id
	`, taskID)
	if err != nil {
		return nil, databaseOperationError("list extraction artifacts", err)
	}
	defer rows.Close()
	result := make([]ports.ExtractionArtifact, 0)
	for rows.Next() {
		var artifact ports.ExtractionArtifact
		var createdAt string
		if err := rows.Scan(
			&artifact.ID, &artifact.TaskID, &artifact.StepID, &artifact.Kind,
			&artifact.Content, &artifact.SHA256, &createdAt,
		); err != nil {
			return nil, databaseOperationError("scan extraction artifact", err)
		}
		artifact.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse extraction artifact created_at: %w", err)
		}
		result = append(result, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate extraction artifacts", err)
	}
	return result, nil
}

func (s *Store) RecoverInterruptedTasks(ctx context.Context, recoveredAt time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, databaseOperationError("begin recover interrupted extraction tasks", err)
	}
	defer tx.Rollback()
	formatted := formatTime(recoveredAt)

	// Capture the running task IDs before the UPDATE so a job_recovered event
	// and a refreshed snapshot can be appended for each recovered task in the
	// same transaction, mirroring RecoverInterruptedAgentRuns.
	recoveryRows, err := tx.QueryContext(ctx, `
		SELECT id, project_id FROM extraction_tasks WHERE status = 'running'
	`)
	if err != nil {
		return 0, databaseOperationError("list running extraction tasks for recovery", err)
	}
	type recoveredJob struct {
		taskID    string
		projectID string
	}
	var recovered []recoveredJob
	for recoveryRows.Next() {
		var job recoveredJob
		if err := recoveryRows.Scan(&job.taskID, &job.projectID); err != nil {
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
		UPDATE extraction_tasks
		SET status = 'recoverable',
			error_code = 'app_interrupted',
			error_message = '应用上次退出时抽取任务仍在运行，可重试该任务。',
			updated_at = ?
		WHERE status = 'running'
	`, formatted)
	if err != nil {
		return 0, databaseOperationError("recover interrupted extraction tasks", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE extraction_steps
		SET status = 'failed',
			error_code = 'app_interrupted',
			error_message = '应用退出导致步骤中断。',
			completed_at = ?,
			updated_at = ?
		WHERE status = 'running'
	`, formatted, formatted); err != nil {
		return 0, databaseOperationError("recover interrupted extraction steps", err)
	}

	// Append a job_recovered event and refresh the snapshot for each recovered
	// task so the extraction feed stays consistent with the authoritative row.
	for _, job := range recovered {
		draft, draftErr := agentdomain.NewJobEventDraft(
			job.taskID, agentdomain.JobKindExtraction, job.projectID,
			agentdomain.EventJobRecovered, "",
			recoveryEventPayload("app_interrupted", "应用上次退出时抽取任务仍在运行，可重试该任务。"),
			recoveredAt,
		)
		if draftErr != nil {
			return 0, draftErr
		}
		maxSequence, err := appendJobEventDrafts(ctx, tx, []agentdomain.JobEventDraft{draft})
		if err != nil {
			return 0, err
		}
		if err := upsertExtractionJobSnapshot(ctx, tx, job.taskID, maxSequence, formatted); err != nil {
			return 0, err
		}
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, databaseOperationError("commit recover interrupted extraction tasks", err)
	}
	return count, nil
}

func scanExtractionTask(scanner rowScanner) (extractiondomain.Task, error) {
	var task extractiondomain.Task
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	err := scanner.Scan(
		&task.ID, &task.ProjectID, &task.ImportJobID, &task.ConnectionID, &task.Model, &task.Prompt,
		&task.Status, &task.ErrorCode, &task.ErrorMessage, &task.ChangeSetID, &task.RetryOf,
		&task.TokenBudget, &task.ContextCharBudget, &task.InputTokens, &task.OutputTokens,
		&createdAt, &updatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return extractiondomain.Task{}, err
	}
	task.CreatedAt, err = parseTime(createdAt)
	if err == nil {
		task.UpdatedAt, err = parseTime(updatedAt)
	}
	if err == nil {
		task.StartedAt, err = parseNullableTime(startedAt)
	}
	if err == nil {
		task.CompletedAt, err = parseNullableTime(completedAt)
	}
	return task, err
}

func scanExtractionStep(scanner rowScanner) (extractiondomain.Step, error) {
	var step extractiondomain.Step
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	err := scanner.Scan(
		&step.ID, &step.TaskID, &step.ImportChapterID, &step.CreatedChapterID, &step.ChapterTitle, &step.Position,
		&step.SourceStartLine, &step.SourceEndLine,
		&step.Status, &step.Output, &step.ErrorCode, &step.ErrorMessage,
		&step.InputTokens, &step.OutputTokens, &step.ProviderRequestID,
		&createdAt, &updatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return extractiondomain.Step{}, err
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
	return step, err
}
