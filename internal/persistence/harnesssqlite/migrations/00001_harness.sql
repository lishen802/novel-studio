-- +goose Up
CREATE TABLE harness_runs (
    id TEXT PRIMARY KEY,
    dataset_id TEXT NOT NULL,
    dataset_version TEXT NOT NULL,
    dataset_sha256 TEXT NOT NULL CHECK(length(dataset_sha256) = 64),
    mode TEXT NOT NULL CHECK(mode IN ('reduced', 'full')),
    random_seed INTEGER NOT NULL,
    binding_json TEXT NOT NULL CHECK(json_valid(binding_json)),
    case_ids_json TEXT NOT NULL CHECK(json_valid(case_ids_json)),
    status TEXT NOT NULL CHECK(status IN ('running', 'completed', 'failed')),
    started_at TEXT NOT NULL,
    completed_at TEXT,
    failure TEXT NOT NULL DEFAULT ''
);

CREATE TABLE harness_case_results (
    run_id TEXT NOT NULL REFERENCES harness_runs(id) ON DELETE CASCADE,
    case_id TEXT NOT NULL,
    genre TEXT NOT NULL,
    task_kind TEXT NOT NULL CHECK(task_kind IN ('blueprint', 'chapter')),
    candidates_json TEXT NOT NULL CHECK(json_valid(candidates_json)),
    anonymous_order_json TEXT NOT NULL CHECK(json_valid(anonymous_order_json)),
    PRIMARY KEY (run_id, case_id)
);

CREATE TABLE harness_ratings (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    case_id TEXT NOT NULL,
    anonymous_slot TEXT NOT NULL,
    dimension TEXT NOT NULL,
    score INTEGER NOT NULL CHECK(score BETWEEN 1 AND 5),
    accepted INTEGER NOT NULL CHECK(accepted IN (0, 1)),
    note TEXT NOT NULL DEFAULT '',
    local_rater_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (run_id, case_id) REFERENCES harness_case_results(run_id, case_id) ON DELETE CASCADE,
    UNIQUE (run_id, case_id, anonymous_slot, dimension, local_rater_id)
);

CREATE TABLE harness_reports (
    run_id TEXT PRIMARY KEY REFERENCES harness_runs(id) ON DELETE CASCADE,
    report_json TEXT NOT NULL CHECK(json_valid(report_json)),
    generated_at TEXT NOT NULL
);

CREATE INDEX harness_runs_dataset_version_idx ON harness_runs(dataset_id, dataset_version, started_at);
CREATE INDEX harness_case_results_genre_idx ON harness_case_results(genre, task_kind);
CREATE INDEX harness_ratings_run_case_idx ON harness_ratings(run_id, case_id);

-- +goose Down
DROP TABLE harness_reports;
DROP TABLE harness_ratings;
DROP TABLE harness_case_results;
DROP TABLE harness_runs;
