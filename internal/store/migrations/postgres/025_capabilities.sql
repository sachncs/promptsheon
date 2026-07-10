-- Postgres migration 025: capability-centric schema.
--
-- Mirrors internal/store/migrations/022_capabilities.sql with the
-- dialects adjusted for Postgres (TIMESTAMPTZ, JSONB, INTEGER
-- generated identity, etc.). The schema matches migration 023's
-- Manifest column so the codebase can switch backends without
-- semantic divergence.

CREATE TABLE IF NOT EXISTS workspaces (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    organization  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id);

CREATE TABLE IF NOT EXISTS capabilities (
    id                 TEXT PRIMARY KEY,
    project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    description        TEXT NOT NULL DEFAULT '',
    owner              TEXT NOT NULL DEFAULT '',
    tags               JSONB NOT NULL DEFAULT '[]'::jsonb,
    state              TEXT NOT NULL DEFAULT 'draft',
    current_version_id TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_capabilities_project ON capabilities(project_id);

CREATE TABLE IF NOT EXISTS capability_versions (
    id                TEXT PRIMARY KEY,
    capability_id     TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    version           INTEGER NOT NULL DEFAULT 1,
    manifest          JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest_hash     TEXT NOT NULL DEFAULT '',
    prompt            JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_policy      JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_contract  JSONB NOT NULL DEFAULT '{}'::jsonb,
    knowledge         JSONB NOT NULL DEFAULT '[]'::jsonb,
    memory            JSONB NOT NULL DEFAULT '{}'::jsonb,
    guardrails        JSONB NOT NULL DEFAULT '[]'::jsonb,
    tools             JSONB NOT NULL DEFAULT '[]'::jsonb,
    mcp_servers       JSONB NOT NULL DEFAULT '[]'::jsonb,
    runtime_policy    JSONB NOT NULL DEFAULT '{}'::jsonb,
    evaluation_suite  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL,
    created_by        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_versions_capability ON capability_versions(capability_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_capability_version
    ON capability_versions(capability_id, version);
CREATE INDEX IF NOT EXISTS idx_versions_manifest_hash ON capability_versions(manifest_hash);

CREATE TABLE IF NOT EXISTS executions (
    id                   TEXT PRIMARY KEY,
    capability_version_id TEXT NOT NULL REFERENCES capability_versions(id) ON DELETE CASCADE,
    timestamp            TIMESTAMPTZ NOT NULL,
    inputs               JSONB NOT NULL DEFAULT '{}'::jsonb,
    outputs              JSONB NOT NULL DEFAULT '{}'::jsonb,
    model                TEXT NOT NULL DEFAULT '',
    provider             TEXT NOT NULL DEFAULT '',
    latency_ms           INTEGER NOT NULL DEFAULT 0,
    cost_usd             DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    prompt_tokens        INTEGER NOT NULL DEFAULT 0,
    completion_tokens    INTEGER NOT NULL DEFAULT 0,
    total_tokens         INTEGER NOT NULL DEFAULT 0,
    error                TEXT NOT NULL DEFAULT '',
    trace_id             TEXT NOT NULL DEFAULT '',
    environment          TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_executions_version ON executions(capability_version_id);
CREATE INDEX IF NOT EXISTS idx_executions_timestamp ON executions(timestamp);
