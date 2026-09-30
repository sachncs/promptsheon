CREATE TABLE IF NOT EXISTS goal_evolution_state (
  manifest_hash TEXT PRIMARY KEY,
  state_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_goal_evolution_state_updated
  ON goal_evolution_state (updated_at DESC);
