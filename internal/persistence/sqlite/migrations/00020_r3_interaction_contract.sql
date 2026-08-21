-- +goose Up
ALTER TABLE agent_run_steps ADD COLUMN public_stage_id TEXT NOT NULL DEFAULT '' CHECK (
    public_stage_id IN ('', 'context_preparing', 'story_planning', 'chapter_directing',
        'professional_analysis', 'prose_writing', 'independent_review',
        'bounded_revision', 'candidate_ready')
);

ALTER TABLE job_snapshots ADD COLUMN public_projection TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(public_projection));

CREATE TABLE agent_artifact_relations (
    artifact_id TEXT NOT NULL REFERENCES agent_artifacts(id) ON DELETE CASCADE,
    relation TEXT NOT NULL CHECK (relation IN ('derived_from', 'revises', 'supersedes')),
    related_artifact_id TEXT NOT NULL REFERENCES agent_artifacts(id) ON DELETE RESTRICT,
    base_digest TEXT NOT NULL CHECK (
        length(base_digest) = 64 AND base_digest NOT GLOB '*[^0-9a-f]*'
    ),
    created_at TEXT NOT NULL,
    PRIMARY KEY (artifact_id, relation, related_artifact_id),
    CHECK (artifact_id <> related_artifact_id)
);

CREATE INDEX idx_agent_artifact_relations_related ON agent_artifact_relations(related_artifact_id, relation);

-- +goose Down
DROP INDEX idx_agent_artifact_relations_related;
DROP TABLE agent_artifact_relations;
ALTER TABLE job_snapshots DROP COLUMN public_projection;
ALTER TABLE agent_run_steps DROP COLUMN public_stage_id;
