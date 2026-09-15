-- +goose Up
CREATE TABLE assistant_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('author', 'assistant')),
    text TEXT NOT NULL CHECK (length(text) > 0),
    stage TEXT NOT NULL DEFAULT '',
    candidate_id TEXT NOT NULL DEFAULT '',
    entity_ref TEXT NOT NULL DEFAULT '',
    field_path TEXT NOT NULL DEFAULT '',
    action_id TEXT REFERENCES assistant_actions(id) ON DELETE SET NULL,
    idempotency_key TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(session_id, idempotency_key)
);

CREATE INDEX idx_assistant_messages_session_created
    ON assistant_messages(session_id, created_at, id);

-- +goose Down
DROP INDEX idx_assistant_messages_session_created;
DROP TABLE assistant_messages;
