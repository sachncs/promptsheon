-- Migration 044 down: rebuild the dropped tables and re-add the
-- dropped columns. This is a recovery path for a deployment that
-- rolled back to before 044 ran. SQLite has no DROP CONSTRAINT
-- equivalent for ADD COLUMN, so each table is rebuilt via the
-- standard "create new, copy, drop, rename" pattern.
--
-- CRITICAL: this down.sql assumes the drop was the only state
-- change between 043 and 044. If a subsequent migration (045,
-- 046, ...) has been applied, this down may fail or produce
-- unexpected results. Treat down.sql as documentation of the
-- rebuild pattern, not a turnkey recovery script.
--
-- In production, prefer reverting the binary and restoring a
-- database snapshot from before 044 ran.

PRAGMA foreign_keys = OFF;

-- Re-create the dropped tables. Schema mirrors the original
-- migrations verbatim; we do not re-introduce the data, just
-- the table skeletons, because no production code wrote to them
-- post-022.
CREATE TABLE IF NOT EXISTS prompts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    content TEXT NOT NULL,
    variables TEXT DEFAULT '[]',
    tags TEXT DEFAULT '[]',
    model_hint TEXT DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    cas_hash TEXT DEFAULT '',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    metadata TEXT DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS prompt_versions (
    id TEXT PRIMARY KEY,
    prompt_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    manifest_hash TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    steps TEXT NOT NULL DEFAULT '[]',
    tools TEXT DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'draft',
    cas_hash TEXT DEFAULT '',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_executions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    workflow_id TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    input TEXT DEFAULT '{}',
    output TEXT DEFAULT '{}',
    steps TEXT DEFAULT '[]',
    total_cost_usd REAL DEFAULT 0,
    total_latency_ms INTEGER DEFAULT 0,
    guardrail_violations TEXT DEFAULT '[]',
    context_id TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS agent_guardrail_configs (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    max_cost_per_run REAL DEFAULT 0,
    max_latency_ms INTEGER DEFAULT 0,
    max_tokens_per_step INTEGER DEFAULT 0,
    content_policy TEXT DEFAULT '[]',
    restricted_terms TEXT DEFAULT '[]',
    stop_on_violation INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

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

CREATE TABLE IF NOT EXISTS output_snapshots (
    id TEXT PRIMARY KEY,
    prompt_hash TEXT NOT NULL,
    prompt_text TEXT NOT NULL,
    model TEXT NOT NULL,
    response_text TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    token_usage TEXT NOT NULL DEFAULT '{}',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    hallucination_score REAL NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reviews (
    id TEXT PRIMARY KEY,
    resource_id TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    author TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    comments TEXT DEFAULT '[]',
    created_at DATETIME NOT NULL,
    resolved_at DATETIME
);

CREATE TABLE IF NOT EXISTS test_datasets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cases TEXT NOT NULL DEFAULT '[]',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS execution_logs (
    id              TEXT PRIMARY KEY,
    prompt_id       TEXT NOT NULL,
    prompt_name     TEXT NOT NULL DEFAULT '',
    prompt_version  INTEGER NOT NULL DEFAULT 1,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'success',
    variables       TEXT,
    system_prompt   TEXT,
    request_messages INTEGER NOT NULL DEFAULT 1,
    prompt_tokens   INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens    INTEGER NOT NULL DEFAULT 0,
    cost_usd        REAL NOT NULL DEFAULT 0.0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    trace_id        TEXT,
    error           TEXT,
    violations      TEXT,
    environment     TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

PRAGMA foreign_keys = ON;
