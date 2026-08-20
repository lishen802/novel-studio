-- +goose NO TRANSACTION
-- +goose Up
-- R3 Agent Kernel 持久化：旧运行使用空版本字段表示 legacy，不补造不存在的
-- Prompt、Schema 或工作流哈希；可确定的 node_id 和 attempt 从原三角色步骤回填。
PRAGMA foreign_keys = OFF;

ALTER TABLE agent_runs ADD COLUMN task_kind TEXT NOT NULL DEFAULT '' CHECK (
    task_kind IN ('', 'story_blueprint', 'chapter_plan', 'chapter_draft', 'review_bundle', 'revise_draft', 'baseline_draft')
);
ALTER TABLE agent_runs ADD COLUMN workflow_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN workflow_version INTEGER NOT NULL DEFAULT 0 CHECK (workflow_version >= 0);
ALTER TABLE agent_runs ADD COLUMN workflow_hash TEXT NOT NULL DEFAULT '' CHECK (
    workflow_hash = '' OR (length(workflow_hash) = 64 AND workflow_hash NOT GLOB '*[^0-9a-f]*')
);
ALTER TABLE agent_runs ADD COLUMN canonical_head TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN input_digest TEXT NOT NULL DEFAULT '' CHECK (
    input_digest = '' OR (length(input_digest) = 64 AND input_digest NOT GLOB '*[^0-9a-f]*')
);
ALTER TABLE agent_runs ADD COLUMN quality_status TEXT NOT NULL DEFAULT '' CHECK (
    quality_status IN ('', 'passed', 'needs_review', 'blocked')
);
ALTER TABLE agent_runs ADD COLUMN budget_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (budget_policy_version >= 0);
ALTER TABLE agent_runs ADD COLUMN quality_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (quality_policy_version >= 0);
ALTER TABLE agent_runs ADD COLUMN max_revision_attempts INTEGER NOT NULL DEFAULT 0 CHECK (max_revision_attempts >= 0);

CREATE TABLE agent_run_steps_new (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('planner', 'writer', 'continuity_reviewer')),
    profile_id TEXT,
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    creative_profile_id TEXT NOT NULL DEFAULT '',
    creative_profile_version INTEGER NOT NULL DEFAULT 0 CHECK (creative_profile_version >= 0),
    provider_connection_id TEXT NOT NULL DEFAULT '',
    provider_name TEXT NOT NULL DEFAULT '',
    provider_kind TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    credential_ref TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    max_output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (max_output_tokens >= 0),
    temperature REAL NOT NULL DEFAULT 0 CHECK (temperature >= 0 AND temperature <= 2),
    node_id TEXT NOT NULL,
    capability TEXT NOT NULL DEFAULT '' CHECK (
        capability IN ('', 'story_architect', 'chapter_planner', 'prose_writer', 'continuity_review', 'content_review', 'style_review', 'bounded_revision')
    ),
    attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
    prompt_id TEXT NOT NULL DEFAULT '',
    prompt_version INTEGER NOT NULL DEFAULT 0 CHECK (prompt_version >= 0),
    prompt_hash TEXT NOT NULL DEFAULT '' CHECK (
        prompt_hash = '' OR (length(prompt_hash) = 64 AND prompt_hash NOT GLOB '*[^0-9a-f]*')
    ),
    schema_id TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 0 CHECK (schema_version >= 0),
    schema_hash TEXT NOT NULL DEFAULT '' CHECK (
        schema_hash = '' OR (length(schema_hash) = 64 AND schema_hash NOT GLOB '*[^0-9a-f]*')
    ),
    assembler_id TEXT NOT NULL DEFAULT '',
    assembler_version INTEGER NOT NULL DEFAULT 0 CHECK (assembler_version >= 0),
    input_digest TEXT NOT NULL DEFAULT '' CHECK (
        input_digest = '' OR (length(input_digest) = 64 AND input_digest NOT GLOB '*[^0-9a-f]*')
    ),
    output_digest TEXT NOT NULL DEFAULT '' CHECK (
        output_digest = '' OR (length(output_digest) = 64 AND output_digest NOT GLOB '*[^0-9a-f]*')
    ),
    primary_artifact_id TEXT REFERENCES agent_artifacts(id) ON DELETE SET NULL,
    quality_status TEXT NOT NULL DEFAULT '' CHECK (
        quality_status IN ('', 'passed', 'needs_review', 'blocked')
    ),
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(run_id, node_id, attempt),
    UNIQUE(run_id, position, attempt),
    UNIQUE(id, run_id, attempt)
);

INSERT INTO agent_run_steps_new (
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, node_id, capability, attempt,
    prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
    assembler_id, assembler_version, input_digest, output_digest, primary_artifact_id,
    quality_status, started_at, completed_at, created_at, updated_at
)
SELECT
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, role, '', 1,
    '', 0, '', '', 0, '', '', 0, '', '', NULL, '',
    started_at, completed_at, created_at, updated_at
FROM agent_run_steps;

CREATE TABLE agent_artifacts_new (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_id TEXT REFERENCES agent_run_steps(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN (
        'context', 'task_input', 'plan', 'draft', 'review', 'revision',
        'structured_output', 'lint_report', 'quality_report', 'format_repair'
    )),
    content TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TEXT NOT NULL
);

INSERT INTO agent_artifacts_new(id, run_id, step_id, kind, content, sha256, created_at)
SELECT id, run_id, step_id, kind, content, sha256, created_at FROM agent_artifacts;

DROP TABLE agent_artifacts;
DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_new RENAME TO agent_run_steps;
ALTER TABLE agent_artifacts_new RENAME TO agent_artifacts;

CREATE TABLE context_manifests (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL CHECK (version >= 1),
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_id TEXT NOT NULL,
    attempt INTEGER NOT NULL CHECK (attempt >= 1),
    canonical_head TEXT NOT NULL DEFAULT '',
    assembler_id TEXT NOT NULL,
    assembler_version INTEGER NOT NULL CHECK (assembler_version >= 1),
    character_budget INTEGER NOT NULL CHECK (character_budget >= 1),
    estimated_token_budget INTEGER NOT NULL CHECK (estimated_token_budget >= 0),
    used_chars INTEGER NOT NULL CHECK (used_chars >= 0 AND used_chars <= character_budget),
    truncated INTEGER NOT NULL CHECK (truncated IN (0, 1)),
    assembled_content_hash TEXT NOT NULL CHECK (
        length(assembled_content_hash) = 64 AND assembled_content_hash NOT GLOB '*[^0-9a-f]*'
    ),
    created_at TEXT NOT NULL,
    UNIQUE(run_id, step_id, attempt),
    FOREIGN KEY(step_id, run_id, attempt)
        REFERENCES agent_run_steps(id, run_id, attempt) ON DELETE CASCADE
);

CREATE TABLE context_manifest_items (
    manifest_id TEXT NOT NULL REFERENCES context_manifests(id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL CHECK (source_kind IN (
        'harness_spec', 'author_instruction', 'chapter_revision', 'story_asset', 'upstream_artifact', 'anchor'
    )),
    source_id TEXT NOT NULL,
    source_version TEXT NOT NULL,
    range_start INTEGER,
    range_end INTEGER,
    content_hash TEXT NOT NULL CHECK (
        length(content_hash) = 64 AND content_hash NOT GLOB '*[^0-9a-f]*'
    ),
    position INTEGER NOT NULL CHECK (position >= 1),
    priority INTEGER NOT NULL CHECK (priority >= 0 AND priority <= 1000),
    selection_reason TEXT NOT NULL,
    provisional INTEGER NOT NULL CHECK (provisional IN (0, 1)),
    included INTEGER NOT NULL CHECK (included IN (0, 1)),
    omission_reason TEXT NOT NULL DEFAULT '',
    CHECK (
        (range_start IS NULL AND range_end IS NULL) OR
        (range_start IS NOT NULL AND range_start >= 0 AND range_end IS NOT NULL AND range_end > range_start)
    ),
    CHECK (
        (included = 1 AND omission_reason = '') OR
        (included = 0 AND length(trim(omission_reason)) > 0)
    ),
    PRIMARY KEY(manifest_id, position)
);

CREATE TABLE agent_review_findings (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_id TEXT NOT NULL,
    attempt INTEGER NOT NULL CHECK (attempt >= 1),
    category TEXT NOT NULL CHECK (category IN (
        'anchor', 'continuity', 'knowledge_boundary', 'causality', 'motivation', 'pacing', 'hook',
        'information_density', 'world_rule', 'foreshadowing', 'repetition', 'style', 'format', 'originality_risk'
    )),
    severity TEXT NOT NULL CHECK (severity IN ('blocker', 'major', 'minor', 'note')),
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved', 'retained', 'cannot_resolve')),
    source TEXT NOT NULL CHECK (source IN ('model_review', 'deterministic_lint', 'author')),
    message TEXT NOT NULL,
    suggested_action TEXT NOT NULL DEFAULT '',
    quoted_evidence TEXT NOT NULL DEFAULT '',
    scope_start INTEGER,
    scope_end INTEGER,
    constraint_ref TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (scope_start IS NOT NULL AND scope_start >= 0 AND scope_end IS NOT NULL AND scope_end > scope_start AND length(trim(quoted_evidence)) > 0) OR
        (scope_start IS NULL AND scope_end IS NULL AND length(trim(constraint_ref)) > 0)
    ),
    FOREIGN KEY(step_id, run_id, attempt)
        REFERENCES agent_run_steps(id, run_id, attempt) ON DELETE CASCADE
);

CREATE INDEX idx_agent_runs_task_created ON agent_runs(task_kind, created_at DESC);
CREATE INDEX idx_agent_runs_workflow ON agent_runs(workflow_id, workflow_version);
CREATE INDEX idx_agent_run_steps_run_position_attempt ON agent_run_steps(run_id, position, attempt);
CREATE INDEX idx_agent_run_steps_run_node_attempt ON agent_run_steps(run_id, node_id, attempt);
CREATE INDEX idx_agent_artifacts_run_created ON agent_artifacts(run_id, created_at);
CREATE INDEX idx_context_manifests_run_step ON context_manifests(run_id, step_id, attempt);
CREATE INDEX idx_context_manifest_items_source ON context_manifest_items(source_kind, source_id, source_version);
CREATE INDEX idx_agent_review_findings_run_step ON agent_review_findings(run_id, step_id, attempt);
CREATE INDEX idx_agent_review_findings_category_status ON agent_review_findings(category, status);

PRAGMA foreign_keys = ON;

-- +goose NO TRANSACTION
-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX idx_agent_review_findings_category_status;
DROP INDEX idx_agent_review_findings_run_step;
DROP INDEX idx_context_manifest_items_source;
DROP INDEX idx_context_manifests_run_step;
DROP TABLE agent_review_findings;
DROP TABLE context_manifest_items;
DROP TABLE context_manifests;

CREATE TABLE agent_run_steps_old (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('planner', 'writer', 'continuity_reviewer')),
    profile_id TEXT,
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    creative_profile_id TEXT NOT NULL DEFAULT '',
    creative_profile_version INTEGER NOT NULL DEFAULT 0,
    provider_connection_id TEXT NOT NULL DEFAULT '',
    provider_name TEXT NOT NULL DEFAULT '',
    provider_kind TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    credential_ref TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    max_output_tokens INTEGER NOT NULL DEFAULT 0,
    temperature REAL NOT NULL DEFAULT 0,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(run_id, position)
);

INSERT INTO agent_run_steps_old (
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, started_at, completed_at, created_at, updated_at
)
SELECT
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, started_at, completed_at, created_at, updated_at
FROM agent_run_steps
WHERE attempt = 1;

CREATE TABLE agent_artifacts_old (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    step_id TEXT REFERENCES agent_run_steps(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('context', 'plan', 'draft', 'review')),
    content TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TEXT NOT NULL
);

INSERT INTO agent_artifacts_old(id, run_id, step_id, kind, content, sha256, created_at)
SELECT id, run_id, step_id, kind, content, sha256, created_at
FROM agent_artifacts
WHERE kind IN ('context', 'plan', 'draft', 'review');

DROP INDEX idx_agent_run_steps_run_node_attempt;
DROP INDEX idx_agent_run_steps_run_position_attempt;
DROP INDEX idx_agent_artifacts_run_created;
DROP TABLE agent_artifacts;
DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_old RENAME TO agent_run_steps;
ALTER TABLE agent_artifacts_old RENAME TO agent_artifacts;
CREATE INDEX idx_agent_run_steps_run_position ON agent_run_steps(run_id, position);
CREATE INDEX idx_agent_artifacts_run_created ON agent_artifacts(run_id, created_at);

DROP INDEX idx_agent_runs_workflow;
DROP INDEX idx_agent_runs_task_created;
ALTER TABLE agent_runs DROP COLUMN max_revision_attempts;
ALTER TABLE agent_runs DROP COLUMN quality_policy_version;
ALTER TABLE agent_runs DROP COLUMN budget_policy_version;
ALTER TABLE agent_runs DROP COLUMN quality_status;
ALTER TABLE agent_runs DROP COLUMN input_digest;
ALTER TABLE agent_runs DROP COLUMN canonical_head;
ALTER TABLE agent_runs DROP COLUMN workflow_hash;
ALTER TABLE agent_runs DROP COLUMN workflow_version;
ALTER TABLE agent_runs DROP COLUMN workflow_id;
ALTER TABLE agent_runs DROP COLUMN task_kind;

PRAGMA foreign_keys = ON;
