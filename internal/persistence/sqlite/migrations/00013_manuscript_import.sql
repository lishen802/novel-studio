-- +goose Up
CREATE TABLE import_jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_name TEXT NOT NULL CHECK (length(trim(source_name)) >= 1),
    source_format TEXT NOT NULL CHECK (source_format IN ('txt', 'markdown')),
    source_encoding TEXT NOT NULL CHECK (length(trim(source_encoding)) >= 1),
    source_hash TEXT NOT NULL CHECK (
        length(source_hash) = 64 AND source_hash NOT GLOB '*[^0-9a-f]*'
    ),
    source_bytes INTEGER NOT NULL CHECK (source_bytes >= 1),
    archived_relative_path TEXT NOT NULL CHECK (length(trim(archived_relative_path)) >= 1),
    status TEXT NOT NULL CHECK (status IN ('prepared', 'applied', 'failed')),
    chapter_count INTEGER NOT NULL CHECK (chapter_count >= 1),
    selected_chapter_count INTEGER NOT NULL DEFAULT 0 CHECK (
        selected_chapter_count >= 0 AND selected_chapter_count <= chapter_count
    ),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    applied_at TEXT,
    CHECK (
        (
            status = 'prepared'
            AND selected_chapter_count = 0
            AND error_code = ''
            AND error_message = ''
            AND applied_at IS NULL
        )
        OR (
            status = 'applied'
            AND selected_chapter_count >= 1
            AND error_code = ''
            AND error_message = ''
            AND applied_at IS NOT NULL
        )
        OR (
            status = 'failed'
            AND selected_chapter_count = 0
            AND length(trim(error_code)) >= 1
            AND length(trim(error_message)) >= 1
            AND applied_at IS NULL
        )
    )
);

CREATE UNIQUE INDEX idx_import_jobs_project_source_hash
    ON import_jobs(project_id, source_hash);

CREATE TABLE import_chapters (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    volume_title TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL CHECK (length(trim(title)) >= 1),
    body TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position >= 1),
    source_start_line INTEGER NOT NULL CHECK (source_start_line >= 1),
    source_end_line INTEGER NOT NULL CHECK (source_end_line >= source_start_line),
    content_hash TEXT NOT NULL CHECK (
        length(content_hash) = 64 AND content_hash NOT GLOB '*[^0-9a-f]*'
    ),
    selected INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)),
    created_chapter_id TEXT REFERENCES chapters(id),
    CHECK (
        (selected = 0 AND created_chapter_id IS NULL)
        OR (selected = 1 AND created_chapter_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_import_chapters_job_position
    ON import_chapters(job_id, position);

-- +goose Down
DROP TABLE import_chapters;
DROP TABLE import_jobs;
