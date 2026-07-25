-- 000_postgres_init.up.sql — Postgres schema mirror for Promptsheon.
--
-- This migration runs against a fresh Postgres database; it
-- creates every per-Workspace-scoped table the SQLite backend
-- manages, plus the supporting indexes. The schema is a
-- direct mirror so the consumer-defined Repository interface
-- in internal/store/repo.go is satisfied by both backends.
--
-- RLS (Row Level Security) is applied as a second migration
-- (010_rls.up.sql) so this file is the minimal schema; the
-- RLS layer is opt-in for deployments that need multi-tenant
-- isolation.

CREATE TABLE IF NOT EXISTS workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    organization TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id);

CREATE TABLE IF NOT EXISTS capabilities (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL DEFAULT '',
    tags        JSONB NOT NULL DEFAULT '[]'::jsonb,
    contract    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_capabilities_project ON capabilities(project_id);

CREATE TABLE IF NOT EXISTS capability_versions (
    id            TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    version       INTEGER NOT NULL,
    manifest      JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest_hash TEXT NOT NULL DEFAULT '',
    parents       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    TEXT NOT NULL DEFAULT '',
    UNIQUE (capability_id, version)
);
CREATE INDEX IF NOT EXISTS idx_versions_capability ON capability_versions(capability_id);

CREATE TABLE IF NOT EXISTS capability_contracts (
    capability_id      TEXT PRIMARY KEY REFERENCES capabilities(id) ON DELETE CASCADE,
    blast_radius       TEXT NOT NULL,
    success_rubric     TEXT NOT NULL DEFAULT '',
    auto_promotable    BOOLEAN NOT NULL DEFAULT FALSE,
    input_schema       JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_schema      JSONB NOT NULL DEFAULT '{}'::jsonb,
    slo_max_p95_ms     INTEGER NOT NULL DEFAULT 0,
    slo_min_success    REAL NOT NULL DEFAULT 0,
    slo_max_hallu      REAL NOT NULL DEFAULT 0,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS releases (
    id                   TEXT PRIMARY KEY,
    capability_id        TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version   INTEGER NOT NULL,
    manifest             JSONB NOT NULL DEFAULT '{}'::jsonb,
    environment          TEXT NOT NULL,
    status               TEXT NOT NULL,
    approved_by          JSONB NOT NULL DEFAULT '[]'::jsonb,
    superseded_by        TEXT NOT NULL DEFAULT '',
    replaces_release_id  TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by           TEXT NOT NULL DEFAULT '',
    activated_at         TIMESTAMPTZ,
    superseded_at        TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_releases_capability ON releases(capability_id);
CREATE INDEX IF NOT EXISTS idx_releases_status ON releases(status);
-- ADR-0010: one active Release per (Capability, Environment).
CREATE UNIQUE INDEX IF NOT EXISTS idx_releases_active
    ON releases(capability_id, environment)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS approvals (
    id           TEXT PRIMARY KEY,
    release_id   TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    identity     TEXT NOT NULL,
    decision     TEXT NOT NULL,
    ts           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_approvals_release ON approvals(release_id);

CREATE TABLE IF NOT EXISTS audit_entries (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource        TEXT NOT NULL,
    details         JSONB NOT NULL DEFAULT '{}'::jsonb,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    previous_hash   TEXT NOT NULL DEFAULT '',
    entry_hash      TEXT NOT NULL DEFAULT '',
    timestamp_str   TEXT NOT NULL DEFAULT '',
    resource_kind   TEXT NOT NULL DEFAULT '',
    resource_id     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_user_time ON audit_entries(user_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS audit_chain_state (
    id            INTEGER PRIMARY KEY CHECK (id = 0),
    last_hash     TEXT NOT NULL DEFAULT '',
    last_rowid    BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_archive (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource        TEXT NOT NULL,
    details         JSONB NOT NULL DEFAULT '{}'::jsonb,
    timestamp       TIMESTAMPTZ NOT NULL,
    previous_hash   TEXT NOT NULL DEFAULT '',
    entry_hash      TEXT NOT NULL DEFAULT '',
    timestamp_str   TEXT NOT NULL DEFAULT '',
    resource_kind   TEXT NOT NULL DEFAULT '',
    resource_id     TEXT NOT NULL DEFAULT '',
    archived_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS datasets (
    id              TEXT PRIMARY KEY,
    capability_id   TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_datasets_capability ON datasets(capability_id);

CREATE TABLE IF NOT EXISTS dataset_cases (
    id           TEXT PRIMARY KEY,
    dataset_id   TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    seq          INTEGER NOT NULL,
    inputs       JSONB NOT NULL,
    expected     JSONB NOT NULL,
    UNIQUE (dataset_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_cases_dataset ON dataset_cases(dataset_id);

CREATE TABLE IF NOT EXISTS preconditions (
    id           TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    command      TEXT NOT NULL,
    timeout_sec  INTEGER NOT NULL DEFAULT 60,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_preconditions_capability ON preconditions(capability_id);

CREATE TABLE IF NOT EXISTS eval_runs (
    id            TEXT PRIMARY KEY,
    release_id    TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    dataset_id    TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    scorer        TEXT NOT NULL,
    score         REAL NOT NULL DEFAULT 0,
    passed        INTEGER NOT NULL DEFAULT 0,
    failed        INTEGER NOT NULL DEFAULT 0,
    total         INTEGER NOT NULL DEFAULT 0,
    status        TEXT NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_eval_runs_release ON eval_runs(release_id);

CREATE TABLE IF NOT EXISTS eval_results (
    id           TEXT PRIMARY KEY,
    run_id       TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id      TEXT NOT NULL,
    seq          INTEGER NOT NULL,
    passed       BOOLEAN NOT NULL,
    actual       JSONB NOT NULL,
    error        TEXT NOT NULL DEFAULT '',
    latency_ms   BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_eval_results_run ON eval_results(run_id);
