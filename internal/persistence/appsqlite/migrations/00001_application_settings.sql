-- +goose Up
CREATE TABLE app_preferences (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    theme TEXT NOT NULL DEFAULT 'system',
    default_creative_profile_id TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

INSERT INTO app_preferences (id, theme, default_creative_profile_id, updated_at)
VALUES (1, 'system', '', '1970-01-01T00:00:00Z');

CREATE TABLE provider_connections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    credential_ref TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE creative_profile_versions (
    profile_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    name TEXT NOT NULL,
    max_run_tokens INTEGER NOT NULL,
    max_context_chars INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (profile_id, version)
);

CREATE TABLE creative_profile_roles (
    profile_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    role TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    model TEXT NOT NULL,
    max_output_tokens INTEGER NOT NULL,
    temperature REAL NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (profile_id, version, role),
    FOREIGN KEY (profile_id, version) REFERENCES creative_profile_versions(profile_id, version) ON DELETE CASCADE
);

CREATE TABLE recent_projects (
    project_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    last_workspace TEXT NOT NULL DEFAULT '',
    last_opened_at TEXT NOT NULL,
    missing INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE legacy_imports (
    project_id TEXT NOT NULL,
    source_digest TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    profile_version INTEGER NOT NULL,
    imported_at TEXT NOT NULL,
    PRIMARY KEY (project_id, source_digest)
);

-- +goose Down
DROP TABLE legacy_imports;
DROP TABLE recent_projects;
DROP TABLE creative_profile_roles;
DROP TABLE creative_profile_versions;
DROP TABLE provider_connections;
DROP TABLE app_preferences;
