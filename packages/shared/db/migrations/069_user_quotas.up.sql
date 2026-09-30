CREATE TABLE user_quotas (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  label TEXT NOT NULL,
  daily_runs INTEGER CHECK (daily_runs IS NULL OR daily_runs >= 1),
  daily_tokens INTEGER CHECK (daily_tokens IS NULL OR daily_tokens >= 1),
  daily_cost_micros INTEGER CHECK (daily_cost_micros IS NULL OR daily_cost_micros >= 1),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (organization_id, user_id)
);

CREATE INDEX idx_user_quotas_org ON user_quotas(organization_id, updated_at DESC);
