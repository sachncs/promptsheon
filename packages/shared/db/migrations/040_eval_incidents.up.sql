-- 040_eval_incidents.up.sql
-- Phase 2.10: incident-driven eval case proposals. The eval
-- engine seeds new cases from failed execution transcripts;
-- a reviewer accepts (commits into the suite) or rejects.

CREATE TABLE eval_incident_proposals (
    id TEXT PRIMARY KEY,
    suite_id TEXT NOT NULL REFERENCES eval_suites(id) ON DELETE CASCADE,
    case_id TEXT NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('execution_failure', 'manual')),
    source_ref TEXT,
    input_text TEXT NOT NULL,
    expected_text TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'rejected')),
    proposed_by TEXT NOT NULL REFERENCES users(id),
    reviewed_by TEXT REFERENCES users(id),
    reviewed_at DATETIME,
    notes TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_eval_incident_proposals_suite ON eval_incident_proposals(suite_id);
CREATE INDEX idx_eval_incident_proposals_status ON eval_incident_proposals(status);
