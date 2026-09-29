-- 058_evidence_records.up.sql
-- Immutable, tenant-scoped evidence emitted by execution boundaries.

CREATE TABLE evidence_records (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    schema_version TEXT NOT NULL DEFAULT '1.0',
    occurred_at TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    trace_id TEXT,
    execution_id TEXT,
    agent_hash TEXT,
    step_id TEXT,
    retention_class TEXT NOT NULL DEFAULT 'execution',
    payload_json TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE SET NULL
);

CREATE INDEX idx_evidence_org_time ON evidence_records(organization_id, occurred_at DESC);
CREATE INDEX idx_evidence_trace_time ON evidence_records(trace_id, occurred_at ASC);
CREATE INDEX idx_evidence_execution_time ON evidence_records(execution_id, occurred_at ASC);
CREATE INDEX idx_evidence_type_time ON evidence_records(organization_id, event_type, occurred_at DESC);

CREATE TRIGGER evidence_records_immutable_update
BEFORE UPDATE ON evidence_records
BEGIN
    SELECT RAISE(ABORT, 'evidence records are immutable');
END;
