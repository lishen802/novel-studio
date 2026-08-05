-- +goose Up
-- Extraction runtime: tasks/steps drive AI extraction over an applied
-- ImportJob, and change_operations learns a create_* kind family plus an
-- evidence column so extraction proposals can be reviewed against the
-- manuscript that produced them.

CREATE TABLE extraction_tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    import_job_id TEXT NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    connection_id TEXT NOT NULL CHECK (length(trim(connection_id)) >= 1),
    model TEXT NOT NULL CHECK (length(trim(model)) >= 1),
    prompt TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'recoverable')
    ),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    change_set_id TEXT REFERENCES change_sets(id) ON DELETE SET NULL,
    retry_of TEXT REFERENCES extraction_tasks(id) ON DELETE SET NULL,
    token_budget INTEGER NOT NULL DEFAULT 60000 CHECK (token_budget >= 1),
    context_char_budget INTEGER NOT NULL DEFAULT 120000 CHECK (context_char_budget >= 1),
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    CHECK (
        (status IN ('queued', 'cancelled') AND started_at IS NULL AND completed_at IS NULL)
        OR (status IN ('running', 'recoverable') AND started_at IS NOT NULL AND completed_at IS NULL)
        OR (status IN ('completed', 'failed') AND started_at IS NOT NULL AND completed_at IS NOT NULL)
    ),
    CHECK (
        error_code = '' AND error_message = ''
        OR status IN ('failed', 'recoverable')
    )
);

CREATE INDEX idx_extraction_tasks_project
    ON extraction_tasks(project_id, created_at DESC);

CREATE INDEX idx_extraction_tasks_import
    ON extraction_tasks(import_job_id);

CREATE INDEX idx_extraction_tasks_retry
    ON extraction_tasks(retry_of)
    WHERE retry_of IS NOT NULL;

CREATE TABLE extraction_steps (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES extraction_tasks(id) ON DELETE CASCADE,
    import_chapter_id TEXT NOT NULL REFERENCES import_chapters(id) ON DELETE CASCADE,
    created_chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_title TEXT NOT NULL CHECK (length(trim(chapter_title)) >= 1),
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled')
    ),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    provider_request_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    CHECK (
        (status IN ('queued', 'cancelled') AND started_at IS NULL AND completed_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND completed_at IS NULL)
        OR (status IN ('completed', 'failed') AND started_at IS NOT NULL AND completed_at IS NOT NULL)
    ),
    CHECK (
        error_code = '' AND error_message = ''
        OR status = 'failed'
    )
);

CREATE INDEX idx_extraction_steps_task
    ON extraction_steps(task_id, position);

-- Recreate change_operations so its kind CHECK admits the create_* family and
-- the table gains an evidence column. The base_fingerprint / base_revision
-- CHECKs from 00010 already accommodate create_* operations because they use
-- base_revision=0 and base_fingerprint=SHA256('{}'), which satisfies the
-- "(kind <> 'replace_chapter_body' AND base_revision=0 AND base_fingerprint <> '')"
-- branch.
DROP INDEX idx_change_operations_change_set;

ALTER TABLE change_operations RENAME TO change_operations_v13;

CREATE TABLE change_operations (
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

INSERT INTO change_operations(
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, evidence, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, '[]', status, created_at, updated_at
FROM change_operations_v13;

DROP TABLE change_operations_v13;

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, position, id);

ALTER TABLE import_jobs ADD COLUMN extraction_status TEXT NOT NULL DEFAULT 'none'
    CHECK (extraction_status IN ('none', 'running', 'completed', 'failed'));
ALTER TABLE import_jobs ADD COLUMN extraction_run_id TEXT
    REFERENCES extraction_tasks(id) ON DELETE SET NULL;

CREATE TABLE extraction_artifacts (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES extraction_tasks(id) ON DELETE CASCADE,
    step_id TEXT REFERENCES extraction_steps(id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (length(trim(kind)) >= 1),
    content TEXT NOT NULL DEFAULT '',
    sha256 TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX idx_extraction_artifacts_task
    ON extraction_artifacts(task_id, created_at, id);

-- +goose Down
-- Reversing extraction runtime drops the new tables and removes the columns
-- added to import_jobs; change_operations returns to its pre-create-kind
-- definition without an evidence column.

ALTER TABLE import_jobs DROP COLUMN extraction_run_id;
ALTER TABLE import_jobs DROP COLUMN extraction_status;

DROP INDEX idx_change_operations_change_set;

ALTER TABLE change_operations RENAME TO change_operations_v14;

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

-- Guard against downgrade that would lose create_* rows: if any create_
-- operation exists, abort the downgrade so historical extraction data is
-- not silently discarded.
CREATE TEMP TABLE extraction_change_operations_downgrade_guard (
    value INTEGER NOT NULL
);

INSERT INTO extraction_change_operations_downgrade_guard(value)
SELECT NULL
WHERE EXISTS (
    SELECT 1 FROM change_operations_v14
    WHERE kind IN (
        'create_outline_node', 'create_character', 'create_relationship',
        'create_world_rule', 'create_story_event', 'create_fact',
        'create_foreshadowing_hook'
    )
);

DROP TABLE extraction_change_operations_downgrade_guard;

INSERT INTO change_operations(
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, status, created_at, updated_at
)
SELECT
    id, change_set_id, kind, position, target_id, base_revision, base_fingerprint,
    before_value, after_value, reason, status, created_at, updated_at
FROM change_operations_v14
WHERE kind IN (
    'replace_chapter_body', 'update_outline_node', 'update_character', 'update_fact'
);

DROP TABLE change_operations_v14;

CREATE INDEX idx_change_operations_change_set
    ON change_operations(change_set_id, position, id);

DROP INDEX idx_extraction_steps_task;
DROP TABLE extraction_steps;

DROP INDEX idx_extraction_artifacts_task;
DROP TABLE extraction_artifacts;

DROP INDEX idx_extraction_tasks_retry;
DROP INDEX idx_extraction_tasks_import;
DROP INDEX idx_extraction_tasks_project;
DROP TABLE extraction_tasks;
