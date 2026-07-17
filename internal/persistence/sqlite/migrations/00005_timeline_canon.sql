-- +goose Up
CREATE TABLE story_events (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    cause TEXT NOT NULL DEFAULT '',
    result TEXT NOT NULL DEFAULT '',
    story_time TEXT NOT NULL DEFAULT '',
    story_order INTEGER NOT NULL CHECK (story_order >= 0),
    narrative_order INTEGER NOT NULL CHECK (narrative_order >= 1),
    status TEXT NOT NULL CHECK (status IN ('planned', 'occurred', 'uncertain')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE INDEX idx_story_events_story_order
    ON story_events(project_id, story_order, narrative_order)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_story_events_narrative_order
    ON story_events(project_id, narrative_order)
    WHERE deleted_at IS NULL;

CREATE TABLE story_event_links (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL REFERENCES story_events(id),
    target_type TEXT NOT NULL CHECK (target_type IN ('chapter', 'character', 'faction', 'location', 'item')),
    target_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(event_id, target_type, target_id)
);

CREATE INDEX idx_story_event_links_target
    ON story_event_links(project_id, target_type, target_id);

CREATE TABLE canon_facts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fact_text TEXT NOT NULL,
    category TEXT NOT NULL,
    origin TEXT NOT NULL CHECK (origin IN ('author', 'text', 'model', 'rule')),
    status TEXT NOT NULL CHECK (status IN ('author_confirmed', 'text_evidenced', 'ai_inferred', 'conflicted', 'rejected')),
    confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    valid_from_event_id TEXT REFERENCES story_events(id),
    valid_until_event_id TEXT REFERENCES story_events(id),
    created_by TEXT NOT NULL,
    confirmed_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE INDEX idx_canon_facts_status
    ON canon_facts(project_id, status, category)
    WHERE deleted_at IS NULL;

CREATE TABLE fact_evidence (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fact_id TEXT NOT NULL REFERENCES canon_facts(id),
    chapter_id TEXT NOT NULL REFERENCES chapters(id),
    quote TEXT NOT NULL,
    start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
    end_offset INTEGER NOT NULL CHECK (end_offset > start_offset),
    chapter_revision INTEGER NOT NULL CHECK (chapter_revision >= 1),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_fact_evidence_fact
    ON fact_evidence(project_id, fact_id, chapter_id);

CREATE TABLE fact_scopes (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fact_id TEXT NOT NULL REFERENCES canon_facts(id),
    target_type TEXT NOT NULL CHECK (target_type IN ('character', 'faction', 'location', 'item', 'region', 'rule', 'event')),
    target_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(fact_id, target_type, target_id)
);

CREATE INDEX idx_fact_scopes_target
    ON fact_scopes(project_id, target_type, target_id);

CREATE TABLE fact_conflicts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fact_id TEXT NOT NULL REFERENCES canon_facts(id),
    other_fact_id TEXT NOT NULL REFERENCES canon_facts(id),
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    created_at TEXT NOT NULL,
    resolved_at TEXT,
    CHECK (fact_id <> other_fact_id),
    UNIQUE(fact_id, other_fact_id)
);

CREATE INDEX idx_fact_conflicts_status
    ON fact_conflicts(project_id, status, fact_id);

-- +goose Down
DROP TABLE fact_conflicts;
DROP TABLE fact_scopes;
DROP TABLE fact_evidence;
DROP TABLE canon_facts;
DROP TABLE story_event_links;
DROP TABLE story_events;
