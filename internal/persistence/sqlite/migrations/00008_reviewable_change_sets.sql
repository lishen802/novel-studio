-- +goose Up
CREATE TABLE change_sets (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'applied', 'rejected', 'conflicted')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE change_operations (
    id TEXT PRIMARY KEY,
    change_set_id TEXT NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('replace_chapter_body')),
    target_id TEXT NOT NULL,
    base_revision INTEGER NOT NULL CHECK (base_revision >= 1),
    before_value TEXT NOT NULL,
    after_value TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'applied', 'rejected', 'conflicted')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (before_value <> after_value)
);

CREATE INDEX idx_change_sets_project_status
    ON change_sets(project_id, status, created_at DESC);

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, created_at, id);

-- +goose Down
DROP TABLE change_operations;
DROP TABLE change_sets;
