-- +goose NO TRANSACTION
-- +goose Up
-- 档案确认需要把方案信封、势力、地点、锚点纳入同一 ChangeSet。
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_change_operations_change_set;

CREATE TABLE change_operations_new (
    id TEXT PRIMARY KEY,
    change_set_id TEXT NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'replace_chapter_body',
            'update_outline_node',
            'update_character',
            'update_fact',
            'update_project',
            'create_outline_node',
            'create_character',
            'create_relationship',
            'create_world_rule',
            'create_story_event',
            'create_fact',
            'create_foreshadowing_hook',
            'create_faction',
            'create_location',
            'create_anchor',
            'create_story_asset'
        )
    ),
    position INTEGER NOT NULL DEFAULT 1 CHECK (position >= 1),
    target_id TEXT NOT NULL,
    base_revision INTEGER NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    base_fingerprint TEXT NOT NULL DEFAULT '',
    before_value TEXT NOT NULL,
    after_value TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    evidence TEXT NOT NULL DEFAULT '[]' CHECK (
        evidence = '[]' OR (substr(evidence, 1, 1) = '[' AND substr(evidence, length(evidence), 1) = ']')
    ),
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

INSERT INTO change_operations_new (
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, evidence, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, evidence, status, created_at, updated_at
FROM change_operations;

DROP TABLE change_operations;
ALTER TABLE change_operations_new RENAME TO change_operations;

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, position, id);

ALTER TABLE creation_sessions ADD COLUMN archive_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_sessions ADD COLUMN archive_idempotency_key TEXT NOT NULL DEFAULT '';
ALTER TABLE creation_sessions ADD COLUMN archive_change_set_id TEXT NOT NULL DEFAULT '';

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS creation_archive_ops_downgrade_guard;
CREATE TEMP TABLE creation_archive_ops_downgrade_guard (
    value INTEGER NOT NULL
);
INSERT INTO creation_archive_ops_downgrade_guard(value)
SELECT CASE
    WHEN EXISTS (
        SELECT 1 FROM change_operations
        WHERE kind IN ('create_faction', 'create_location', 'create_anchor', 'create_story_asset', 'update_project')
    ) THEN NULL ELSE 1 END;
DROP TABLE creation_archive_ops_downgrade_guard;

DROP INDEX IF EXISTS idx_change_operations_change_set;

CREATE TABLE change_operations_old (
    id TEXT PRIMARY KEY,
    change_set_id TEXT NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'replace_chapter_body',
            'update_outline_node',
            'update_character',
            'update_fact',
            'create_outline_node',
            'create_character',
            'create_relationship',
            'create_world_rule',
            'create_story_event',
            'create_fact',
            'create_foreshadowing_hook'
        )
    ),
    position INTEGER NOT NULL DEFAULT 1 CHECK (position >= 1),
    target_id TEXT NOT NULL,
    base_revision INTEGER NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    base_fingerprint TEXT NOT NULL DEFAULT '',
    before_value TEXT NOT NULL,
    after_value TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    evidence TEXT NOT NULL DEFAULT '[]' CHECK (
        evidence = '[]' OR (substr(evidence, 1, 1) = '[' AND substr(evidence, length(evidence), 1) = ']')
    ),
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

INSERT INTO change_operations_old (
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, evidence, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, evidence, status, created_at, updated_at
FROM change_operations;

DROP TABLE change_operations;
ALTER TABLE change_operations_old RENAME TO change_operations;

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, position, id);

ALTER TABLE creation_sessions DROP COLUMN archive_digest;
ALTER TABLE creation_sessions DROP COLUMN archive_idempotency_key;
ALTER TABLE creation_sessions DROP COLUMN archive_change_set_id;

PRAGMA foreign_keys = ON;
