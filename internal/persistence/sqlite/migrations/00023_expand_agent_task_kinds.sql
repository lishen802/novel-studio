-- +goose NO TRANSACTION
-- +goose Up
-- R4 重建运行表 CHECK：纳入全部任务类型与能力，避免新产品工作流写入失败。
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_agent_runs_project_created;
DROP INDEX IF EXISTS idx_agent_runs_task_created;
DROP INDEX IF EXISTS idx_agent_runs_workflow;

CREATE TABLE agent_runs_new (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_base_revision INTEGER NOT NULL CHECK (chapter_base_revision >= 1),
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
            'revise_draft', 'baseline_draft', 'revise_blueprint', 'verify_revision',
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
    context_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (context_budget_unlimited IN (0, 1))
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

DROP TABLE agent_runs;
ALTER TABLE agent_runs_new RENAME TO agent_runs;

CREATE INDEX idx_agent_runs_project_created ON agent_runs(project_id, created_at DESC);
CREATE INDEX idx_agent_runs_task_created ON agent_runs(task_kind, created_at DESC);
CREATE INDEX idx_agent_runs_workflow ON agent_runs(workflow_id, workflow_version);

DROP INDEX IF EXISTS idx_agent_run_steps_run_position_attempt;
DROP INDEX IF EXISTS idx_agent_run_steps_run_node_attempt;

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
        capability IN (
            '', 'story_architect', 'chapter_planner', 'prose_writer', 'continuity_review',
            'content_review', 'style_review', 'bounded_revision', 'revision_verify',
            'skeleton_writer', 'slot_writer', 'integration_writer'
        )
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
    public_stage_id TEXT NOT NULL DEFAULT '' CHECK (
        public_stage_id IN ('', 'context_preparing', 'story_planning', 'chapter_directing',
            'professional_analysis', 'prose_writing', 'independent_review',
            'bounded_revision', 'candidate_ready', 'revision_verify')
    ),
    reasoning_effort TEXT NOT NULL DEFAULT '',
    output_limit_mode TEXT NOT NULL DEFAULT '',
    UNIQUE(run_id, node_id, attempt),
    UNIQUE(run_id, position, attempt),
    UNIQUE(id, run_id, attempt)
);

INSERT INTO agent_run_steps_new (
    id, run_id, role, profile_id, position, status, output,
    error_code, error_message, provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, node_id, capability, attempt,
    prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
    assembler_id, assembler_version, input_digest, output_digest, primary_artifact_id,
    quality_status, started_at, completed_at, created_at, updated_at,
    public_stage_id, reasoning_effort, output_limit_mode
)
SELECT
    id, run_id, role, profile_id, position, status, output,
    error_code, error_message, provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, node_id, capability, attempt,
    prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
    assembler_id, assembler_version, input_digest, output_digest, primary_artifact_id,
    quality_status, started_at, completed_at, created_at, updated_at,
    public_stage_id, reasoning_effort, output_limit_mode
FROM agent_run_steps;

DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_new RENAME TO agent_run_steps;

CREATE INDEX idx_agent_run_steps_run_position_attempt ON agent_run_steps(run_id, position, attempt);
CREATE INDEX idx_agent_run_steps_run_node_attempt ON agent_run_steps(run_id, node_id, attempt);

PRAGMA foreign_keys = ON;

-- +goose NO TRANSACTION
-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_agent_runs_project_created;
DROP INDEX IF EXISTS idx_agent_runs_task_created;
DROP INDEX IF EXISTS idx_agent_runs_workflow;

CREATE TABLE agent_runs_old (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_base_revision INTEGER NOT NULL CHECK (chapter_base_revision >= 1),
    prompt TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'recoverable')
    ),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    change_set_id TEXT REFERENCES change_sets(id) ON DELETE SET NULL,
    retry_of TEXT REFERENCES agent_runs_old(id) ON DELETE SET NULL,
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
        task_kind IN ('', 'story_blueprint', 'chapter_plan', 'chapter_draft', 'review_bundle', 'revise_draft', 'baseline_draft')
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
    context_budget_unlimited INTEGER NOT NULL DEFAULT 0 CHECK (context_budget_unlimited IN (0, 1))
);

INSERT INTO agent_runs_old (
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

DROP TABLE agent_runs;
ALTER TABLE agent_runs_old RENAME TO agent_runs;

CREATE INDEX idx_agent_runs_project_created ON agent_runs(project_id, created_at DESC);
CREATE INDEX idx_agent_runs_task_created ON agent_runs(task_kind, created_at DESC);
CREATE INDEX idx_agent_runs_workflow ON agent_runs(workflow_id, workflow_version);

DROP INDEX IF EXISTS idx_agent_run_steps_run_position_attempt;
DROP INDEX IF EXISTS idx_agent_run_steps_run_node_attempt;

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
    public_stage_id TEXT NOT NULL DEFAULT '' CHECK (
        public_stage_id IN ('', 'context_preparing', 'story_planning', 'chapter_directing',
            'professional_analysis', 'prose_writing', 'independent_review',
            'bounded_revision', 'candidate_ready')
    ),
    reasoning_effort TEXT NOT NULL DEFAULT '',
    output_limit_mode TEXT NOT NULL DEFAULT '',
    UNIQUE(run_id, node_id, attempt),
    UNIQUE(run_id, position, attempt),
    UNIQUE(id, run_id, attempt)
);

INSERT INTO agent_run_steps_old (
    id, run_id, role, profile_id, position, status, output,
    error_code, error_message, provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, node_id, capability, attempt,
    prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
    assembler_id, assembler_version, input_digest, output_digest, primary_artifact_id,
    quality_status, started_at, completed_at, created_at, updated_at,
    public_stage_id, reasoning_effort, output_limit_mode
)
SELECT
    id, run_id, role, profile_id, position, status, output,
    error_code, error_message, provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature, node_id, capability, attempt,
    prompt_id, prompt_version, prompt_hash, schema_id, schema_version, schema_hash,
    assembler_id, assembler_version, input_digest, output_digest, primary_artifact_id,
    quality_status, started_at, completed_at, created_at, updated_at,
    public_stage_id, reasoning_effort, output_limit_mode
FROM agent_run_steps;

DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_old RENAME TO agent_run_steps;

CREATE INDEX idx_agent_run_steps_run_position_attempt ON agent_run_steps(run_id, position, attempt);
CREATE INDEX idx_agent_run_steps_run_node_attempt ON agent_run_steps(run_id, node_id, attempt);

PRAGMA foreign_keys = ON;
