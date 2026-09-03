-- +goose Up
ALTER TABLE assistant_actions ADD COLUMN result_refs_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE assistant_actions DROP COLUMN result_refs_json;
