CREATE TABLE mutation_proposals (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    source_hash TEXT NOT NULL,
    candidate_hash TEXT,
    mutation_kind TEXT NOT NULL CHECK (mutation_kind IN ('prompt', 'guardrail', 'model', 'routing', 'context', 'tool', 'permission', 'execution', 'memory', 'budget')),
    changes_json TEXT NOT NULL,
    rationale TEXT NOT NULL,
    expected_outcome TEXT NOT NULL,
    author_type TEXT NOT NULL CHECK (author_type IN ('human', 'system', 'simulator')),
    author_id TEXT NOT NULL,
    risk TEXT NOT NULL CHECK (risk IN ('low', 'medium', 'high', 'critical')),
    confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    status TEXT NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed', 'validated', 'approved', 'rejected', 'abandoned')),
    evaluation_run_id TEXT,
    decision_reason TEXT,
    reviewed_by TEXT,
    reviewed_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (organization_id, source_hash, changes_json)
);

CREATE INDEX idx_mutation_proposals_org_status
    ON mutation_proposals(organization_id, status, created_at DESC);
CREATE INDEX idx_mutation_proposals_source
    ON mutation_proposals(organization_id, source_hash, created_at DESC);

CREATE TRIGGER mutation_proposals_updated_at
AFTER UPDATE OF status, candidate_hash, evaluation_run_id, decision_reason, reviewed_by, reviewed_at
ON mutation_proposals
BEGIN
    UPDATE mutation_proposals SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
