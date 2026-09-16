-- +goose Up
-- R4 逐项审阅：按候选版本记录作者的接受/排除关系，并保存审阅位置（独立 view_version）。

CREATE TABLE creation_candidate_reviews (
    candidate_id TEXT PRIMARY KEY REFERENCES creation_candidates(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES creation_sessions(id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (
        stage IN ('intent', 'directions', 'proposal', 'people_world', 'roadmap', 'archive_confirm')
    ),
    review_version INTEGER NOT NULL DEFAULT 1 CHECK (review_version >= 1),
    accepted_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(accepted_json)),
    rejected_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(rejected_json)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_creation_candidate_reviews_session
    ON creation_candidate_reviews(session_id, stage, candidate_id);

CREATE TABLE creation_review_cursors (
    session_id TEXT PRIMARY KEY REFERENCES creation_sessions(id) ON DELETE CASCADE,
    view_version INTEGER NOT NULL DEFAULT 1 CHECK (view_version >= 1),
    stage TEXT NOT NULL DEFAULT '',
    candidate_id TEXT NOT NULL DEFAULT '',
    entity_id TEXT NOT NULL DEFAULT '',
    section_id TEXT NOT NULL DEFAULT '',
    comparison_ref TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE creation_review_cursors;
DROP TABLE creation_candidate_reviews;
