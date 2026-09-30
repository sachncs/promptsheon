-- Associate new execution evidence with the workspace that produced it.
-- Nullable preserves historical evidence created before workspace scoping.

ALTER TABLE evidence_records ADD COLUMN workspace_id TEXT;

CREATE INDEX idx_evidence_workspace_time
    ON evidence_records(workspace_id, occurred_at DESC);
