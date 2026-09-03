-- +goose Up
ALTER TABLE creation_sessions ADD COLUMN active_run_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE creation_sessions DROP COLUMN active_run_id;
