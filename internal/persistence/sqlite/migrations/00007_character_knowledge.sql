-- +goose Up
CREATE TABLE character_knowledge (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    character_id TEXT NOT NULL REFERENCES characters(id),
    fact_id TEXT REFERENCES canon_facts(id),
    state TEXT NOT NULL CHECK (state IN ('known', 'believed', 'misinformed')),
    belief_text TEXT NOT NULL DEFAULT '',
    available_from_chapter_id TEXT REFERENCES chapters(id),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    CHECK (
        (state IN ('known', 'believed') AND fact_id IS NOT NULL) OR
        (state = 'misinformed' AND belief_text <> '')
    )
);

CREATE INDEX idx_character_knowledge_character
    ON character_knowledge(project_id, character_id, available_from_chapter_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_character_knowledge_fact
    ON character_knowledge(project_id, fact_id)
    WHERE deleted_at IS NULL AND fact_id IS NOT NULL;

-- +goose Down
DROP TABLE character_knowledge;
