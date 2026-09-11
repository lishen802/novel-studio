-- +goose Up
-- 旧候选不回填结果键，避免历史重复数据阻断升级；所有新模型结果由写入层计算稳定键。
ALTER TABLE creation_candidates ADD COLUMN result_key TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_creation_candidates_result_key
    ON creation_candidates(result_key)
    WHERE result_key <> '';

-- +goose Down
DROP INDEX idx_creation_candidates_result_key;
ALTER TABLE creation_candidates DROP COLUMN result_key;
