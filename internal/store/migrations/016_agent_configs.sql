-- 016_agent_configs.sql
-- Add context_id and guardrail_config_id to agents table
-- Create agent_guardrail_configs table

ALTER TABLE agents ADD COLUMN context_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN guardrail_config_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS agent_guardrail_configs (
    id                  TEXT PRIMARY KEY,
    agent_id            TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    enabled             INTEGER NOT NULL DEFAULT 1,
    max_cost_per_run    REAL DEFAULT 0,
    max_latency_ms      INTEGER DEFAULT 0,
    max_tokens_per_step INTEGER DEFAULT 0,
    content_policy      TEXT DEFAULT '[]',
    restricted_terms    TEXT DEFAULT '[]',
    stop_on_violation   INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_guardrail_configs_agent_id ON agent_guardrail_configs(agent_id);
