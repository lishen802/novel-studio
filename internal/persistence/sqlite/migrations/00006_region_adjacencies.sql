-- +goose Up
CREATE TABLE region_adjacencies (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    first_region_id TEXT NOT NULL REFERENCES world_regions(id),
    second_region_id TEXT NOT NULL REFERENCES world_regions(id),
    travel_note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(project_id, first_region_id, second_region_id),
    CHECK (first_region_id < second_region_id)
);

CREATE INDEX idx_region_adjacencies_second
    ON region_adjacencies(project_id, second_region_id);

-- +goose Down
DROP TABLE region_adjacencies;
