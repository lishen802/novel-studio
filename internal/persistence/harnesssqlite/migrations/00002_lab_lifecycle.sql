-- +goose Up
ALTER TABLE harness_runs ADD COLUMN lifecycle_state TEXT NOT NULL DEFAULT 'running'
    CHECK(lifecycle_state IN ('queued', 'running', 'recoverable', 'completed', 'failed', 'cancelled'));
ALTER TABLE harness_runs ADD COLUMN retry_of TEXT NOT NULL DEFAULT '';
ALTER TABLE harness_runs ADD COLUMN current_case_id TEXT NOT NULL DEFAULT '';
ALTER TABLE harness_runs ADD COLUMN public_stage_id TEXT NOT NULL DEFAULT '';
ALTER TABLE harness_runs ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE harness_runs ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE harness_runs ADD COLUMN calls INTEGER NOT NULL DEFAULT 0;
UPDATE harness_runs SET lifecycle_state = status;

CREATE TABLE harness_rating_submissions (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    case_id TEXT NOT NULL,
    local_rater_id TEXT NOT NULL,
    ratings_json TEXT NOT NULL CHECK(json_valid(ratings_json)),
    created_at TEXT NOT NULL,
    FOREIGN KEY (run_id, case_id) REFERENCES harness_case_results(run_id, case_id) ON DELETE CASCADE,
    UNIQUE (run_id, case_id, local_rater_id)
);

CREATE INDEX harness_rating_submissions_run_case_idx
    ON harness_rating_submissions(run_id, case_id);

-- +goose Down
DROP TABLE harness_rating_submissions;
ALTER TABLE harness_runs DROP COLUMN calls;
ALTER TABLE harness_runs DROP COLUMN output_tokens;
ALTER TABLE harness_runs DROP COLUMN input_tokens;
ALTER TABLE harness_runs DROP COLUMN public_stage_id;
ALTER TABLE harness_runs DROP COLUMN current_case_id;
ALTER TABLE harness_runs DROP COLUMN retry_of;
ALTER TABLE harness_runs DROP COLUMN lifecycle_state;
