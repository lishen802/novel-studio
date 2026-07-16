-- +goose Up
CREATE TABLE world_regions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id TEXT REFERENCES world_regions(id),
    kind TEXT NOT NULL CHECK (kind IN ('world', 'continent', 'territory', 'settlement', 'terrain', 'secret_realm', 'resource')),
    name TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('north', 'north_east', 'east', 'south_east', 'south', 'south_west', 'west', 'north_west', 'center', 'surrounding')),
    summary TEXT NOT NULL DEFAULT '',
    terrain TEXT NOT NULL DEFAULT '',
    climate TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE UNIQUE INDEX idx_world_regions_single_root
    ON world_regions(project_id)
    WHERE kind = 'world' AND deleted_at IS NULL;

CREATE INDEX idx_world_regions_parent
    ON world_regions(project_id, parent_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE locations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    region_id TEXT NOT NULL REFERENCES world_regions(id),
    kind TEXT NOT NULL CHECK (kind IN ('city', 'settlement', 'building', 'landmark', 'wilderness', 'other')),
    name TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive', 'destroyed')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT
);

CREATE INDEX idx_locations_region
    ON locations(project_id, region_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE location_links (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    location_id TEXT NOT NULL REFERENCES locations(id),
    target_type TEXT NOT NULL CHECK (target_type IN ('chapter', 'scene', 'character', 'faction')),
    target_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(location_id, target_type, target_id)
);

CREATE INDEX idx_location_links_target
    ON location_links(project_id, target_type, target_id);

CREATE TABLE items (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('active', 'lost', 'destroyed')),
    holder_type TEXT CHECK (holder_type IS NULL OR holder_type IN ('character', 'faction', 'location')),
    holder_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    CHECK ((holder_type IS NULL AND holder_id IS NULL) OR (holder_type IS NOT NULL AND holder_id IS NOT NULL))
);

CREATE INDEX idx_items_holder
    ON items(project_id, holder_type, holder_id)
    WHERE deleted_at IS NULL;

CREATE TABLE item_transfers (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    item_id TEXT NOT NULL REFERENCES items(id),
    from_type TEXT CHECK (from_type IS NULL OR from_type IN ('character', 'faction', 'location')),
    from_id TEXT,
    to_type TEXT NOT NULL CHECK (to_type IN ('character', 'faction', 'location')),
    to_id TEXT NOT NULL,
    chapter_id TEXT,
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY(project_id, chapter_id) REFERENCES chapters(project_id, id)
);

CREATE INDEX idx_item_transfers_item
    ON item_transfers(project_id, item_id, created_at);

CREATE TABLE world_rules (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    scope_type TEXT NOT NULL CHECK (scope_type IN ('project', 'region', 'location')),
    scope_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('active', 'inactive')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    CHECK ((scope_type = 'project' AND scope_id IS NULL) OR (scope_type <> 'project' AND scope_id IS NOT NULL))
);

CREATE INDEX idx_world_rules_scope
    ON world_rules(project_id, scope_type, scope_id)
    WHERE deleted_at IS NULL;

CREATE TABLE foreshadowing_hooks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    setup TEXT NOT NULL,
    planted_chapter_id TEXT NOT NULL,
    due_chapter_id TEXT,
    resolved_chapter_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('planted', 'progressing', 'resolved', 'abandoned')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    FOREIGN KEY(project_id, planted_chapter_id) REFERENCES chapters(project_id, id),
    FOREIGN KEY(project_id, due_chapter_id) REFERENCES chapters(project_id, id),
    FOREIGN KEY(project_id, resolved_chapter_id) REFERENCES chapters(project_id, id)
);

CREATE INDEX idx_foreshadowing_hooks_status
    ON foreshadowing_hooks(project_id, status, due_chapter_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE foreshadowing_hooks;
DROP TABLE world_rules;
DROP TABLE item_transfers;
DROP TABLE items;
DROP TABLE location_links;
DROP TABLE locations;
DROP TABLE world_regions;
