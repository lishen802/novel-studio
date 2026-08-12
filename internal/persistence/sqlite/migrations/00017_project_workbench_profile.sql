-- +goose Up
CREATE TABLE project_creative_profile (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    profile_id TEXT NOT NULL,
    profile_version INTEGER NOT NULL,
    role_overrides_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL
);

CREATE TABLE project_workbench_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    stage TEXT NOT NULL DEFAULT 'idea',
    canvas TEXT NOT NULL DEFAULT '',
    ai_panel TEXT NOT NULL DEFAULT 'hidden',
    pending_review_count INTEGER NOT NULL DEFAULT 0,
    recoverable_job_count INTEGER NOT NULL DEFAULT 0,
    archive_status TEXT NOT NULL DEFAULT 'not_generated',
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE project_workbench_state;
DROP TABLE project_creative_profile;
