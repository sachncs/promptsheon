CREATE TABLE execution_jobs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  agent_hash TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  input_json TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'timed-out', 'partially-completed')),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts >= 1 AND max_attempts <= 10),
  available_at TEXT NOT NULL,
  lease_owner TEXT,
  lease_expires_at TEXT,
  result_json TEXT,
  error TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  UNIQUE (organization_id, idempotency_key)
);

CREATE INDEX idx_execution_jobs_claim ON execution_jobs(state, available_at, created_at);
CREATE INDEX idx_execution_jobs_org_state ON execution_jobs(organization_id, state, created_at);
CREATE INDEX idx_execution_jobs_workspace ON execution_jobs(workspace_id, created_at DESC);
CREATE INDEX idx_execution_jobs_agent ON execution_jobs(organization_id, agent_hash, created_at DESC);
