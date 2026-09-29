CREATE TABLE execution_checkpoints (
  execution_id TEXT NOT NULL REFERENCES execution_jobs(id) ON DELETE CASCADE,
  step_id TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('completed', 'failed')),
  output TEXT NOT NULL,
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  PRIMARY KEY (execution_id, step_id)
);

CREATE INDEX idx_execution_checkpoints_execution ON execution_checkpoints(execution_id, created_at ASC);
