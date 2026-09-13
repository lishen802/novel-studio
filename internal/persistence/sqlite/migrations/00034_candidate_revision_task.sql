-- +goose NO TRANSACTION
-- +goose Up
-- R4 扩大运行任务约束：加入四类候选定向修订，保留全部旧运行列与引用。
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
            'revise_story_candidate', 'revise_draft', 'baseline_draft', 'revise_blueprint', 'verify_revision',
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

PRAGMA foreign_keys = ON;

-- +goose Down
-- 保留新任务运行，降级只读不收窄已存任务的约束。
SELECT 1;
