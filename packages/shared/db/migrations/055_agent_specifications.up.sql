CREATE TABLE agent_specifications (
  hash TEXT NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  schema_version TEXT NOT NULL,
  parent_hash TEXT,
  author TEXT NOT NULL,
  change_reason TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'candidate', 'published', 'retired')),
  created_at TEXT NOT NULL,
  published_at TEXT,
  PRIMARY KEY (workspace_id, hash),
  FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_specifications_hash ON agent_specifications(hash);
CREATE INDEX idx_agent_specifications_parent ON agent_specifications(workspace_id, parent_hash);
CREATE INDEX idx_agent_specifications_created ON agent_specifications(workspace_id, created_at DESC);
