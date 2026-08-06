-- v9 legacy project fixture seed data.
--
-- All IDs, titles, and content are synthetic and free of copyright risk.
-- The fixture exercises the v9 -> v14 migration path by providing rows that
-- must survive structured change-set migration (v10), agent runtime controls
-- (v11-v12), manuscript import (v13), and extraction runtime (v14).
--
-- Times are fixed UTC RFC3339Nano for deterministic generation. Do not use
-- CURRENT_TIMESTAMP or random values here; the fixture must produce identical
-- business rows across runs and platforms.

INSERT INTO projects(id, title, genre, summary, created_at, updated_at)
VALUES (
    'legacy-project-1',
    '雾港异闻录',
    '奇幻',
    '旧版数据库迁移测试用合成稿件',
    '2024-01-01T00:00:00Z',
    '2024-01-02T00:00:00Z'
);

INSERT INTO chapters(
    id, project_id, title, position, body, revision,
    revision_source, revision_summary, created_at, updated_at
) VALUES (
    'legacy-chapter-1',
    'legacy-project-1',
    '第一章 雾港',
    1,
    '雾港的清晨，林澈握紧了那把旧铜钥匙。',
    2,
    'author',
    '扩写镜头',
    '2024-01-01T00:00:00Z',
    '2024-01-02T00:00:00Z'
);

INSERT INTO chapter_revisions(
    id, chapter_id, revision, title, body, source, summary, created_at
) VALUES (
    1,
    'legacy-chapter-1',
    1,
    '第一章 雾港',
    '雾港的清晨，林澈走在湿滑的石板路上。',
    'initial',
    '',
    '2024-01-01T00:00:00Z'
);

INSERT INTO drafts(chapter_id, body, base_revision, updated_at)
VALUES (
    'legacy-chapter-1',
    '林澈打开旧铜钥匙下的信件。',
    2,
    '2024-01-02T00:00:00Z'
);

INSERT INTO characters(
    id, project_id, name, aliases_json, role, summary, motivation, fear,
    secret, arc, status, created_at, updated_at, deleted_at
) VALUES (
    'legacy-character-1',
    'legacy-project-1',
    '林澈',
    '[]',
    'protagonist',
    '旧铜钥匙的持有者',
    '解开雾港潮汐的秘密',
    '被潮汐吞没',
    '钥匙实为雾港的封印',
    '从逃亡者到守护者',
    'active',
    '2024-01-01T00:00:00Z',
    '2024-01-02T00:00:00Z',
    NULL
);

INSERT INTO world_rules(
    id, project_id, name, description, scope_type, scope_id, status,
    created_at, updated_at, deleted_at
) VALUES (
    'legacy-world-rule-1',
    'legacy-project-1',
    '雾港潮汐律',
    '每逢朔月，雾港会被浓雾吞没。',
    'project',
    NULL,
    'active',
    '2024-01-01T00:00:00Z',
    '2024-01-02T00:00:00Z',
    NULL
);

INSERT INTO canon_facts(
    id, project_id, fact_text, category, origin, status, confidence,
    valid_from_event_id, valid_until_event_id, created_by, confirmed_by,
    created_at, updated_at, deleted_at
) VALUES (
    'legacy-fact-1',
    'legacy-project-1',
    '林澈持有旧铜钥匙。',
    'character',
    'author',
    'author_confirmed',
    1.0,
    NULL,
    NULL,
    'author',
    'author',
    '2024-01-01T00:00:00Z',
    '2024-01-02T00:00:00Z',
    NULL
);

INSERT INTO change_sets(
    id, project_id, title, summary, created_by, status, created_at, updated_at
) VALUES (
    'legacy-change-set-1',
    'legacy-project-1',
    '旧版本正文候选',
    '',
    'agent:writer',
    'pending',
    '2024-01-02T00:00:00Z',
    '2024-01-02T00:00:00Z'
);

INSERT INTO change_operations(
    id, change_set_id, kind, target_id, base_revision,
    before_value, after_value, reason, status, created_at, updated_at
) VALUES (
    'legacy-change-op-1',
    'legacy-change-set-1',
    'replace_chapter_body',
    'legacy-chapter-1',
    2,
    '雾港的清晨，林澈握紧了那把旧铜钥匙。',
    '雾港的清晨，林澈紧握那把旧铜钥匙，听见远处的钟声。',
    '扩写镜头细节',
    'pending',
    '2024-01-02T00:00:00Z',
    '2024-01-02T00:00:00Z'
);
