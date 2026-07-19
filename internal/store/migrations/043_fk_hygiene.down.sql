-- Migration 043 down: rebuild the affected tables without FK constraints.
-- This is a recovery path, not a routine operation. SQLite has no
-- DROP CONSTRAINT, so each affected table is rebuilt via the
-- standard "create new, copy, drop, rename" pattern.
--
-- CRITICAL: this down.sql assumes the data in the affected tables
-- has not been mutated since 043_fk_hygiene.up.sql was applied.
-- If new rows were inserted that reference parents which were
-- later deleted, the rebuild below will FAIL because the new rows
-- have orphan references that were prevented by the constraint
-- now being dropped.
--
-- In production, prefer reverting the binary and restoring a
-- database snapshot from before 043 ran. Use this file only on
-- fresh test databases where you have confirmed zero data loss
-- risk.

PRAGMA foreign_keys = OFF;
BEGIN;

CREATE TABLE eval_runs_new (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    scorer TEXT NOT NULL,
    score REAL NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    started_at DATETIME NOT NULL,
    finished_at DATETIME
);
INSERT INTO eval_runs_new SELECT * FROM eval_runs;
DROP TABLE eval_runs;
ALTER TABLE eval_runs_new RENAME TO eval_runs;
CREATE INDEX idx_eval_runs_release ON eval_runs(release_id, started_at DESC);

CREATE TABLE recommendations_new (
    id                     TEXT    PRIMARY KEY,
    capability_version_id  TEXT    NOT NULL,
    type                   TEXT    NOT NULL,
    payload                TEXT    NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO recommendations_new SELECT * FROM recommendations;
DROP TABLE recommendations;
ALTER TABLE recommendations_new RENAME TO recommendations;
CREATE INDEX idx_recommendations_version_created
  ON recommendations (capability_version_id, created_at);

CREATE TABLE schedules_new (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    release_id   TEXT NOT NULL,
    kind         TEXT NOT NULL,
    cron         TEXT NOT NULL DEFAULT '',
    webhook_path TEXT NOT NULL DEFAULT '',
    next_fire_at DATETIME NOT NULL,
    last_fire_at DATETIME,
    fired_count  INTEGER NOT NULL DEFAULT 0,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL,
    created_by   TEXT NOT NULL DEFAULT ''
);
INSERT INTO schedules_new SELECT * FROM schedules;
DROP TABLE schedules;
ALTER TABLE schedules_new RENAME TO schedules;
CREATE INDEX idx_schedules_next_fire ON schedules(next_fire_at);
CREATE INDEX idx_schedules_release   ON schedules(release_id);

CREATE TABLE releases_new (
    id                  TEXT    PRIMARY KEY,
    capability_id       TEXT    NOT NULL,
    capability_version  INTEGER NOT NULL,
    manifest            TEXT    NOT NULL DEFAULT '{}',
    environment        TEXT    NOT NULL,
    status              TEXT    NOT NULL DEFAULT 'pending',
    approved_by         TEXT    NOT NULL DEFAULT '[]',
    superseded_by       TEXT             DEFAULT '',
    replaces_release_id TEXT             DEFAULT '',
    created_at         DATETIME NOT NULL,
    created_by         TEXT    NOT NULL DEFAULT '',
    activated_at       DATETIME,
    superseded_at      DATETIME
);
INSERT INTO releases_new SELECT
    id, capability_id, capability_version, manifest, environment, status,
    approved_by,
    COALESCE(superseded_by, ''),
    COALESCE(replaces_release_id, ''),
    created_at, created_by, activated_at, superseded_at
  FROM releases;
DROP TABLE releases;
ALTER TABLE releases_new RENAME TO releases;
CREATE INDEX idx_releases_capability        ON releases(capability_id);
CREATE INDEX idx_releases_environment_status ON releases(environment, status);
CREATE INDEX idx_releases_status              ON releases(status);
CREATE UNIQUE INDEX uniq_releases_active_capability_env
  ON releases (capability_id, environment) WHERE status = 'active';

CREATE TABLE audit_entries_new (
    id            TEXT    PRIMARY KEY,
    user_id       TEXT    NOT NULL,
    action        TEXT    NOT NULL,
    resource      TEXT    NOT NULL,
    details       TEXT             DEFAULT '{}',
    timestamp     DATETIME NOT NULL,
    previous_hash TEXT             DEFAULT '',
    entry_hash    TEXT             DEFAULT '',
    timestamp_str TEXT    NOT NULL DEFAULT ''
);
INSERT INTO audit_entries_new SELECT * FROM audit_entries;
DROP TABLE audit_entries;
ALTER TABLE audit_entries_new RENAME TO audit_entries;
CREATE INDEX idx_audit_resource ON audit_entries(resource);
CREATE INDEX idx_audit_timestamp ON audit_entries(timestamp);
CREATE INDEX idx_audit_user      ON audit_entries(user_id);

CREATE TABLE guardrail_violations_new (
    id             TEXT    PRIMARY KEY,
    rule_id        TEXT    NOT NULL,
    rule_name      TEXT    NOT NULL,
    type           TEXT    NOT NULL,
    severity       TEXT    NOT NULL DEFAULT 'medium',
    resource_type  TEXT    NOT NULL DEFAULT '',
    resource_id    TEXT    NOT NULL DEFAULT '',
    user_id        TEXT,
    message        TEXT    NOT NULL DEFAULT '',
    details        TEXT,
    resolved       INTEGER NOT NULL DEFAULT 0,
    resolved_by    TEXT,
    resolved_at    DATETIME,
    timestamp      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO guardrail_violations_new
  SELECT id, rule_id, rule_name, type, severity, resource_type, resource_id,
         user_id, message, details, resolved, resolved_by, resolved_at, timestamp
    FROM guardrail_violations;
DROP TABLE guardrail_violations;
ALTER TABLE guardrail_violations_new RENAME TO guardrail_violations;
CREATE INDEX idx_guardrail_violations_rule_id  ON guardrail_violations(rule_id);
CREATE INDEX idx_guardrail_violations_resolved ON guardrail_violations(resolved);

COMMIT;
PRAGMA foreign_keys = ON;
