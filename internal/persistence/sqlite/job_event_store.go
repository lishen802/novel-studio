package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	sqlitedriver "modernc.org/sqlite"
)

// recoveryEventPayload marshals a small JSON payload for the job_recovered
// event appended by RecoverInterruptedAgentRuns/RecoverInterruptedTasks. The
// store builds it directly (rather than receiving it from the workspace layer)
// because recovery runs inside store.Open before the service layer is wired.
// A marshal failure (impossible for this simple map) yields an empty string so
// the event is still appended with a blank payload.
func recoveryEventPayload(reason, message string) string {
	encoded, err := json.Marshal(map[string]string{
		"reason":        reason,
		"error_message": message,
	})
	if err != nil {
		return ""
	}
	return string(encoded)
}

// sqliteUniqueViolation is the extended result code for SQLITE_CONSTRAINT_UNIQUE
// (2067). modernc.org/sqlite returns extended codes from (*sqlite.Error).Code().
const sqliteUniqueViolation = 2067

// txExecutor is satisfied by both *sql.DB and *sql.Tx, letting the
// transactional event-append helpers run against either. The update methods use
// it to append events and refresh the snapshot inside the same BeginTx that
// mutates the run/task row.
type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// AppendJobEvent inserts a single append-only event row with a caller-assigned
// sequence number. It is the low-level primitive used by tests and callers that
// assign sequences themselves; the transactional update methods
// (UpdateAgentRun/UpdateAgentRunStep/UpdateTask/UpdateStep) use
// appendJobEventDrafts instead so the sequence is assigned atomically with the
// run/task mutation.
func (s *Store) AppendJobEvent(ctx context.Context, event agentdomain.JobEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO job_events(
			job_id, job_kind, project_id, sequence_number, event_type, step_id, payload, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.JobID, event.JobKind, event.ProjectID, event.SequenceNumber,
		event.EventType, event.StepID, event.Payload, formatTime(event.CreatedAt),
	)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return fmt.Errorf("%w: job %s sequence %d", ports.ErrJobEventConflict, event.JobID, event.SequenceNumber)
		}
		return databaseOperationError("append job event", err)
	}
	return nil
}

// UpsertJobSnapshot inserts or replaces the authoritative projection for a job.
// It is intended to be called in the same transaction that appends the
// triggering JobEvent and mutates the run/task row.
func (s *Store) UpsertJobSnapshot(ctx context.Context, snapshot agentdomain.JobSnapshot) error {
	stepStates, err := json.Marshal(snapshot.StepStates)
	if err != nil {
		return fmt.Errorf("marshal job snapshot step states: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO job_snapshots(
			job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			job_kind = excluded.job_kind,
			project_id = excluded.project_id,
			status = excluded.status,
			max_sequence = excluded.max_sequence,
			earliest_replayable_sequence = excluded.earliest_replayable_sequence,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			pending_action = excluded.pending_action,
			terminal_info = excluded.terminal_info,
			updated_at = excluded.updated_at,
			step_states = excluded.step_states
	`,
		snapshot.JobID, snapshot.JobKind, snapshot.ProjectID, snapshot.Status,
		snapshot.MaxSequence, snapshot.EarliestReplayableSequence,
		snapshot.InputTokens, snapshot.OutputTokens,
		snapshot.PendingAction, snapshot.TerminalInfo, formatTime(snapshot.UpdatedAt), string(stepStates),
	)
	if err != nil {
		return databaseOperationError("upsert job snapshot", err)
	}
	return nil
}

// JobEvents returns events for a job with sequence_number > afterSequence in
// ascending order. The Service layer compares afterSequence against the
// snapshot's earliest_replayable_sequence to decide whether a snapshot rebuild
// is required; the store only performs the cursor read.
func (s *Store) JobEvents(ctx context.Context, jobID string, afterSequence uint64, limit int) ([]agentdomain.JobEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT job_id, job_kind, project_id, sequence_number, event_type, step_id, payload, created_at
		FROM job_events
		WHERE job_id = ? AND sequence_number > ?
		ORDER BY sequence_number ASC
		LIMIT ?
	`, jobID, afterSequence, limit)
	if err != nil {
		return nil, databaseOperationError("read job events", err)
	}
	defer rows.Close()
	result := make([]agentdomain.JobEvent, 0)
	for rows.Next() {
		event, err := scanJobEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

// JobSnapshot loads the authoritative projection for a job. Returns
// ErrJobSnapshotMissing only for an unknown/corrupt legacy job: new jobs write
// their initial job_queued event and snapshot atomically at creation time.
func (s *Store) JobSnapshot(ctx context.Context, jobID string) (agentdomain.JobSnapshot, error) {
	var (
		snapshot   agentdomain.JobSnapshot
		updatedAt  string
		stepStates string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
		FROM job_snapshots
		WHERE job_id = ?
	`, jobID).Scan(
		&snapshot.JobID, &snapshot.JobKind, &snapshot.ProjectID, &snapshot.Status,
		&snapshot.MaxSequence, &snapshot.EarliestReplayableSequence,
		&snapshot.InputTokens, &snapshot.OutputTokens,
		&snapshot.PendingAction, &snapshot.TerminalInfo, &updatedAt, &stepStates,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return agentdomain.JobSnapshot{}, fmt.Errorf("%w: %s", ports.ErrJobSnapshotMissing, jobID)
	}
	if err != nil {
		return agentdomain.JobSnapshot{}, databaseOperationError("read job snapshot", err)
	}
	snapshot.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal([]byte(stepStates), &snapshot.StepStates); err != nil {
		return agentdomain.JobSnapshot{}, fmt.Errorf("decode job snapshot step states: %w", err)
	}
	return snapshot, nil
}

// ListRecoverableJobs returns non-terminal (queued, running, recoverable) agent
// runs and extraction tasks for a project, newest first. max_sequence is
// folded in from job_snapshots when one exists; otherwise 0 (the job has not
// yet emitted an event).
func (s *Store) ListRecoverableJobs(ctx context.Context, projectID string) ([]agentdomain.JobSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT 'agent' AS job_kind, id, project_id, status,
			COALESCE((SELECT max_sequence FROM job_snapshots s WHERE s.job_id = agent_runs.id), 0) AS max_sequence,
			updated_at
		FROM agent_runs
		WHERE project_id = ? AND status IN ('queued', 'running', 'recoverable')
		UNION ALL
		SELECT 'extraction', id, project_id, status,
			COALESCE((SELECT max_sequence FROM job_snapshots s WHERE s.job_id = extraction_tasks.id), 0),
			updated_at
		FROM extraction_tasks
		WHERE project_id = ? AND status IN ('queued', 'running', 'recoverable')
		ORDER BY updated_at DESC
	`, projectID, projectID)
	if err != nil {
		return nil, databaseOperationError("list recoverable jobs", err)
	}
	defer rows.Close()
	result := make([]agentdomain.JobSummary, 0)
	for rows.Next() {
		var (
			summary   agentdomain.JobSummary
			updatedAt string
		)
		if err := rows.Scan(
			&summary.JobKind, &summary.JobID, &summary.ProjectID, &summary.Status,
			&summary.MaxSequence, &updatedAt,
		); err != nil {
			return nil, err
		}
		parsed, err := parseTime(updatedAt)
		if err != nil {
			return nil, err
		}
		summary.UpdatedAt = parsed
		result = append(result, summary)
	}
	return result, rows.Err()
}

func scanJobEvent(scanner rowScanner) (agentdomain.JobEvent, error) {
	var (
		event     agentdomain.JobEvent
		createdAt string
	)
	err := scanner.Scan(
		&event.JobID, &event.JobKind, &event.ProjectID, &event.SequenceNumber,
		&event.EventType, &event.StepID, &event.Payload, &createdAt,
	)
	if err != nil {
		return event, err
	}
	event.CreatedAt, err = parseTime(createdAt)
	return event, err
}

func isUniqueConstraintViolation(err error) bool {
	var sqliteErr *sqlitedriver.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqliteUniqueViolation
	}
	return false
}

// appendJobEventDrafts assigns per-job monotonic sequence numbers to drafts and
// inserts them within exec (a *sql.Tx or *sql.DB), returning the highest
// assigned sequence. It is intended to run inside the same transaction that
// mutates the run/task row so the event feed and the authoritative row commit
// atomically. Sequence assignment uses SELECT MAX(sequence_number)+1 guarded by
// the UNIQUE(job_id, sequence_number) constraint; with the single-connection
// pool this is race-free within the transaction.
func appendJobEventDrafts(
	ctx context.Context,
	exec txExecutor,
	drafts []agentdomain.JobEventDraft,
) (uint64, error) {
	if len(drafts) == 0 {
		return 0, nil
	}
	var currentMax uint64
	if err := exec.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(sequence_number), 0) FROM job_events WHERE job_id = ?
	`, drafts[0].JobID).Scan(&currentMax); err != nil {
		return 0, databaseOperationError("read job event max sequence", err)
	}
	for i, draft := range drafts {
		sequence := currentMax + uint64(i) + 1
		event, err := agentdomain.NewJobEvent(
			draft.JobID, draft.JobKind, draft.ProjectID, sequence,
			draft.EventType, draft.StepID, draft.Payload, draft.CreatedAt,
		)
		if err != nil {
			return 0, err
		}
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO job_events(
				job_id, job_kind, project_id, sequence_number, event_type, step_id, payload, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			event.JobID, event.JobKind, event.ProjectID, event.SequenceNumber,
			event.EventType, event.StepID, event.Payload, formatTime(event.CreatedAt),
		); err != nil {
			if isUniqueConstraintViolation(err) {
				return 0, fmt.Errorf("%w: job %s sequence %d", ports.ErrJobEventConflict, event.JobID, event.SequenceNumber)
			}
			return 0, databaseOperationError("append job event", err)
		}
	}
	return currentMax + uint64(len(drafts)), nil
}

// upsertAgentJobSnapshot refreshes the authoritative projection for an agent
// run by projecting status/tokens/project from the just-updated agent_runs row
// and advancing max_sequence. earliest_replayable_sequence defaults to 1 for a
// new snapshot and is preserved (via max()) when a future cleanup advances it.
// Runs inside the caller's transaction so the snapshot commits atomically with
// the event append.
func upsertAgentJobSnapshot(
	ctx context.Context,
	exec txExecutor,
	runID string,
	maxSequence uint64,
	updatedAt string,
) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO job_snapshots(
			job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
		)
		SELECT ?, 'agent', r.project_id, r.status, ?, 1,
			r.input_tokens, r.output_tokens, '', '', ?,
			COALESCE((
				SELECT json_group_array(json_object('stepId', ordered.id, 'status', ordered.status))
				FROM (
					SELECT st.id, st.status
					FROM agent_run_steps st
					WHERE st.run_id = r.id
					ORDER BY st.position, st.attempt
				) AS ordered
			), '[]')
		FROM agent_runs r
		WHERE r.id = ?
		ON CONFLICT(job_id) DO UPDATE SET
			job_kind = 'agent',
			project_id = excluded.project_id,
			status = excluded.status,
			max_sequence = excluded.max_sequence,
			earliest_replayable_sequence = max(excluded.earliest_replayable_sequence, job_snapshots.earliest_replayable_sequence),
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			pending_action = excluded.pending_action,
			terminal_info = excluded.terminal_info,
			updated_at = excluded.updated_at,
			step_states = excluded.step_states
	`, runID, maxSequence, updatedAt, runID)
	if err != nil {
		return databaseOperationError("upsert agent job snapshot", err)
	}
	return nil
}

// upsertExtractionJobSnapshot mirrors upsertAgentJobSnapshot for extraction
// tasks, projecting from the extraction_tasks row.
func upsertExtractionJobSnapshot(
	ctx context.Context,
	exec txExecutor,
	taskID string,
	maxSequence uint64,
	updatedAt string,
) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO job_snapshots(
			job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
		)
		SELECT ?, 'extraction', t.project_id, t.status, ?, 1,
			t.input_tokens, t.output_tokens, '', '', ?,
			COALESCE((
				SELECT json_group_array(json_object('stepId', st.id, 'status', st.status))
				FROM extraction_steps st WHERE st.task_id = t.id
			), '[]')
		FROM extraction_tasks t
		WHERE t.id = ?
		ON CONFLICT(job_id) DO UPDATE SET
			job_kind = 'extraction',
			project_id = excluded.project_id,
			status = excluded.status,
			max_sequence = excluded.max_sequence,
			earliest_replayable_sequence = max(excluded.earliest_replayable_sequence, job_snapshots.earliest_replayable_sequence),
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			pending_action = excluded.pending_action,
			terminal_info = excluded.terminal_info,
			updated_at = excluded.updated_at,
			step_states = excluded.step_states
	`, taskID, maxSequence, updatedAt, taskID)
	if err != nil {
		return databaseOperationError("upsert extraction job snapshot", err)
	}
	return nil
}
