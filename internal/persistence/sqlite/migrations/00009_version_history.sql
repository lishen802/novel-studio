-- +goose Up
ALTER TABLE chapters
    ADD COLUMN revision_source TEXT NOT NULL DEFAULT 'initial'
    CHECK (revision_source IN ('initial', 'author', 'agent', 'restore'));

ALTER TABLE chapters
    ADD COLUMN revision_summary TEXT NOT NULL DEFAULT '';

ALTER TABLE chapter_revisions
    ADD COLUMN source TEXT NOT NULL DEFAULT 'author'
    CHECK (source IN ('initial', 'author', 'agent', 'restore'));

ALTER TABLE chapter_revisions
    ADD COLUMN summary TEXT NOT NULL DEFAULT '';

UPDATE chapter_revisions
SET source = 'initial'
WHERE revision = 1;

UPDATE chapters
SET revision_source = 'author'
WHERE revision > 1;

CREATE INDEX idx_chapter_revisions_chapter_revision_desc
    ON chapter_revisions(chapter_id, revision DESC);

-- +goose Down
DROP INDEX idx_chapter_revisions_chapter_revision_desc;

ALTER TABLE chapter_revisions DROP COLUMN summary;
ALTER TABLE chapter_revisions DROP COLUMN source;
ALTER TABLE chapters DROP COLUMN revision_summary;
ALTER TABLE chapters DROP COLUMN revision_source;
