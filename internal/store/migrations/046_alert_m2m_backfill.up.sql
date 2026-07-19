-- Migration 045b (alert M2M backfill): seed alert_rule_notification_groups
-- with the rows the legacy name-match heuristic would have matched.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- The previous getNotificationChannels() matched groups by
-- case-insensitive string on group.Name. The selection order
-- (per the legacy comment) was:
--
--   1. group.Name == lower(rule.Severity)
--   2. group.Name == lower(rule.Type)            (first match)
--   3. group.ID == "default"
--   4. hard-coded ["webhook"] fallback
--
-- This migration reproduces that logic in SQL so the cutover to
-- 045c (the Go switch) is bit-for-bit identical. Operators who
-- want to re-route after the cutover edit the M2M table by hand
-- (INSERT / DELETE on alert_rule_notification_groups); no
-- automatic re-routing runs.
--
-- For every alert_rule, this migration inserts:
--   (a) the matching severity-keyed group, if one exists;
--   (b) ELSE the matching type-keyed group, if one exists;
--   (c) ELSE the "default" group, if it exists;
--   (d) ELSE a synthetic "severity:low" / "severity:medium" /
--       "severity:high" / "severity:critical" group with channels
--       '["webhook"]' is NOT created here — the legacy code's
--       channel-level fallback. Migration 045c preserves that
--       fallback at the Go layer (so alerts without an M2M row
--       still get delivered to the webhook channel), so we
--       don't need to backfill a row.
--
-- Every backfill row uses lower(group.name) so the comparison is
-- case-insensitive, matching the legacy behaviour.
--
-- The migration is idempotent: every INSERT is OR IGNORE. A
-- second run is a no-op.

-- (a) + (b) severity / type match, in that order.
INSERT OR IGNORE INTO alert_rule_notification_groups (alert_rule_id, notification_group_id)
  SELECT r.id, ng.id
    FROM alert_rules r
    JOIN notification_groups ng ON lower(ng.name) = lower(r.severity);

INSERT OR IGNORE INTO alert_rule_notification_groups (alert_rule_id, notification_group_id)
  SELECT r.id, ng.id
    FROM alert_rules r
    JOIN notification_groups ng ON lower(ng.name) = lower(r.type)
   WHERE NOT EXISTS (
     SELECT 1 FROM alert_rule_notification_groups m2m
      WHERE m2m.alert_rule_id = r.id
   );

-- (c) "default" group fallback for rules that didn't match by
-- severity or type. Inserts only when no M2M row exists yet for
-- the rule, mirroring the legacy "first match wins" semantics.
-- We match on lower(ng.name) == 'default' so the fallback works
-- regardless of whether the operator used id='default' or
-- id='g-default' (the typical naming in production).
INSERT OR IGNORE INTO alert_rule_notification_groups (alert_rule_id, notification_group_id)
  SELECT r.id, ng.id
    FROM alert_rules r
    JOIN notification_groups ng ON lower(ng.name) = 'default'
   WHERE NOT EXISTS (
     SELECT 1 FROM alert_rule_notification_groups m2m
      WHERE m2m.alert_rule_id = r.id
   );
