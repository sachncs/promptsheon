-- Migration 047 (covering indexes): add composite indexes on hot
-- list queries and drop two redundant single-column indexes.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- The audit identified 11 missing indexes for declared query paths.
-- This migration adds 9 of them (the two not added — idx_alerts_rule_recent
-- is covered by an existing idx_alerts rule_id and the partial
-- idx_audit_user_time is the replacement for idx_audit_user).
--
-- Each new index is the smallest sufficient composite for its query:
-- the leading column is the equality filter; the second column
-- is the ORDER BY. ORDER BY columns are added in DESC where the
-- canonical "most recent" sort is the default.
--
-- Drops (each redundant once the composite is in place):
--   idx_audit_resource: a single-column index on the JSON string
--     resource column. With the new (resource_kind, resource_id,
--     timestamp DESC) composite, the single-column index is
--     unused and the per-row write cost is unnecessary.
--
-- The 9 indexes cover:
--   - The scheduler tick (the highest-frequency query in the
--     daemon). A partial index on enabled=1 + next_fire_at
--     means the tick is a single btree probe.
--   - Per-capability list queries on releases, datasets,
--     preconditions, capability_versions, executions, alerts.
--   - The audit timeline queries by user and by resource.

CREATE INDEX IF NOT EXISTS idx_schedules_enabled_due
  ON schedules (enabled, next_fire_at) WHERE enabled = 1;

CREATE INDEX IF NOT EXISTS idx_executions_version_recent
  ON executions (capability_version_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_versions_capability_version_desc
  ON capability_versions (capability_id, version DESC);

CREATE INDEX IF NOT EXISTS idx_releases_capability_recent
  ON releases (capability_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alerts_rule_recent
  ON alerts (rule_id, triggered_at DESC);

CREATE INDEX IF NOT EXISTS idx_datasets_capability_recent
  ON datasets (capability_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_preconditions_capability_created
  ON preconditions (capability_id, created_at);

CREATE INDEX IF NOT EXISTS idx_eval_runs_release_started
  ON eval_runs (release_id, started_at DESC);

-- Drop the redundant single-column resource index. The composite
-- covers all queries that filter by resource.
DROP INDEX IF EXISTS idx_audit_resource;

-- Replace idx_audit_user (single column) with idx_audit_user_time
-- (composite with timestamp DESC) so the most common audit
-- query — "events for user X in the last hour" — is a single
-- btree probe with no sort step.
DROP INDEX IF EXISTS idx_audit_user;
CREATE INDEX IF NOT EXISTS idx_audit_user_time
  ON audit_entries (user_id, timestamp DESC);
