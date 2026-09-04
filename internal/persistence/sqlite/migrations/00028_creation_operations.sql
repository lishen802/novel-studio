-- +goose Up
ALTER TABLE creation_sessions ADD COLUMN selected_direction_card_id TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_sessions ADD COLUMN kept_direction_ids_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE creation_sessions ADD COLUMN active_operation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_candidates ADD COLUMN quality_status TEXT NOT NULL DEFAULT '';

-- 创作操作回执：持久化幂等键与运行/候选关联，跨重启去重付费调用。
CREATE TABLE creation_operations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    branch_id TEXT NOT NULL,
    command TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    arguments_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('reserved', 'running', 'succeeded', 'failed')),
    run_id TEXT NOT NULL DEFAULT '',
    candidate_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, branch_id, command, idempotency_key)
);
CREATE INDEX idx_creation_operations_project
    ON creation_operations(project_id, updated_at DESC);

-- +goose Down
DROP INDEX idx_creation_operations_project;
DROP TABLE creation_operations;
ALTER TABLE creation_candidates DROP COLUMN quality_status;
ALTER TABLE creation_sessions DROP COLUMN active_operation_id;
ALTER TABLE creation_sessions DROP COLUMN kept_direction_ids_json;
ALTER TABLE creation_sessions DROP COLUMN selected_direction_card_id;
