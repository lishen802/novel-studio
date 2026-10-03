-- +goose Up
-- 原始投影不改写，按稳定章序追加约束版本并保留确认回执。
CREATE TABLE story_constraint_changes (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 asset_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision >= 2),
 chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE RESTRICT,
 position INTEGER NOT NULL CHECK(position >= 1),
 reason TEXT NOT NULL,
 digest TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 receipt_json TEXT NOT NULL CHECK(json_valid(receipt_json)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(asset_id,revision),
 UNIQUE(project_id,idempotency_key),
 UNIQUE(asset_id,position),
 FOREIGN KEY(asset_id,revision) REFERENCES story_asset_revisions(asset_id,revision) ON DELETE RESTRICT
);
-- +goose StatementBegin
CREATE TRIGGER protect_constraint_chapter_order BEFORE UPDATE OF position ON chapters
WHEN OLD.position<>NEW.position AND EXISTS(SELECT 1 FROM story_constraint_changes)
BEGIN
 SELECT RAISE(ABORT,'章节已有约束生效边界，不能直接修改章序');
END;
-- +goose StatementEnd
-- +goose Down
-- 已有章节边界不能在降级时静默丢失。
CREATE TEMP TABLE constraint_downgrade_guard(value INTEGER NOT NULL);
INSERT INTO constraint_downgrade_guard SELECT CASE WHEN EXISTS(SELECT 1 FROM story_constraint_changes) THEN NULL ELSE 1 END;
DROP TABLE constraint_downgrade_guard;
DROP TRIGGER protect_constraint_chapter_order;
DROP TABLE story_constraint_changes;
