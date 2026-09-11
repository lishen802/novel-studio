-- +goose Up
-- 起点与材料追加版本；旧会话不伪造作者来源，仍沿用灵感流程。
CREATE TABLE creation_entry_revisions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES creation_sessions(id),
    parent_id TEXT,
    version INTEGER NOT NULL CHECK(version > 0),
    kind TEXT NOT NULL CHECK(kind IN ('idea', 'author_plan')),
    material TEXT NOT NULL,
    material_sha256 TEXT NOT NULL CHECK(length(material_sha256) = 64),
    created_at TEXT NOT NULL,
    UNIQUE(session_id, version),
    UNIQUE(session_id, id),
    FOREIGN KEY(session_id, parent_id) REFERENCES creation_entry_revisions(session_id, id),
    CHECK((version = 1 AND parent_id IS NULL) OR (version > 1 AND parent_id IS NOT NULL))
);
CREATE TABLE creation_entry_heads (
    session_id TEXT PRIMARY KEY REFERENCES creation_sessions(id),
    entry_id TEXT NOT NULL,
    FOREIGN KEY(session_id, entry_id) REFERENCES creation_entry_revisions(session_id, id)
);

-- +goose Down
DROP TABLE creation_entry_heads;
DROP TABLE creation_entry_revisions;
