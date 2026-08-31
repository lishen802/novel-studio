-- +goose Up
ALTER TABLE creative_profile_versions ADD COLUMN run_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (run_budget_unlimited IN (0, 1));
ALTER TABLE creative_profile_versions ADD COLUMN context_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (context_budget_unlimited IN (0, 1));
ALTER TABLE creative_profile_roles ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE creative_profile_roles ADD COLUMN output_limit_mode TEXT NOT NULL DEFAULT 'fixed';

-- +goose Down
ALTER TABLE creative_profile_roles DROP COLUMN output_limit_mode;
ALTER TABLE creative_profile_roles DROP COLUMN reasoning_effort;
ALTER TABLE creative_profile_versions DROP COLUMN context_budget_unlimited;
ALTER TABLE creative_profile_versions DROP COLUMN run_budget_unlimited;
