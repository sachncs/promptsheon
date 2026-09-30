ALTER TABLE releases ADD COLUMN promotion_proposal_id TEXT
    REFERENCES mutation_proposals(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_releases_promotion_proposal
    ON releases(promotion_proposal_id)
    WHERE promotion_proposal_id IS NOT NULL;
