-- +goose Up
-- 单章产品记录只追加，不替代章节 canonical 正文。
CREATE TABLE chapter_loop_records (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL CHECK(json_valid(payload_json))
);
CREATE INDEX chapter_loop_subject ON chapter_loop_records(project_id, kind, subject_id, sequence);
CREATE UNIQUE INDEX chapter_loop_idempotency ON chapter_loop_records(project_id, kind, subject_id, idempotency_key) WHERE idempotency_key <> '';
-- +goose Down
DROP TABLE chapter_loop_records;
