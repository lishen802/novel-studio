-- +goose Up
-- 整组规划再次入档的独立回执；各资产仍通过 story_constraint_changes 按章生效。
CREATE TABLE planning_rearchives (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 branch_id TEXT NOT NULL,
 chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE RESTRICT,
 position INTEGER NOT NULL CHECK(position >= 1),
 reason TEXT NOT NULL,
 digest TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
 created_at TEXT NOT NULL,
 UNIQUE(project_id,branch_id,idempotency_key)
);

-- +goose Down
CREATE TEMP TABLE planning_rearchive_downgrade_guard(value INTEGER NOT NULL);
INSERT INTO planning_rearchive_downgrade_guard SELECT CASE WHEN EXISTS(SELECT 1 FROM planning_rearchives) THEN NULL ELSE 1 END;
DROP TABLE planning_rearchive_downgrade_guard;
DROP TABLE planning_rearchives;
