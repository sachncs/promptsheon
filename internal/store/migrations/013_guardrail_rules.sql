-- 013_guardrail_rules.sql
-- Persist guardrail rules to database

CREATE TABLE IF NOT EXISTS guardrail_rules (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT 'medium',
    enabled      INTEGER NOT NULL DEFAULT 1,
    config       TEXT,  -- JSON
    environments TEXT,  -- JSON array
    prompt_ids   TEXT,  -- JSON array
    agent_ids    TEXT,  -- JSON array
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guardrail_violations (
    id             TEXT PRIMARY KEY,
    rule_id        TEXT NOT NULL,
    rule_name      TEXT NOT NULL,
    type           TEXT NOT NULL,
    severity       TEXT NOT NULL DEFAULT 'medium',
    resource_type  TEXT NOT NULL DEFAULT '',
    resource_id    TEXT NOT NULL DEFAULT '',
    user_id        TEXT NOT NULL DEFAULT '',
    message        TEXT NOT NULL DEFAULT '',
    details        TEXT,  -- JSON
    resolved       INTEGER NOT NULL DEFAULT 0,
    resolved_by    TEXT,
    resolved_at    DATETIME,
    timestamp      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_guardrail_violations_rule_id ON guardrail_violations(rule_id);
CREATE INDEX IF NOT EXISTS idx_guardrail_violations_resolved ON guardrail_violations(resolved);
