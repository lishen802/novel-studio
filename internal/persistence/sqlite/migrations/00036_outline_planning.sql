-- +goose Up
-- 大纲节点保存故事弧规划目标等版本化载荷，避免只塞进信封后正式页读不到。
ALTER TABLE outline_nodes ADD COLUMN planning_json TEXT NOT NULL DEFAULT '{}';

-- +goose Down
-- SQLite 无法安全删除已发布列；降级保留空对象以兼容旧读取。
