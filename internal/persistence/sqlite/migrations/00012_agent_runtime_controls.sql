-- +goose Up
CREATE TABLE agent_runtime_settings (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    max_run_tokens INTEGER NOT NULL CHECK (max_run_tokens >= 1),
    max_context_chars INTEGER NOT NULL CHECK (max_context_chars >= 1),
    updated_at TEXT NOT NULL
);

ALTER TABLE agent_runs ADD COLUMN token_budget INTEGER NOT NULL DEFAULT 30000 CHECK (token_budget >= 1);
ALTER TABLE agent_runs ADD COLUMN context_char_budget INTEGER NOT NULL DEFAULT 120000 CHECK (context_char_budget >= 1);
ALTER TABLE agent_runs ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0);
ALTER TABLE agent_runs ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0);
ALTER TABLE agent_runs ADD COLUMN context_chars INTEGER NOT NULL DEFAULT 0 CHECK (context_chars >= 0);
ALTER TABLE agent_runs ADD COLUMN context_truncated INTEGER NOT NULL DEFAULT 0 CHECK (context_truncated IN (0, 1));

ALTER TABLE agent_run_steps ADD COLUMN provider_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_run_steps ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0);
ALTER TABLE agent_run_steps ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0);

-- +goose Down
ALTER TABLE agent_run_steps DROP COLUMN output_tokens;
ALTER TABLE agent_run_steps DROP COLUMN input_tokens;
ALTER TABLE agent_run_steps DROP COLUMN provider_request_id;

ALTER TABLE agent_runs DROP COLUMN context_truncated;
ALTER TABLE agent_runs DROP COLUMN context_chars;
ALTER TABLE agent_runs DROP COLUMN output_tokens;
ALTER TABLE agent_runs DROP COLUMN input_tokens;
ALTER TABLE agent_runs DROP COLUMN context_char_budget;
ALTER TABLE agent_runs DROP COLUMN token_budget;

DROP TABLE agent_runtime_settings;
