package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	sqlitedriver "modernc.org/sqlite"
)

// sqliteUniqueViolation is the extended result code for SQLITE_CONSTRAINT_UNIQUE
// (2067). modernc.org/sqlite returns extended codes from (*sqlite.Error).Code().
const sqliteUniqueViolation = 2067

// AppendJobEvent inserts a single append-only event row. The caller is
// responsible for assigning SequenceNumber within the persistence transaction
// (see nextJobEventSequence); this method validates the UNIQUE(job_id,
// sequence_number) constraint and surfaces a typed conflict error.
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO job_snapshots(
			job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			updated_at = excluded.updated_at
	`,
		snapshot.JobID, snapshot.JobKind, snapshot.ProjectID, snapshot.Status,
		snapshot.MaxSequence, snapshot.EarliestReplayableSequence,
		snapshot.InputTokens, snapshot.OutputTokens,
		snapshot.PendingAction, snapshot.TerminalInfo, formatTime(snapshot.UpdatedAt),
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
// ErrJobSnapshotMissing when no events have been appended for the job yet.
func (s *Store) JobSnapshot(ctx context.Context, jobID string) (agentdomain.JobSnapshot, error) {
	var (
		snapshot   agentdomain.JobSnapshot
		updatedAt  string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
			input_tokens, output_tokens, pending_action, terminal_info, updated_at
		FROM job_snapshots
		WHERE job_id = ?
	`, jobID).Scan(
		&snapshot.JobID, &snapshot.JobKind, &snapshot.ProjectID, &snapshot.Status,
		&snapshot.MaxSequence, &snapshot.EarliestReplayableSequence,
		&snapshot.InputTokens, &snapshot.OutputTokens,
		&snapshot.PendingAction, &snapshot.TerminalInfo, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return agentdomain.JobSnapshot{}, fmt.Errorf("%w: %s", ports.ErrJobSnapshotMissing, jobID)
	}
	if err != nil {
		return agentdomain.JobSnapshot{}, databaseOperationError("read job snapshot", err)
	}
	snapshot.UpdatedAt, err = parseTime(updatedAt)
	return snapshot, err
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
