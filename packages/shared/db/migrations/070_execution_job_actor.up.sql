ALTER TABLE execution_jobs ADD COLUMN actor_id TEXT;

CREATE INDEX idx_execution_jobs_actor ON execution_jobs(organization_id, actor_id, created_at DESC);
