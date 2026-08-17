-- +goose NO TRANSACTION
-- +goose Up
-- R2.1 运行冻结快照：为 agent_run_steps 增加连接、模型与生成参数冻结字段，
-- 并把 profile_id 放宽为可空。旧任务尽量从 agent_profiles 与 provider_connections
-- 回填可推导的连接与模型信息；creative_profile_id/version 无法回填（旧任务未锁定
-- 方案版本），保持空值。该迁移通过重建表完成列约束变更，全程关闭外键校验。
PRAGMA foreign_keys = OFF;

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

INSERT INTO agent_run_steps_new (
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    creative_profile_id, creative_profile_version, provider_connection_id,
    provider_name, provider_kind, base_url, credential_ref, model,
    max_output_tokens, temperature,
    started_at, completed_at, created_at, updated_at
)
SELECT
    s.id, s.run_id, s.role, s.profile_id, s.position, s.status, s.output,
    s.error_code, s.error_message,
    s.provider_request_id, s.input_tokens, s.output_tokens,
    '', 0,
    COALESCE(p.connection_id, ''),
    COALESCE(c.name, ''), COALESCE(c.kind, ''), COALESCE(c.base_url, ''),
    COALESCE(c.credential_ref, ''), COALESCE(p.model, ''),
    COALESCE(p.max_output_tokens, 0), COALESCE(p.temperature, 0),
    s.started_at, s.completed_at, s.created_at, s.updated_at
FROM agent_run_steps s
LEFT JOIN agent_profiles p ON p.id = s.profile_id
LEFT JOIN provider_connections c ON c.id = p.connection_id;

DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_new RENAME TO agent_run_steps;
CREATE INDEX idx_agent_run_steps_run_position ON agent_run_steps(run_id, position);

PRAGMA foreign_keys = ON;

-- +goose NO TRANSACTION
-- +goose Down
-- 还原为 R2.1 之前的表结构。冻结字段属于 R2.1 新运行，旧结构无法表达，因此
-- 只保留仍带项目级智能体档案的旧任务；新运行会随回退丢失（生产仅前进迁移）。
PRAGMA foreign_keys = OFF;

CREATE TABLE agent_run_steps_old (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('planner', 'writer', 'continuity_reviewer')),
    profile_id TEXT NOT NULL REFERENCES agent_profiles(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    output TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(run_id, position)
);

INSERT INTO agent_run_steps_old (
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    started_at, completed_at, created_at, updated_at
)
SELECT
    id, run_id, role, profile_id, position, status, output, error_code, error_message,
    provider_request_id, input_tokens, output_tokens,
    started_at, completed_at, created_at, updated_at
FROM agent_run_steps
WHERE profile_id IS NOT NULL AND profile_id != '';

DROP TABLE agent_run_steps;
ALTER TABLE agent_run_steps_old RENAME TO agent_run_steps;
CREATE INDEX idx_agent_run_steps_run_position ON agent_run_steps(run_id, position);

PRAGMA foreign_keys = ON;
