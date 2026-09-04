-- +goose Up
ALTER TABLE creation_candidates ADD COLUMN stale_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_candidates ADD COLUMN impact_refs_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE creation_candidates ADD COLUMN upstream_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_candidates ADD COLUMN excepted_upstream_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_candidates ADD COLUMN impact_severity TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE creation_candidates DROP COLUMN impact_severity;
ALTER TABLE creation_candidates DROP COLUMN excepted_upstream_digest;
ALTER TABLE creation_candidates DROP COLUMN upstream_digest;
ALTER TABLE creation_candidates DROP COLUMN impact_refs_json;
ALTER TABLE creation_candidates DROP COLUMN stale_reason;
