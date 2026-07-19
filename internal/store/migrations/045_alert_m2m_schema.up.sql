-- Migration 045a (alert M2M schema): introduce a proper many-to-many
-- between alert_rules and notification_groups.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- The previous alerting manager joined alert_rules to
-- notification_groups by case-insensitive string match on
-- group.Name vs rule.Severity / rule.Type. That routing was
-- implicit, unindexable, and the cause of CRIT-1 in the
-- production-readiness review. This migration replaces the
-- implicit join with a real M2M table.
--
-- Schema (3 changes):
--   1. notification_groups: add UNIQUE on name, add created_at.
--      The existing channels column stays (channels belong to
--      the group, not the rule).
--   2. alert_rule_notification_groups: new M2M join table.
--   3. alert_rules: no schema change in this migration; the
--      rule→group routing moves to the M2M table.
--
-- 045a alone is forward-compatible but routing-broken: the
-- M2M table is empty so no rules are wired to any group.
-- 045b populates the M2M with backfilled rows so the cutover
-- is silent. 045c (Go) flips getNotificationChannels to the
-- new path.
--
-- A deployment that runs only 045a (and not 045b / 045c) will
-- have no alert notifications sent until 045b + 045c are
-- applied. All three ship in the same PR.

-- (1) notification_groups: rebuild to add UNIQUE on name and a
-- created_at column. The rebuild dance is required because
-- SQLite has no ADD CONSTRAINT UNIQUE syntax.
PRAGMA foreign_keys = OFF;

CREATE TABLE notification_groups_new (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    channels    TEXT    NOT NULL DEFAULT '[]',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO notification_groups_new (id, name, channels, created_at)
  SELECT id, name, channels, CURRENT_TIMESTAMP
    FROM notification_groups;
DROP TABLE notification_groups;
ALTER TABLE notification_groups_new RENAME TO notification_groups;

-- (2) M2M join table.
CREATE TABLE alert_rule_notification_groups (
    alert_rule_id          TEXT    NOT NULL REFERENCES alert_rules(id)         ON DELETE CASCADE,
    notification_group_id  TEXT    NOT NULL REFERENCES notification_groups(id) ON DELETE CASCADE,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (alert_rule_id, notification_group_id)
);
CREATE INDEX idx_arng_group
  ON alert_rule_notification_groups (notification_group_id);

PRAGMA foreign_keys = ON;
