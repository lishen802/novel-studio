-- +goose Up
CREATE TABLE provider_connections (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('openai', 'anthropic')),
    base_url TEXT NOT NULL,
    credential_ref TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE agent_profiles (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('planner', 'writer', 'continuity_reviewer')),
    connection_id TEXT NOT NULL REFERENCES provider_connections(id) ON DELETE RESTRICT,
    model TEXT NOT NULL,
    max_output_tokens INTEGER NOT NULL CHECK (max_output_tokens >= 1),
    temperature REAL NOT NULL CHECK (temperature >= 0 AND temperature <= 2),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, role)
);

CREATE TABLE agent_runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_base_revision INTEGER NOT NULL CHECK (chapter_base_revision >= 1),
    prompt TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'recoverable')
    ),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    change_set_id TEXT REFERENCES change_sets(id) ON DELETE SET NULL,
    retry_of TEXT REFERENCES agent_runs(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT
);

CREATE TABLE agent_run_steps (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('planner', 'writer', 'continuity_reviewer')),
    profile_id TEXT NOT NULL REFERENCES agent_profiles(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(run_id, position)
);

CREATE TABLE agent_artifacts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_id TEXT REFERENCES agent_run_steps(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('context', 'plan', 'draft', 'review')),
    content TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_provider_connections_project ON provider_connections(project_id, name);
CREATE INDEX idx_agent_profiles_project_role ON agent_profiles(project_id, role);
CREATE INDEX idx_agent_runs_project_created ON agent_runs(project_id, created_at DESC);
CREATE INDEX idx_agent_run_steps_run_position ON agent_run_steps(run_id, position);
CREATE INDEX idx_agent_artifacts_run_created ON agent_artifacts(run_id, created_at);

-- +goose Down
DROP INDEX idx_agent_artifacts_run_created;
DROP INDEX idx_agent_run_steps_run_position;
DROP INDEX idx_agent_runs_project_created;
DROP INDEX idx_agent_profiles_project_role;
DROP INDEX idx_provider_connections_project;
DROP TABLE agent_artifacts;
DROP TABLE agent_run_steps;
DROP TABLE agent_runs;
DROP TABLE agent_profiles;
DROP TABLE provider_connections;
