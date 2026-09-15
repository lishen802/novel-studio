-- +goose NO TRANSACTION
-- +goose Up
-- R4 共创讨论授权：新增授权表、消息运行字段，并扩大 Action 与项目级任务约束。

CREATE TABLE assistant_authorizations (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id TEXT NOT NULL,
    action_id TEXT NOT NULL REFERENCES assistant_actions(id) ON DELETE CASCADE,
    candidate_id TEXT NOT NULL,
    candidate_version INTEGER NOT NULL CHECK (candidate_version >= 1),
    candidate_sha256 TEXT NOT NULL,
    stage TEXT NOT NULL DEFAULT '',
    entity_ref TEXT NOT NULL DEFAULT '',
    field_paths_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(field_paths_json)),
    profile_version_id TEXT NOT NULL,
    provider_id TEXT NOT NULL DEFAULT '',
    provider_name TEXT NOT NULL,
    model TEXT NOT NULL,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    max_calls INTEGER NOT NULL CHECK (max_calls >= 1),
    max_total_tokens INTEGER NOT NULL CHECK (max_total_tokens >= 1),
    max_turn_tokens INTEGER NOT NULL CHECK (max_turn_tokens >= 1 AND max_turn_tokens <= max_total_tokens),
    reserved_calls INTEGER NOT NULL DEFAULT 0 CHECK (reserved_calls >= 0),
    reserved_tokens INTEGER NOT NULL DEFAULT 0 CHECK (reserved_tokens >= 0),
    used_calls INTEGER NOT NULL DEFAULT 0 CHECK (used_calls >= 0),
    used_tokens INTEGER NOT NULL DEFAULT 0 CHECK (used_tokens >= 0),
    state TEXT NOT NULL CHECK (
        state IN ('pending_confirmation', 'active', 'exhausted', 'expired', 'revoked', 'stale')
    ),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(action_id)
);

CREATE INDEX idx_assistant_authorizations_session_state
    ON assistant_authorizations(session_id, state, expires_at);
CREATE INDEX idx_assistant_authorizations_session_candidate
    ON assistant_authorizations(session_id, candidate_id, created_at);

ALTER TABLE assistant_messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'revision_request';
ALTER TABLE assistant_messages ADD COLUMN status TEXT NOT NULL DEFAULT 'answered';
ALTER TABLE assistant_messages ADD COLUMN reply_to_id TEXT NOT NULL DEFAULT '';
ALTER TABLE assistant_messages ADD COLUMN authorization_id TEXT REFERENCES assistant_authorizations(id) ON DELETE SET NULL;
ALTER TABLE assistant_messages ADD COLUMN run_id TEXT NOT NULL DEFAULT '';
ALTER TABLE assistant_messages ADD COLUMN operation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE assistant_messages ADD COLUMN error_code TEXT NOT NULL DEFAULT '';
ALTER TABLE assistant_messages ADD COLUMN suggestions_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE assistant_messages ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

UPDATE assistant_messages SET updated_at = created_at WHERE updated_at = '';

CREATE INDEX idx_assistant_messages_authorization_created
    ON assistant_messages(authorization_id, created_at);
CREATE INDEX idx_assistant_messages_run
    ON assistant_messages(run_id);

PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_assistant_actions_session_state;

CREATE TABLE assistant_actions_new (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_id TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (
        operation IN ('regenerate_candidate', 'revise_fields', 'authorize_discussion', 'combine_directions', 'edit_locks')
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
    result_refs_json TEXT NOT NULL DEFAULT '[]',
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(session_id, idempotency_key)
);

INSERT INTO assistant_actions_new (
    id, session_id, project_id, branch_id, operation, risk, target_refs_json,
    base_versions_json, arguments, arguments_hash, idempotency_key, estimated_cost,
    state, error_code, result_refs_json, expires_at, created_at, updated_at
)
SELECT
    id, session_id, project_id, branch_id, operation, risk, target_refs_json,
    base_versions_json, arguments, arguments_hash, idempotency_key, estimated_cost,
    state, error_code, result_refs_json, expires_at, created_at, updated_at
FROM assistant_actions;

CREATE TABLE assistant_actions_copy_check (
    expected INTEGER NOT NULL,
    actual INTEGER NOT NULL,
    CHECK (expected = actual)
);
INSERT INTO assistant_actions_copy_check(expected, actual)
SELECT (SELECT COUNT(*) FROM assistant_actions), (SELECT COUNT(*) FROM assistant_actions_new);
DROP TABLE assistant_actions_copy_check;

DROP TABLE assistant_actions;
ALTER TABLE assistant_actions_new RENAME TO assistant_actions;

CREATE INDEX idx_assistant_actions_session_state
    ON assistant_actions(session_id, state, created_at);

DROP INDEX IF EXISTS idx_agent_runs_project_created;
DROP INDEX IF EXISTS idx_agent_runs_task_created;
DROP INDEX IF EXISTS idx_agent_runs_workflow;

CREATE TABLE agent_runs_new (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_base_revision INTEGER NOT NULL CHECK (chapter_base_revision >= 0),
    prompt TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'recoverable')
    ),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    change_set_id TEXT REFERENCES change_sets(id) ON DELETE SET NULL,
    retry_of TEXT REFERENCES agent_runs_new(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    completed_at TEXT,
    token_budget INTEGER NOT NULL DEFAULT 30000 CHECK (token_budget >= 1),
    context_char_budget INTEGER NOT NULL DEFAULT 120000 CHECK (context_char_budget >= 1),
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    context_chars INTEGER NOT NULL DEFAULT 0 CHECK (context_chars >= 0),
    context_truncated INTEGER NOT NULL DEFAULT 0 CHECK (context_truncated IN (0, 1)),
    task_kind TEXT NOT NULL DEFAULT '' CHECK (
        task_kind IN (
            '', 'story_blueprint', 'chapter_plan', 'chapter_draft', 'review_bundle',
            'revise_story_candidate', 'discuss_story_candidate', 'revise_draft', 'baseline_draft', 'revise_blueprint', 'verify_revision',
            'chapter_skeleton', 'fill_slot', 'story_directions', 'story_cast', 'story_roadmap'
        )
    ),
    workflow_id TEXT NOT NULL DEFAULT '',
    workflow_version INTEGER NOT NULL DEFAULT 0 CHECK (workflow_version >= 0),
    workflow_hash TEXT NOT NULL DEFAULT '' CHECK (
        workflow_hash = '' OR (length(workflow_hash) = 64 AND workflow_hash NOT GLOB '*[^0-9a-f]*')
    ),
    canonical_head TEXT NOT NULL DEFAULT '',
    input_digest TEXT NOT NULL DEFAULT '' CHECK (
        input_digest = '' OR (length(input_digest) = 64 AND input_digest NOT GLOB '*[^0-9a-f]*')
    ),
    quality_status TEXT NOT NULL DEFAULT '' CHECK (
        quality_status IN ('', 'passed', 'needs_review', 'blocked')
    ),
    budget_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (budget_policy_version >= 0),
    quality_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (quality_policy_version >= 0),
    max_revision_attempts INTEGER NOT NULL DEFAULT 0 CHECK (max_revision_attempts >= 0),
    token_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (token_budget_unlimited IN (0, 1)),
    context_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (context_budget_unlimited IN (0, 1)),
    CHECK ((chapter_id IS NULL AND chapter_base_revision = 0 AND task_kind IN ('revise_story_candidate', 'discuss_story_candidate')) OR (chapter_id IS NOT NULL AND chapter_base_revision >= 1))
);

INSERT INTO agent_runs_new (
    id, project_id, chapter_id, chapter_base_revision, prompt, status,
    error_code, error_message, change_set_id, retry_of, created_at, updated_at,
    started_at, completed_at, token_budget, context_char_budget,
    input_tokens, output_tokens, context_chars, context_truncated,
    task_kind, workflow_id, workflow_version, workflow_hash, canonical_head,
    input_digest, quality_status, budget_policy_version, quality_policy_version,
    max_revision_attempts, token_budget_unlimited, context_budget_unlimited
)
SELECT
    id, project_id, chapter_id, chapter_base_revision, prompt, status,
    error_code, error_message, change_set_id, retry_of, created_at, updated_at,
    started_at, completed_at, token_budget, context_char_budget,
    input_tokens, output_tokens, context_chars, context_truncated,
    task_kind, workflow_id, workflow_version, workflow_hash, canonical_head,
    input_digest, quality_status, budget_policy_version, quality_policy_version,
    max_revision_attempts, token_budget_unlimited, context_budget_unlimited
FROM agent_runs;

CREATE TABLE agent_runs_copy_check (
    expected INTEGER NOT NULL,
    actual INTEGER NOT NULL,
    CHECK (expected = actual)
);
INSERT INTO agent_runs_copy_check(expected, actual)
SELECT (SELECT COUNT(*) FROM agent_runs), (SELECT COUNT(*) FROM agent_runs_new);
DROP TABLE agent_runs_copy_check;

DROP TABLE agent_runs;
ALTER TABLE agent_runs_new RENAME TO agent_runs;

CREATE INDEX idx_agent_runs_project_created ON agent_runs(project_id, created_at DESC);
CREATE INDEX idx_agent_runs_task_created ON agent_runs(task_kind, created_at DESC);
CREATE INDEX idx_agent_runs_workflow ON agent_runs(workflow_id, workflow_version);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;
DROP INDEX IF EXISTS idx_assistant_messages_authorization_created;
DROP INDEX IF EXISTS idx_assistant_messages_run;
DROP INDEX IF EXISTS idx_assistant_authorizations_session_state;
DROP INDEX IF EXISTS idx_assistant_authorizations_session_candidate;
DROP TABLE IF EXISTS assistant_authorizations;
PRAGMA foreign_keys = ON;
