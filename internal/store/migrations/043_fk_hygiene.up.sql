-- Migration 043 (FK hygiene): add foreign-key constraints to production-hot
-- tables that previously had only string columns.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- This migration only adds FK constraints. It is forward-only.
-- The down migration (043_fk_hygiene.down.sql) is a recovery tool,
-- not a routine path; see that file for the rebuild pattern.
--
-- SQLite limitation: ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY is
-- not supported. The only way to attach an FK to an existing
-- table is the rebuild dance (create new, copy, drop, rename).
-- This migration uses the rebuild dance for each affected table
-- with PRAGMA foreign_keys=OFF around the swap so the INSERT
-- does not trigger an FK check on the half-built copy.
--
-- Pre-flight (run by hand, must report 0 in every row before applying):
--   SELECT 'eval_runs.dataset_id orphan'  AS check_name, COUNT(*) FROM eval_runs r
--     LEFT JOIN datasets d ON d.id = r.dataset_id WHERE d.id IS NULL;
--   SELECT 'recommendations.version orphan' AS check_name, COUNT(*) FROM recommendations rec
--     LEFT JOIN capability_versions v ON v.id = rec.capability_version_id WHERE v.id IS NULL;
--   SELECT 'schedules.workspace orphan' AS check_name, COUNT(*) FROM schedules s
--     LEFT JOIN workspaces w ON w.id = s.workspace_id WHERE w.id IS NULL;
--   SELECT 'schedules.release orphan'  AS check_name, COUNT(*) FROM schedules s
--     LEFT JOIN releases r ON r.id = s.release_id WHERE r.id IS NULL;
--   SELECT 'releases.replaces orphan'   AS check_name, COUNT(*) FROM releases r1
--     LEFT JOIN releases r2 ON r2.id = r1.replaces_release_id WHERE r1.replaces_release_id != '' AND r2.id IS NULL;
--   SELECT 'releases.superseded_by orphan' AS check_name, COUNT(*) FROM releases r1
--     LEFT JOIN releases r2 ON r2.id = r1.superseded_by WHERE r1.superseded_by != '' AND r2.id IS NULL;
--   SELECT 'audit.user_id orphan'        AS check_name, COUNT(*) FROM audit_entries a
--     LEFT JOIN users u ON u.id = a.user_id WHERE u.id IS NULL;
--   SELECT 'gv.rule orphan'             AS check_name, COUNT(*) FROM guardrail_violations gv
--     LEFT JOIN guardrail_rules gr ON gr.id = gv.rule_id WHERE gv.rule_id != '' AND gr.id IS NULL;
--   SELECT 'gv.user orphan'             AS check_name, COUNT(*) FROM guardrail_violations gv
--     LEFT JOIN users u ON u.id = gv.user_id WHERE gv.user_id != '' AND u.id IS NULL;
-- If any count is non-zero, this migration fails with a foreign key
-- constraint error during the rebuild INSERT step. Operators must
-- fix the orphan rows (backfill, delete, or repoint) before
-- re-running.

PRAGMA foreign_keys = OFF;

-- ---------------------------------------------------------------------------
-- eval_runs.dataset_id → datasets(id) ON DELETE CASCADE
-- ---------------------------------------------------------------------------
CREATE TABLE eval_runs_new (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    dataset_id TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
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

-- ---------------------------------------------------------------------------
-- recommendations.capability_version_id → capability_versions(id) ON DELETE CASCADE
-- ---------------------------------------------------------------------------
CREATE TABLE recommendations_new (
    id                     TEXT    PRIMARY KEY,
    capability_version_id  TEXT    NOT NULL REFERENCES capability_versions(id) ON DELETE CASCADE,
    type                   TEXT    NOT NULL,
    payload                TEXT    NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO recommendations_new SELECT * FROM recommendations;
DROP TABLE recommendations;
ALTER TABLE recommendations_new RENAME TO recommendations;
CREATE INDEX idx_recommendations_version_created
  ON recommendations (capability_version_id, created_at);

-- ---------------------------------------------------------------------------
-- schedules FKs
-- ---------------------------------------------------------------------------
CREATE TABLE schedules_new (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    release_id   TEXT NOT NULL REFERENCES releases(id)   ON DELETE CASCADE,
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

-- ---------------------------------------------------------------------------
-- releases self-references
-- ---------------------------------------------------------------------------
-- We need the new table to reference itself for replaces_release_id
-- and superseded_by. SQLite handles self-FK as long as the column
-- types match.
CREATE TABLE releases_new (
    id                  TEXT    PRIMARY KEY,
    capability_id       TEXT    NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version  INTEGER NOT NULL,
    manifest            TEXT    NOT NULL DEFAULT '{}',
    environment        TEXT    NOT NULL,
    status              TEXT    NOT NULL DEFAULT 'pending',
    approved_by         TEXT    NOT NULL DEFAULT '[]',
    -- superseded_by and replaces_release_id are nullable so the FK
    -- can express "no reference" without the empty-string sentinel.
    -- The Go side translates "" to NULL.
    superseded_by       TEXT             DEFAULT NULL REFERENCES releases(id) ON DELETE SET NULL,
    replaces_release_id TEXT             DEFAULT NULL REFERENCES releases(id) ON DELETE SET NULL,
    created_at         DATETIME NOT NULL,
    created_by         TEXT    NOT NULL DEFAULT '',
    activated_at       DATETIME,
    superseded_at      DATETIME
);
INSERT INTO releases_new SELECT
    id, capability_id, capability_version, manifest, environment, status,
    approved_by,
    NULLIF(superseded_by, ''),
    NULLIF(replaces_release_id, ''),
    created_at, created_by, activated_at, superseded_at
  FROM releases;
DROP TABLE releases;
ALTER TABLE releases_new RENAME TO releases;
CREATE INDEX idx_releases_capability        ON releases(capability_id);
CREATE INDEX idx_releases_environment_status ON releases(environment, status);
CREATE INDEX idx_releases_status              ON releases(status);
CREATE UNIQUE INDEX uniq_releases_active_capability_env
  ON releases (capability_id, environment) WHERE status = 'active';

-- ---------------------------------------------------------------------------
-- audit_entries.user_id → users(id) ON DELETE RESTRICT
-- ---------------------------------------------------------------------------
-- RESTRICT (not CASCADE) is intentional: deleting a user must not
-- silently drop the security boundary. If the FK rejects the
-- DELETE, the operator sees the explicit error and the audit
-- chain is preserved.
CREATE TABLE audit_entries_new (
    id            TEXT    PRIMARY KEY,
    user_id       TEXT    NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
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
CREATE INDEX idx_audit_resource   ON audit_entries(resource);
CREATE INDEX idx_audit_timestamp   ON audit_entries(timestamp);
CREATE INDEX idx_audit_user        ON audit_entries(user_id);

-- ---------------------------------------------------------------------------
-- guardrail_violations.rule_id → guardrail_rules(id) ON DELETE CASCADE
-- guardrail_violations.user_id → users(id) ON DELETE SET NULL
-- ---------------------------------------------------------------------------
-- user_id SET NULL is safe because the column already has DEFAULT ''
-- and the existing handler writes empty string for the "system
-- actor" path. The new FK only constrains non-empty values.
CREATE TABLE guardrail_violations_new (
    id             TEXT    PRIMARY KEY,
    rule_id        TEXT    NOT NULL REFERENCES guardrail_rules(id) ON DELETE CASCADE,
    rule_name      TEXT    NOT NULL,
    type           TEXT    NOT NULL,
    severity       TEXT    NOT NULL DEFAULT 'medium',
    resource_type  TEXT    NOT NULL DEFAULT '',
    resource_id    TEXT    NOT NULL DEFAULT '',
    user_id        TEXT             REFERENCES users(id) ON DELETE SET NULL,
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

PRAGMA foreign_keys = ON;
