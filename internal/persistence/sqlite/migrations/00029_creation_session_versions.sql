-- +goose Up
ALTER TABLE creation_sessions ADD COLUMN content_version INTEGER NOT NULL DEFAULT 1 CHECK (content_version > 0);
ALTER TABLE creation_sessions ADD COLUMN run_slot_version INTEGER NOT NULL DEFAULT 0 CHECK (run_slot_version >= 0);

-- +goose Down
ALTER TABLE creation_sessions DROP COLUMN run_slot_version;
ALTER TABLE creation_sessions DROP COLUMN content_version;
