-- Evidence is emitted by both legacy executions and durable execution jobs.
-- Keep execution_id as a polymorphic correlation field rather than enforcing
-- a foreign key to only one execution table.

DROP INDEX idx_evidence_org_time;
DROP INDEX idx_evidence_trace_time;
DROP INDEX idx_evidence_execution_time;
DROP INDEX idx_evidence_type_time;
DROP INDEX idx_evidence_workspace_time;
DROP INDEX idx_evidence_org_workspace_time;
DROP TRIGGER evidence_records_immutable_update;

CREATE TABLE evidence_records_new (
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
    workspace_id TEXT
);

INSERT INTO evidence_records_new (
    id, event_type, schema_version, occurred_at, organization_id,
    correlation_id, trace_id, execution_id, agent_hash, step_id,
    retention_class, payload_json, payload_hash, created_at, workspace_id
)
SELECT id, event_type, schema_version, occurred_at, organization_id,
       correlation_id, trace_id, execution_id, agent_hash, step_id,
       retention_class, payload_json, payload_hash, created_at, workspace_id
FROM evidence_records;

DROP TABLE evidence_records;
ALTER TABLE evidence_records_new RENAME TO evidence_records;

CREATE INDEX idx_evidence_org_time ON evidence_records(organization_id, occurred_at DESC);
CREATE INDEX idx_evidence_trace_time ON evidence_records(trace_id, occurred_at ASC);
CREATE INDEX idx_evidence_execution_time ON evidence_records(execution_id, occurred_at ASC);
CREATE INDEX idx_evidence_type_time ON evidence_records(organization_id, event_type, occurred_at DESC);
CREATE INDEX idx_evidence_workspace_time ON evidence_records(workspace_id, occurred_at DESC);
CREATE INDEX idx_evidence_org_workspace_time ON evidence_records(organization_id, workspace_id, occurred_at DESC);

CREATE TRIGGER evidence_records_immutable_update
BEFORE UPDATE ON evidence_records
BEGIN
    SELECT RAISE(ABORT, 'evidence records are immutable');
END;
