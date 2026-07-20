-- Migration 054 down: restore guardrail tables and drop the
-- releases FK. The down is best-effort and not part of the
-- production recovery path.

PRAGMA foreign_keys = OFF;

CREATE TABLE guardrail_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    pattern TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT 'redact',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE guardrail_violations (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    input TEXT,
    output TEXT,
    action TEXT NOT NULL DEFAULT 'redact',
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rule_id) REFERENCES guardrail_rules(id)
);

ALTER TABLE releases DROP COLUMN capability_version_id;
DROP INDEX IF EXISTS idx_releases_capability_version_id;

PRAGMA foreign_keys = ON;
