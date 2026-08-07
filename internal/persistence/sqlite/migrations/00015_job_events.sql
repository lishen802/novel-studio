-- +goose Up
-- JobEvent feed: append-only event history and authoritative snapshot
-- projection for agent and extraction jobs. Events are appended in the same
-- short transaction that mutates agent_runs/extraction_tasks, so the feed is
-- the authoritative history once committed. job_id intentionally has no
-- foreign key because it references either agent_runs(id) or extraction_tasks(id);
-- project_id is denormalized so deleting a project cascades to its event
-- history (referential integrity, not retention cleanup).

CREATE TABLE job_events (
    job_id TEXT NOT NULL,
    job_kind TEXT NOT NULL CHECK (job_kind IN ('agent', 'extraction')),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sequence_number INTEGER NOT NULL CHECK (sequence_number >= 1),
    event_type TEXT NOT NULL,
    step_id TEXT NOT NULL DEFAULT '',
    payload TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(job_id, sequence_number)
);

CREATE INDEX idx_job_events_id_sequence ON job_events(job_id, sequence_number);
CREATE INDEX idx_job_events_project ON job_events(project_id, created_at);

CREATE TABLE job_snapshots (
    job_id TEXT PRIMARY KEY,
    job_kind TEXT NOT NULL CHECK (job_kind IN ('agent', 'extraction')),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    max_sequence INTEGER NOT NULL CHECK (max_sequence >= 1),
    earliest_replayable_sequence INTEGER NOT NULL CHECK (earliest_replayable_sequence >= 1),
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    pending_action TEXT NOT NULL DEFAULT '',
    terminal_info TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    CHECK (max_sequence >= earliest_replayable_sequence)
);

CREATE INDEX idx_job_snapshots_project_status ON job_snapshots(project_id, status);

-- +goose Down
DROP INDEX idx_job_snapshots_project_status;
DROP TABLE job_snapshots;

DROP INDEX idx_job_events_project;
DROP INDEX idx_job_events_id_sequence;
DROP TABLE job_events;
