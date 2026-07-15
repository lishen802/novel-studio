-- +goose Up
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    genre TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE chapters (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 1),
    body TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, position)
);

CREATE TABLE chapter_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(chapter_id, revision)
);

CREATE TABLE drafts (
    chapter_id TEXT PRIMARY KEY REFERENCES chapters(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    base_revision INTEGER NOT NULL CHECK (base_revision >= 1),
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_chapters_project_position
    ON chapters(project_id, position);

-- +goose Down
DROP TABLE drafts;
DROP TABLE chapter_revisions;
DROP TABLE chapters;
DROP TABLE projects;
