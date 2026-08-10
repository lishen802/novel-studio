-- +goose Up
-- Store the compact step projection needed to rebuild an in-flight job, then
-- backfill the initial queued event/snapshot that R1.2 did not create for
-- already-persisted queued jobs.

ALTER TABLE job_snapshots ADD COLUMN step_states TEXT NOT NULL DEFAULT '[]';

INSERT INTO job_events(
    job_id, job_kind, project_id, sequence_number, event_type, step_id, payload, created_at
)
SELECT r.id, 'agent', r.project_id, 1, 'job_queued', '', '{}', r.created_at
FROM agent_runs r
WHERE r.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM job_snapshots s WHERE s.job_id = r.id);

INSERT INTO job_snapshots(
    job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
    input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
)
SELECT r.id, 'agent', r.project_id, r.status, 1, 1,
       r.input_tokens, r.output_tokens, '', '', r.updated_at,
       COALESCE((
           SELECT json_group_array(json_object('stepId', st.id, 'status', st.status))
           FROM agent_run_steps st WHERE st.run_id = r.id
       ), '[]')
FROM agent_runs r
WHERE r.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM job_snapshots s WHERE s.job_id = r.id);

INSERT INTO job_events(
    job_id, job_kind, project_id, sequence_number, event_type, step_id, payload, created_at
)
SELECT t.id, 'extraction', t.project_id, 1, 'job_queued', '', '{}', t.created_at
FROM extraction_tasks t
WHERE t.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM job_snapshots s WHERE s.job_id = t.id);

INSERT INTO job_snapshots(
    job_id, job_kind, project_id, status, max_sequence, earliest_replayable_sequence,
    input_tokens, output_tokens, pending_action, terminal_info, updated_at, step_states
)
SELECT t.id, 'extraction', t.project_id, t.status, 1, 1,
       t.input_tokens, t.output_tokens, '', '', t.updated_at,
       COALESCE((
           SELECT json_group_array(json_object('stepId', st.id, 'status', st.status))
           FROM extraction_steps st WHERE st.task_id = t.id
       ), '[]')
FROM extraction_tasks t
WHERE t.status = 'queued'
  AND NOT EXISTS (SELECT 1 FROM job_snapshots s WHERE s.job_id = t.id);

-- +goose Down
-- SQLite cannot DROP COLUMN on older supported engines without rebuilding the
-- table. Down migrations are intentionally unsupported once an R1.2 project
-- has been upgraded, matching the existing non-destructive migration policy.
SELECT 1;
