-- +goose Up
CREATE TABLE creation_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id TEXT NOT NULL,
    stage TEXT NOT NULL CHECK (
        stage IN ('intent', 'directions', 'proposal', 'people_world', 'roadmap', 'archive_confirm')
    ),
    advance_mode TEXT NOT NULL CHECK (advance_mode IN ('step_review', 'auto_advance')),
    intent_text TEXT NOT NULL DEFAULT '',
    selected_direction_candidate_id TEXT,
    blocking_reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id)
);

CREATE TABLE creation_candidates (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES creation_sessions(id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (
        stage IN ('intent', 'directions', 'proposal', 'people_world', 'roadmap', 'archive_confirm')
    ),
    kind TEXT NOT NULL CHECK (
        kind IN ('direction_set', 'direction', 'proposal', 'people_world', 'roadmap')
    ),
    version INTEGER NOT NULL CHECK (version >= 1),
    parent_candidate_id TEXT REFERENCES creation_candidates(id) ON DELETE RESTRICT,
    supersedes_candidate_id TEXT REFERENCES creation_candidates(id) ON DELETE RESTRICT,
    run_id TEXT,
    artifact_id TEXT,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    payload_schema_id TEXT NOT NULL,
    payload_schema_version INTEGER NOT NULL CHECK (payload_schema_version >= 1),
    content_sha256 TEXT NOT NULL CHECK (
        length(content_sha256) = 64 AND content_sha256 NOT GLOB '*[^0-9a-f]*'
    ),
    lock_map_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(lock_map_json)),
    status TEXT NOT NULL CHECK (
        status IN ('draft', 'selected', 'confirmed', 'superseded', 'stale', 'discarded')
    ),
    origin TEXT NOT NULL CHECK (origin IN ('model', 'author')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_creation_candidates_session_stage
    ON creation_candidates(session_id, stage, version DESC, id);

CREATE TABLE story_assets (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (
        kind IN (
            'proposal', 'character', 'relationship', 'faction', 'location',
            'world_rule', 'item', 'outline_node', 'hard_anchor', 'soft_anchor',
            'forbidden', 'story_event'
        )
    ),
    status TEXT NOT NULL CHECK (status IN ('canonical', 'stale', 'archived')),
    current_revision INTEGER NOT NULL CHECK (current_revision >= 1),
    origin TEXT NOT NULL CHECK (origin IN ('author', 'model', 'import')),
    source_candidate_id TEXT,
    source_run_id TEXT,
    source_artifact_id TEXT,
    typed_table TEXT NOT NULL DEFAULT '',
    typed_id TEXT NOT NULL DEFAULT '',
    locked INTEGER NOT NULL DEFAULT 0 CHECK (locked IN (0, 1)),
    stale INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
    stale_reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_story_assets_project_kind
    ON story_assets(project_id, kind, status);

CREATE TABLE story_asset_revisions (
    asset_id TEXT NOT NULL REFERENCES story_assets(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    content_sha256 TEXT NOT NULL CHECK (
        length(content_sha256) = 64 AND content_sha256 NOT GLOB '*[^0-9a-f]*'
    ),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (asset_id, revision)
);

CREATE TABLE story_anchors (
    id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES story_assets(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    strength TEXT NOT NULL CHECK (strength IN ('hard', 'soft', 'forbidden')),
    scope_kind TEXT NOT NULL CHECK (scope_kind IN ('proposal', 'roadmap_node', 'chapter_range')),
    scope_id TEXT NOT NULL DEFAULT '',
    statement TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_story_anchors_project_strength
    ON story_anchors(project_id, strength);

CREATE TABLE assistant_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('creation')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, branch_id, kind)
);

CREATE TABLE assistant_actions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (
        operation IN ('regenerate_candidate', 'revise_fields', 'combine_directions', 'edit_locks')
    ),
    risk TEXT NOT NULL CHECK (risk IN ('read', 'low', 'paid', 'write')),
    target_refs_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(target_refs_json)),
    base_versions_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(base_versions_json)),
    arguments TEXT NOT NULL DEFAULT '{}',
    arguments_hash TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    estimated_cost TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (
        state IN (
            'pending_confirmation', 'executing', 'succeeded', 'rejected',
            'expired', 'conflict', 'failed', 'outcome_unknown'
        )
    ),
    error_code TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(session_id, idempotency_key)
);

CREATE INDEX idx_assistant_actions_session_state
    ON assistant_actions(session_id, state, created_at);

-- +goose Down
DROP INDEX idx_assistant_actions_session_state;
DROP TABLE assistant_actions;
DROP TABLE assistant_sessions;
DROP INDEX idx_story_anchors_project_strength;
DROP TABLE story_anchors;
DROP TABLE story_asset_revisions;
DROP INDEX idx_story_assets_project_kind;
DROP TABLE story_assets;
DROP INDEX idx_creation_candidates_session_stage;
DROP TABLE creation_candidates;
DROP TABLE creation_sessions;
