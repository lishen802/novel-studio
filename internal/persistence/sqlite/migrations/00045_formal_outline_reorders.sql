-- +goose Up
CREATE TABLE formal_outline_reorders (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 branch_id TEXT NOT NULL,
 first_asset_id TEXT NOT NULL REFERENCES story_assets(id) ON DELETE RESTRICT,
 second_asset_id TEXT NOT NULL REFERENCES story_assets(id) ON DELETE RESTRICT,
 parent_node_id TEXT NOT NULL REFERENCES outline_nodes(id) ON DELETE RESTRICT,
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
CREATE INDEX idx_formal_outline_reorders_boundary ON formal_outline_reorders(project_id,branch_id,parent_node_id,position);

-- +goose Down
CREATE TEMP TABLE formal_outline_reorders_downgrade_guard(value INTEGER NOT NULL);
INSERT INTO formal_outline_reorders_downgrade_guard SELECT CASE WHEN EXISTS(SELECT 1 FROM formal_outline_reorders) THEN NULL ELSE 1 END;
DROP TABLE formal_outline_reorders_downgrade_guard;
DROP TABLE formal_outline_reorders;
