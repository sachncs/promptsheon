CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    input TEXT DEFAULT '{}',
    output TEXT DEFAULT '{}',
    error TEXT DEFAULT '',
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_steps (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    input TEXT DEFAULT '{}',
    output TEXT DEFAULT '{}',
    error TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '[]',
    latency_ms INTEGER DEFAULT 0,
    started_at DATETIME,
    finished_at DATETIME,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id)
);

CREATE INDEX IF NOT EXISTS idx_workflows_agent_id ON workflows(agent_id);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_id ON workflow_steps(workflow_id);
