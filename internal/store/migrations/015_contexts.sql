-- 015_contexts.sql
-- Context management for agent steps

CREATE TABLE IF NOT EXISTS contexts (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    description         TEXT DEFAULT '',
    type                TEXT NOT NULL DEFAULT 'system_prompt',
    system_prompt       TEXT DEFAULT '',
    messages            TEXT DEFAULT '[]',
    token_budget        INTEGER DEFAULT 4096,
    token_count         INTEGER DEFAULT 0,
    truncation_strategy TEXT DEFAULT 'sliding_window',
    agent_id            TEXT DEFAULT '',
    version             INTEGER DEFAULT 1,
    status              TEXT DEFAULT 'draft',
    metadata            TEXT DEFAULT '{}',
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_contexts_agent_id ON contexts(agent_id);
CREATE INDEX IF NOT EXISTS idx_contexts_type ON contexts(type);
CREATE INDEX IF NOT EXISTS idx_contexts_status ON contexts(status);
