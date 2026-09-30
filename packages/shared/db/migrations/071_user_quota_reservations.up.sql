CREATE TABLE IF NOT EXISTS user_quota_reservations (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  execution_job_id TEXT NOT NULL UNIQUE,
  usage_day TEXT NOT NULL,
  reserved_tokens INTEGER NOT NULL,
  reserved_cost_micros INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (execution_job_id) REFERENCES execution_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_quota_reservations_usage
  ON user_quota_reservations (organization_id, user_id, usage_day);
