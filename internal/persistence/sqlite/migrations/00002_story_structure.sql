-- +goose Up
CREATE UNIQUE INDEX idx_chapters_project_id_id
    ON chapters(project_id, id);

CREATE TABLE outline_nodes (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id TEXT,
    level TEXT NOT NULL CHECK (level IN ('book', 'volume', 'arc', 'chapter', 'scene')),
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    goal TEXT NOT NULL DEFAULT '',
    conflict TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position >= 1),
    chapter_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id),
    FOREIGN KEY(project_id, parent_id) REFERENCES outline_nodes(project_id, id),
    FOREIGN KEY(project_id, chapter_id) REFERENCES chapters(project_id, id),
    CHECK (
        (level = 'book' AND parent_id IS NULL) OR
        (level <> 'book' AND parent_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_outline_nodes_parent_position
    ON outline_nodes(project_id, IFNULL(parent_id, ''), position)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_outline_nodes_single_book
    ON outline_nodes(project_id)
    WHERE level = 'book' AND deleted_at IS NULL;

CREATE TABLE story_lines (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('planned', 'active', 'closed')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id)
);

CREATE TABLE story_line_links (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    story_line_id TEXT NOT NULL,
    outline_node_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 1),
    created_at TEXT NOT NULL,
    PRIMARY KEY(story_line_id, outline_node_id),
    UNIQUE(story_line_id, position),
    FOREIGN KEY(project_id, story_line_id) REFERENCES story_lines(project_id, id),
    FOREIGN KEY(project_id, outline_node_id) REFERENCES outline_nodes(project_id, id)
);

CREATE INDEX idx_story_line_links_position
    ON story_line_links(story_line_id, position);

CREATE TABLE scene_cards (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL,
    outline_node_id TEXT NOT NULL,
    title TEXT NOT NULL,
    pov TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    goal TEXT NOT NULL DEFAULT '',
    conflict TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position >= 1),
    status TEXT NOT NULL CHECK (status IN ('planned', 'drafted', 'final')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    UNIQUE(project_id, id),
    FOREIGN KEY(project_id, chapter_id) REFERENCES chapters(project_id, id),
    FOREIGN KEY(project_id, outline_node_id) REFERENCES outline_nodes(project_id, id)
);

CREATE UNIQUE INDEX idx_scene_cards_chapter_position
    ON scene_cards(chapter_id, position)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE scene_cards;
DROP TABLE story_line_links;
DROP TABLE story_lines;
DROP TABLE outline_nodes;
DROP INDEX idx_chapters_project_id_id;
