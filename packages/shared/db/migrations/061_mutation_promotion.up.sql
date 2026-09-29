ALTER TABLE mutation_proposals ADD COLUMN promoted_release_id TEXT;
ALTER TABLE mutation_proposals ADD COLUMN promoted_at TEXT;

CREATE INDEX idx_mutation_proposals_promoted_release
    ON mutation_proposals(promoted_release_id);
