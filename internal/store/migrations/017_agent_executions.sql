-- 017_agent_executions.sql
-- Track agent execution history

CREATE TABLE IF NOT EXISTS agent_executions (
    id                   TEXT PRIMARY KEY,
    agent_id             TEXT NOT NULL,
    workflow_id          TEXT DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'pending',
    input                TEXT DEFAULT '{}',
    output               TEXT DEFAULT '{}',
    steps                TEXT DEFAULT '[]',
    total_cost_usd       REAL DEFAULT 0,
    total_latency_ms     INTEGER DEFAULT 0,
    guardrail_violations TEXT DEFAULT '[]',
    context_id           TEXT DEFAULT '',
    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at         DATETIME
);

CREATE INDEX IF NOT EXISTS idx_agent_executions_agent_id ON agent_executions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_status ON agent_executions(status);
CREATE INDEX IF NOT EXISTS idx_agent_executions_created_at ON agent_executions(created_at);
