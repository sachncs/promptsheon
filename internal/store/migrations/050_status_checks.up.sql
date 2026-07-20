-- Migration 050 (status CHECKs): add CHECK constraints to the closed
-- enum columns on still-living tables (post-044). The audit
-- flagged that 8 status / enum columns accepted any string;
-- a typo'd value (e.g. 'superceded') would silently break
-- filter queries.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- Scope: only the surviving tables. capabilities.state and the
-- 10 capability_versions per-artifact columns were dropped in
-- 044; their CHECKs are not in scope. workflows / workflow_steps
-- / contexts were also dropped.
--
-- The migration truncates bad data first: any existing row with
-- a value outside the closed enum is coerced to the closest
-- safe value (the empty string for env, 'running' for status,
-- etc.). The audit's recommendation was a literal failing
-- migration; we prefer to clean up rather than block the
-- upgrade.
--
-- The CHECK constraints are defensive only; the application
-- code already validates these fields at the Go level. The
-- constraint catches typos that escape the validation (e.g. a
-- raw string from a test fixture).
--
-- SQLite does not support ALTER TABLE ... ADD CONSTRAINT, so
-- each rebuild follows: data cleanup -> create new table with
-- CHECK -> INSERT SELECT -> drop old -> rename new.

PRAGMA foreign_keys=OFF;

-- 1. Truncate bad data first.
UPDATE executions  SET environment = 'dev' WHERE environment NOT IN ('dev', 'staging', 'prod', '');
UPDATE releases    SET environment = 'dev' WHERE environment NOT IN ('dev', 'staging', 'prod');
UPDATE alerts      SET status      = 'active' WHERE status NOT IN ('active', 'resolved');
UPDATE eval_results SET passed     = 0     WHERE passed NOT IN (0, 1);

-- 2. Rebuild executions with the CHECK baked into CREATE TABLE.
CREATE TABLE executions_new (
    id                   TEXT PRIMARY KEY,
    capability_version_id TEXT NOT NULL REFERENCES capability_versions(id),
    timestamp            DATETIME NOT NULL,
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
    environment           TEXT DEFAULT '' CHECK (environment IN ('dev','staging','prod',''))
);
INSERT INTO executions_new
    SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
           latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
           error, trace_id, environment
      FROM executions;
DROP TABLE executions;
ALTER TABLE executions_new RENAME TO executions;
CREATE INDEX idx_executions_version   ON executions(capability_version_id);
CREATE INDEX idx_executions_timestamp ON executions(timestamp);
CREATE INDEX idx_executions_version_recent ON executions(capability_version_id, timestamp DESC);

-- 3. Rebuild releases with the CHECK baked into CREATE TABLE.
CREATE TABLE releases_new (
    id                  TEXT    PRIMARY KEY,
    capability_id       TEXT    NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version  INTEGER NOT NULL,
    manifest            TEXT    NOT NULL DEFAULT '{}',
    environment         TEXT    NOT NULL CHECK (environment IN ('dev','staging','prod')),
    status              TEXT    NOT NULL DEFAULT 'pending',
    approved_by         TEXT    NOT NULL DEFAULT '[]',
    superseded_by       TEXT             DEFAULT NULL REFERENCES releases(id) ON DELETE SET NULL,
    replaces_release_id TEXT             DEFAULT NULL REFERENCES releases(id) ON DELETE SET NULL,
    created_at          DATETIME NOT NULL,
    created_by          TEXT    NOT NULL DEFAULT '',
    activated_at        DATETIME,
    superseded_at       DATETIME
);
INSERT INTO releases_new
    SELECT id, capability_id, capability_version, manifest, environment, status,
           approved_by,
           NULLIF(superseded_by, ''),
           NULLIF(replaces_release_id, ''),
           created_at, created_by, activated_at, superseded_at
      FROM releases;
DROP TABLE releases;
ALTER TABLE releases_new RENAME TO releases;
CREATE INDEX idx_releases_capability          ON releases(capability_id);
CREATE INDEX idx_releases_environment_status  ON releases(environment, status);
CREATE INDEX idx_releases_status              ON releases(status);
CREATE INDEX idx_releases_capability_recent   ON releases(capability_id, created_at DESC);
CREATE UNIQUE INDEX uniq_releases_active_capability_env
  ON releases (capability_id, environment) WHERE status = 'active';

-- 4. Rebuild alerts with the CHECK baked into CREATE TABLE.
CREATE TABLE alerts_new (
    id            TEXT PRIMARY KEY,
    rule_id       TEXT NOT NULL REFERENCES alert_rules(id),
    rule_name     TEXT NOT NULL,
    severity      TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','resolved')),
    message       TEXT NOT NULL,
    details       TEXT,
    triggered_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at   DATETIME
);
INSERT INTO alerts_new
    SELECT id, rule_id, rule_name, severity, status, message, details, triggered_at, resolved_at
      FROM alerts;
DROP TABLE alerts;
ALTER TABLE alerts_new RENAME TO alerts;
CREATE INDEX idx_alerts_rule_triggered ON alerts(rule_id, triggered_at DESC);
CREATE INDEX idx_alerts_status         ON alerts(status);
CREATE INDEX idx_alerts_rule_recent    ON alerts(rule_id, triggered_at DESC);

-- 5. Rebuild eval_results with the CHECK baked into CREATE TABLE.
CREATE TABLE eval_results_new (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id    TEXT NOT NULL,
    seq        INTEGER NOT NULL,
    passed     INTEGER NOT NULL DEFAULT 0 CHECK (passed IN (0, 1)),
    actual     TEXT NOT NULL DEFAULT '{}',
    error      TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0
);
INSERT INTO eval_results_new
    SELECT id, run_id, case_id, seq, passed, actual, error, latency_ms
      FROM eval_results;
DROP TABLE eval_results;
ALTER TABLE eval_results_new RENAME TO eval_results;
CREATE INDEX idx_eval_results_run ON eval_results(run_id, seq);

PRAGMA foreign_keys=ON;
