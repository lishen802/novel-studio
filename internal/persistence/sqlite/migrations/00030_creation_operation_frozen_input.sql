-- +goose Up
-- 运行启动时冻结上游依赖摘要，崩溃恢复不得用当前会话状态重新解释旧结果。
ALTER TABLE creation_operations ADD COLUMN frozen_input_digest TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE creation_operations DROP COLUMN frozen_input_digest;
