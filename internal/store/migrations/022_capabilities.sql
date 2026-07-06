-- Capability-centric schema
-- New tables for the capability-oriented domain model.
-- Existing tables are untouched — this migration is purely additive.

CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    organization TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id);

CREATE TABLE IF NOT EXISTS capabilities (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    owner TEXT DEFAULT '',
    tags TEXT DEFAULT '[]',
    state TEXT NOT NULL DEFAULT 'draft',
    current_version_id TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_capabilities_project ON capabilities(project_id);
CREATE INDEX IF NOT EXISTS idx_capabilities_state ON capabilities(state);

CREATE TABLE IF NOT EXISTS capability_versions (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id),
    version INTEGER NOT NULL DEFAULT 1,
    prompt TEXT NOT NULL DEFAULT '{}',
    model_policy TEXT NOT NULL DEFAULT '{}',
    context_contract TEXT NOT NULL DEFAULT '{}',
    knowledge TEXT DEFAULT '[]',
    memory TEXT NOT NULL DEFAULT '{}',
    guardrails TEXT DEFAULT '[]',
    tools TEXT DEFAULT '[]',
    mcp_servers TEXT DEFAULT '[]',
    runtime_policy TEXT NOT NULL DEFAULT '{}',
    evaluation_suite TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL,
    created_by TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_versions_capability ON capability_versions(capability_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_capability_version ON capability_versions(capability_id, version);

CREATE TABLE IF NOT EXISTS executions (
    id TEXT PRIMARY KEY,
    capability_version_id TEXT NOT NULL REFERENCES capability_versions(id),
    timestamp DATETIME NOT NULL,
    inputs TEXT DEFAULT '{}',
    outputs TEXT DEFAULT '{}',
    model TEXT DEFAULT '',
    provider TEXT DEFAULT '',
    latency_ms INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0.0,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    error TEXT DEFAULT '',
    trace_id TEXT DEFAULT '',
    environment TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_executions_version ON executions(capability_version_id);
CREATE INDEX IF NOT EXISTS idx_executions_timestamp ON executions(timestamp);
