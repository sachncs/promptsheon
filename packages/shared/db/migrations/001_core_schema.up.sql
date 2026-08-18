-- 002: Core schema. All modern tables in final form with FKs,
-- CHECKs, UNIQUE, basic indexes. The Phase 1.x fixes (1.3 url
-- UNIQUE, 1.4 secret column dropped, 1.5 updated_at / last_used,
-- 1.6 enabled CHECK, 1.7 lineage_edges typed parent/child, 1.9
-- alerts acknowledgement, 1.10 ON DELETE SET NULL for executions
-- and alerts) are folded into this single CREATE TABLE block.

-- Users
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'reader',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_users_role ON users(role);

-- API keys
CREATE TABLE api_keys (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    key_hash   TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL DEFAULT '',
    role       TEXT NOT NULL DEFAULT 'reader',
    expires_at DATETIME,
    last_used  DATETIME,
    created_at DATETIME NOT NULL,
    revoked    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_user_created ON api_keys(user_id, created_at DESC);

-- Provider keys (encrypted at rest, BLOB ciphertext)
CREATE TABLE provider_keys (
    id            TEXT PRIMARY KEY,
    provider_name TEXT NOT NULL,
    key_name      TEXT NOT NULL,
    encrypted_key BLOB NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    rotated_at    DATETIME,
    last_used_at  DATETIME
);
CREATE UNIQUE INDEX idx_provider_keys_provider_name
    ON provider_keys(provider_name, key_name);

-- Audit entries (hash chain columns included; resource_kind/id
-- split is added by migration 003).
CREATE TABLE audit_entries (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action        TEXT NOT NULL,
    resource      TEXT NOT NULL,
    details       TEXT DEFAULT '{}',
    timestamp     DATETIME NOT NULL,
    previous_hash TEXT DEFAULT '',
    entry_hash    TEXT DEFAULT '',
    timestamp_str TEXT NOT NULL DEFAULT '',
    resource_kind TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT ''
);

-- Audit chain state (singleton row; CHECK(id=0))
CREATE TABLE audit_chain_state (
    id         INTEGER PRIMARY KEY CHECK (id = 0),
    last_hash  TEXT NOT NULL DEFAULT '',
    last_rowid INTEGER NOT NULL DEFAULT 0
);

-- Workspaces, projects, capabilities, capability_versions
CREATE TABLE workspaces (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    organization TEXT DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

CREATE TABLE projects (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);
CREATE INDEX idx_projects_workspace ON projects(workspace_id);

CREATE TABLE capabilities (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
CREATE INDEX idx_capabilities_project ON capabilities(project_id);

CREATE TABLE capability_versions (
    id            TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    version       INTEGER NOT NULL DEFAULT 1,
    manifest      TEXT NOT NULL DEFAULT '{}',
    manifest_hash TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL,
    created_by    TEXT DEFAULT ''
);
CREATE INDEX idx_versions_capability ON capability_versions(capability_id);
CREATE UNIQUE INDEX idx_versions_capability_version
    ON capability_versions(capability_id, version);

-- Executions (FK ON DELETE SET NULL per Phase 1.10; column
-- nullable so SET NULL can take effect)
CREATE TABLE executions (
    id                    TEXT PRIMARY KEY,
    capability_version_id TEXT REFERENCES capability_versions(id) ON DELETE SET NULL,
    timestamp             DATETIME NOT NULL,
    inputs                TEXT DEFAULT '{}',
    outputs               TEXT DEFAULT '{}',
    model                 TEXT DEFAULT '',
    provider              TEXT DEFAULT '',
    latency_ms            INTEGER DEFAULT 0,
    cost_usd              REAL DEFAULT 0.0,
    prompt_tokens         INTEGER DEFAULT 0,
    completion_tokens     INTEGER DEFAULT 0,
    total_tokens          INTEGER DEFAULT 0,
    error                 TEXT DEFAULT '',
    trace_id              TEXT DEFAULT '',
    environment           TEXT DEFAULT ''
                            CHECK (environment IN ('dev','staging','prod',''))
);
CREATE INDEX idx_executions_version ON executions(capability_version_id);
CREATE INDEX idx_executions_timestamp ON executions(timestamp);

-- Releases (full dual-representation; capability_version_id FK is
-- CASCADE, the legacy (capability_id, capability_version) pair is
-- retained for the application layer to derive capability_version_id).
CREATE TABLE releases (
    id                   TEXT PRIMARY KEY,
    capability_id        TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version   INTEGER NOT NULL,
    capability_version_id TEXT REFERENCES capability_versions(id) ON DELETE CASCADE,
    manifest             TEXT NOT NULL DEFAULT '{}',
    environment          TEXT NOT NULL
                           CHECK (environment IN ('dev','staging','prod')),
    status               TEXT NOT NULL DEFAULT 'pending',
    approved_by          TEXT NOT NULL DEFAULT '[]',
    superseded_by        TEXT DEFAULT NULL
                           REFERENCES releases(id) ON DELETE SET NULL,
    replaces_release_id  TEXT DEFAULT NULL
                           REFERENCES releases(id) ON DELETE SET NULL,
    created_at           DATETIME NOT NULL,
    created_by           TEXT NOT NULL DEFAULT '',
    activated_at         DATETIME,
    superseded_at        DATETIME
);
CREATE INDEX idx_releases_capability ON releases(capability_id);
CREATE INDEX idx_releases_environment_status ON releases(environment, status);
CREATE INDEX idx_releases_status ON releases(status);
CREATE INDEX idx_releases_capability_version_id ON releases(capability_version_id);

-- Approvals
CREATE TABLE approvals (
    release_id TEXT PRIMARY KEY REFERENCES releases(id) ON DELETE CASCADE,
    votes      TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL
);

-- Schedules (enabled CHECK per Phase 1.6)
CREATE TABLE schedules (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    release_id   TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    kind         TEXT NOT NULL,
    cron         TEXT NOT NULL DEFAULT '',
    webhook_path TEXT NOT NULL DEFAULT '',
    next_fire_at DATETIME NOT NULL,
    last_fire_at DATETIME,
    fired_count  INTEGER NOT NULL DEFAULT 0,
    enabled      INTEGER NOT NULL DEFAULT 1
                 CHECK (enabled IN (0,1)),
    created_at   DATETIME NOT NULL,
    created_by   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_schedules_next_fire ON schedules(next_fire_at);
CREATE INDEX idx_schedules_release ON schedules(release_id);

-- Datasets and cases
CREATE TABLE datasets (
    id            TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX idx_datasets_capability ON datasets(capability_id);

CREATE TABLE dataset_cases (
    id          TEXT PRIMARY KEY,
    dataset_id  TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    seq         INTEGER NOT NULL,
    inputs      TEXT NOT NULL DEFAULT '{}',
    expected    TEXT NOT NULL DEFAULT '{}',
    description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_dataset_cases_dataset ON dataset_cases(dataset_id, seq);

-- Preconditions (enabled CHECK per Phase 1.6)
CREATE TABLE preconditions (
    id            TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    command       TEXT NOT NULL,
    timeout_sec   INTEGER NOT NULL DEFAULT 60,
    enabled       INTEGER NOT NULL DEFAULT 1
                 CHECK (enabled IN (0,1)),
    created_at    DATETIME NOT NULL
);
CREATE INDEX idx_preconditions_capability ON preconditions(capability_id);

-- Eval runs and results
CREATE TABLE eval_runs (
    id          TEXT PRIMARY KEY,
    release_id  TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    dataset_id  TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    scorer      TEXT NOT NULL,
    score       REAL NOT NULL DEFAULT 0,
    passed      INTEGER NOT NULL DEFAULT 0,
    failed      INTEGER NOT NULL DEFAULT 0,
    total       INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'running',
    started_at  DATETIME NOT NULL,
    finished_at DATETIME
);

CREATE TABLE eval_results (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id    TEXT REFERENCES dataset_cases(id) ON DELETE SET NULL,
    seq        INTEGER NOT NULL,
    passed     INTEGER NOT NULL DEFAULT 0
               CHECK (passed IN (0,1)),
    actual     TEXT NOT NULL DEFAULT '{}',
    error      TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_eval_results_run ON eval_results(run_id, seq);

-- Alerts (rule_id FK ON DELETE SET NULL per Phase 1.10;
-- acknowledged_at/_by per Phase 1.9)
CREATE TABLE alert_rules (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL,
    severity   TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1
               CHECK (enabled IN (0,1)),
    threshold  REAL NOT NULL DEFAULT 0,
    duration   INTEGER NOT NULL DEFAULT 0,
    window     INTEGER NOT NULL DEFAULT 0,
    config     TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE alerts (
    id              TEXT PRIMARY KEY,
    rule_id         TEXT REFERENCES alert_rules(id) ON DELETE SET NULL,
    rule_name       TEXT NOT NULL,
    severity        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','resolved')),
    message         TEXT NOT NULL,
    details         TEXT,
    triggered_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at     DATETIME,
    acknowledged_at DATETIME,
    acknowledged_by TEXT
);
CREATE INDEX idx_alerts_status ON alerts(status);

-- Notification groups and M2M
CREATE TABLE notification_groups (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    channels   TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE alert_rule_notification_groups (
    alert_rule_id         TEXT NOT NULL
                           REFERENCES alert_rules(id) ON DELETE CASCADE,
    notification_group_id TEXT NOT NULL
                           REFERENCES notification_groups(id) ON DELETE CASCADE,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (alert_rule_id, notification_group_id)
);
CREATE INDEX idx_arng_group
    ON alert_rule_notification_groups(notification_group_id);

-- Recommendations and decisions (decisions.id TEXT PK NOT NULL)
CREATE TABLE recommendations (
    id                    TEXT PRIMARY KEY,
    capability_version_id TEXT NOT NULL
                           REFERENCES capability_versions(id) ON DELETE CASCADE,
    type                  TEXT NOT NULL,
    payload               TEXT NOT NULL,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_recommendations_version_created
    ON recommendations(capability_version_id, created_at);

CREATE TABLE decisions (
    id                TEXT PRIMARY KEY NOT NULL,
    recommendation_id TEXT NOT NULL UNIQUE,
    payload           TEXT NOT NULL,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_decisions_created ON decisions(created_at);

-- Lineage edges (typed parent/child columns per Phase 1.7)
CREATE TABLE lineage_edges (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    capability_id          TEXT NOT NULL
                           REFERENCES capabilities(id) ON DELETE CASCADE,
    parent_capability_id   TEXT NOT NULL,
    parent_version         INTEGER NOT NULL,
    child_capability_id    TEXT NOT NULL,
    child_version          INTEGER NOT NULL,
    source                 TEXT NOT NULL
                           CHECK (source IN ('recommendation','manual','migration')),
    recommendation_id      TEXT,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by             TEXT NOT NULL DEFAULT '',
    notes                  TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (parent_capability_id, parent_version)
        REFERENCES capability_versions(capability_id, version) ON DELETE CASCADE,
    FOREIGN KEY (child_capability_id, child_version)
        REFERENCES capability_versions(capability_id, version) ON DELETE CASCADE,
    UNIQUE (capability_id, parent_capability_id, parent_version, child_capability_id, child_version)
);
CREATE INDEX idx_lineage_edges_capability_id ON lineage_edges(capability_id);
CREATE INDEX idx_lineage_edges_parent ON lineage_edges(parent_capability_id, parent_version);
CREATE INDEX idx_lineage_edges_child ON lineage_edges(child_capability_id, child_version);

-- Feature flags (enabled CHECK per Phase 1.6)
CREATE TABLE feature_flags (
    name        TEXT PRIMARY KEY,
    enabled     INTEGER NOT NULL DEFAULT 0
                CHECK (enabled IN (0,1)),
    description TEXT NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Webhook endpoints (url UNIQUE per Phase 1.3; secret column
-- dropped per Phase 1.4; secret_ciphertext declared here so it
-- exists from the start; 006_security is now a no-op for fresh
-- installs)
CREATE TABLE webhook_endpoints (
    id                TEXT PRIMARY KEY,
    url               TEXT NOT NULL UNIQUE,
    events            TEXT NOT NULL DEFAULT '',
    active            INTEGER NOT NULL DEFAULT 1,
    secret_ciphertext BLOB,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Partial unique index: exactly one active release per
-- (capability_id, environment). Created here so the constraint
-- is part of the core schema from the start.
CREATE UNIQUE INDEX uniq_releases_active_capability_env
    ON releases(capability_id, environment)
    WHERE status = 'active';
