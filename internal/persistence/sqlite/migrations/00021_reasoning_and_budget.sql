-- +goose Up
ALTER TABLE agent_runs ADD COLUMN token_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (token_budget_unlimited IN (0, 1));
ALTER TABLE agent_runs ADD COLUMN context_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (context_budget_unlimited IN (0, 1));
ALTER TABLE agent_run_steps ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_run_steps ADD COLUMN output_limit_mode TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE agent_run_steps DROP COLUMN output_limit_mode;
ALTER TABLE agent_run_steps DROP COLUMN reasoning_effort;
ALTER TABLE agent_runs DROP COLUMN context_budget_unlimited;
ALTER TABLE agent_runs DROP COLUMN token_budget_unlimited;
