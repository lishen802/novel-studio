-- +goose Up
DROP INDEX idx_change_operations_change_set;
DROP INDEX idx_change_sets_project_status;

ALTER TABLE change_operations RENAME TO change_operations_v9;
ALTER TABLE change_sets RENAME TO change_sets_v9;

CREATE TABLE change_sets (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('pending', 'partially_applied', 'applied', 'rejected', 'conflicted')
    ),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE change_operations (
    id TEXT PRIMARY KEY,
    change_set_id TEXT NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'replace_chapter_body',
            'update_outline_node',
            'update_character',
            'update_fact'
        )
    ),
    position INTEGER NOT NULL DEFAULT 1 CHECK (position >= 1),
    target_id TEXT NOT NULL,
    base_revision INTEGER NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    base_fingerprint TEXT NOT NULL DEFAULT '',
    before_value TEXT NOT NULL,
    after_value TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (
        status IN ('pending', 'applied', 'rejected', 'conflicted')
    ),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (before_value <> after_value),
    CHECK (
        (kind = 'replace_chapter_body' AND base_revision >= 1)
        OR
        (kind <> 'replace_chapter_body' AND base_revision = 0 AND base_fingerprint <> '')
    )
);

INSERT INTO change_sets(
    id, project_id, title, summary, created_by, status, created_at, updated_at
)
SELECT id, project_id, title, summary, created_by, status, created_at, updated_at
FROM change_sets_v9;

INSERT INTO change_operations(
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind,
    ROW_NUMBER() OVER (PARTITION BY change_set_id ORDER BY created_at, id),
    target_id, base_revision, '',
    before_value, after_value, reason, status, created_at, updated_at
FROM change_operations_v9;

DROP TABLE change_operations_v9;
DROP TABLE change_sets_v9;

CREATE INDEX idx_change_sets_project_status
    ON change_sets(project_id, status, created_at DESC);

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, position, id);

-- +goose Down
CREATE TEMP TABLE structured_change_set_downgrade_guard (
    value INTEGER NOT NULL
);

INSERT INTO structured_change_set_downgrade_guard(value)
SELECT NULL
WHERE EXISTS (
    SELECT 1
    FROM change_operations
    WHERE kind <> 'replace_chapter_body'
)
OR EXISTS (
    SELECT 1
    FROM change_sets
    WHERE status = 'partially_applied'
);

DROP TABLE structured_change_set_downgrade_guard;
DROP INDEX idx_change_operations_change_set;
DROP INDEX idx_change_sets_project_status;

ALTER TABLE change_operations RENAME TO change_operations_v10;
ALTER TABLE change_sets RENAME TO change_sets_v10;

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

INSERT INTO change_sets(
    id, project_id, title, summary, created_by, status, created_at, updated_at
)
SELECT id, project_id, title, summary, created_by, status, created_at, updated_at
FROM change_sets_v10;

INSERT INTO change_operations(
    id, change_set_id, kind, target_id, base_revision,
    before_value, after_value, reason, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind, target_id, base_revision,
    before_value, after_value, reason, status, created_at, updated_at
FROM change_operations_v10;

DROP TABLE change_operations_v10;
DROP TABLE change_sets_v10;

CREATE INDEX idx_change_sets_project_status
    ON change_sets(project_id, status, created_at DESC);

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, created_at, id);
