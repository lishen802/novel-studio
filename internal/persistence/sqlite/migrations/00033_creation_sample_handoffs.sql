-- +goose Up
-- 试写准备追加保存；不创建正文或模型运行。
CREATE TABLE creation_sample_handoffs (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 id TEXT NOT NULL UNIQUE,
 session_id TEXT NOT NULL REFERENCES creation_sessions(id),
 direction_candidate_id TEXT NOT NULL REFERENCES creation_candidates(id),
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE INDEX creation_sample_session ON creation_sample_handoffs(session_id,sequence DESC);
-- +goose Down
DROP TABLE creation_sample_handoffs;
