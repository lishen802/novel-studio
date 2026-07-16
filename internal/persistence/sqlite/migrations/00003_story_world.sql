-- +goose Up
CREATE TABLE characters (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    aliases_json TEXT NOT NULL DEFAULT '[]',
    role TEXT NOT NULL CHECK (role IN ('protagonist', 'supporting', 'antagonist', 'minor')),
    summary TEXT NOT NULL DEFAULT '',
    motivation TEXT NOT NULL DEFAULT '',
    fear TEXT NOT NULL DEFAULT '',
    secret TEXT NOT NULL DEFAULT '',
    arc TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive', 'deceased')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id)
);

CREATE INDEX idx_characters_name
    ON characters(project_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE factions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    goal TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive', 'disbanded')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id)
);

CREATE INDEX idx_factions_name
    ON factions(project_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE faction_memberships (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    character_id TEXT NOT NULL,
    faction_id TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'former', 'secret')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(character_id, faction_id),
    FOREIGN KEY(project_id, character_id) REFERENCES characters(project_id, id),
    FOREIGN KEY(project_id, faction_id) REFERENCES factions(project_id, id)
);

CREATE INDEX idx_faction_memberships_faction
    ON faction_memberships(faction_id, status);

CREATE TABLE character_relationships (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_character_id TEXT NOT NULL,
    target_character_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('ally', 'family', 'romantic', 'rival', 'enemy', 'mentor', 'other')),
    label TEXT NOT NULL DEFAULT '',
    strength INTEGER NOT NULL CHECK (strength BETWEEN -100 AND 100),
    public_state TEXT NOT NULL DEFAULT '',
    true_state TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id),
    FOREIGN KEY(project_id, source_character_id) REFERENCES characters(project_id, id),
    FOREIGN KEY(project_id, target_character_id) REFERENCES characters(project_id, id),
    CHECK (source_character_id <> target_character_id)
);

CREATE UNIQUE INDEX idx_relationships_unique_active
    ON character_relationships(project_id, source_character_id, target_character_id, kind)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_relationships_source
    ON character_relationships(source_character_id, kind)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_relationships_target
    ON character_relationships(target_character_id, kind)
    WHERE deleted_at IS NULL;

CREATE TABLE chapter_characters (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL,
    character_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('pov', 'present', 'mentioned')),
    created_at TEXT NOT NULL,
    PRIMARY KEY(chapter_id, character_id),
    FOREIGN KEY(project_id, chapter_id) REFERENCES chapters(project_id, id),
    FOREIGN KEY(project_id, character_id) REFERENCES characters(project_id, id)
);

CREATE INDEX idx_chapter_characters_character
    ON chapter_characters(character_id, chapter_id);

CREATE TABLE scene_card_characters (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    scene_card_id TEXT NOT NULL,
    character_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('pov', 'present', 'mentioned')),
    created_at TEXT NOT NULL,
    PRIMARY KEY(scene_card_id, character_id),
    FOREIGN KEY(project_id, scene_card_id) REFERENCES scene_cards(project_id, id),
    FOREIGN KEY(project_id, character_id) REFERENCES characters(project_id, id)
);

CREATE INDEX idx_scene_card_characters_character
    ON scene_card_characters(character_id, scene_card_id);

-- +goose Down
DROP TABLE scene_card_characters;
DROP TABLE chapter_characters;
DROP TABLE character_relationships;
DROP TABLE faction_memberships;
DROP TABLE factions;
DROP TABLE characters;
