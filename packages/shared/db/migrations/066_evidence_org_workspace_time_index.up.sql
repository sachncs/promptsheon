-- Evidence reads always scope by organization before optional workspace and time.
-- Keep the tenant boundary first so workspace timelines remain index-backed.

CREATE INDEX idx_evidence_org_workspace_time
    ON evidence_records(organization_id, workspace_id, occurred_at DESC);
