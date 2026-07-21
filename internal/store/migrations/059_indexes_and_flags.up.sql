-- Migration 059 (api_keys + audit + eval indexes, redundant cleanup,
-- feature flags). Forward-only; down is best-effort.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * audit_chain_state layout
--   * harness tables
--   * provider_keys
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts
--
-- DB-5a: api_keys(user_id, created_at DESC) index for the
-- list-by-user path.
CREATE INDEX IF NOT EXISTS idx_api_keys_user_created
  ON api_keys(user_id, created_at DESC);

-- DB-8a: idx_audit_resource_kind_id_time. Migration 048 added
-- resource_kind/resource_id columns; this index covers the
-- per-resource timeline query path.
CREATE INDEX IF NOT EXISTS idx_audit_resource_kind_id_time
  ON audit_entries(resource_kind, resource_id, timestamp DESC);

-- DB-16a: drop redundant indexes that the new (resource_kind,
-- resource_id, timestamp) covering index and migration 051's
-- user+action+time composite supersede.
DROP INDEX IF EXISTS idx_audit_resource;
DROP INDEX IF EXISTS idx_eval_runs_release_started;
DROP INDEX IF EXISTS idx_versions_capability_version_desc;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_api_keys_hash;
DROP INDEX IF EXISTS idx_webhook_endpoints_active;

-- DB-REV-3: covering (release_id, status, started_at DESC) for
-- the eval listing path.
CREATE INDEX IF NOT EXISTS idx_eval_runs_release_status_started
  ON eval_runs(release_id, status, started_at DESC);

-- DB-REV-2: feature_flags table for forward-only feature
-- toggles that don't belong in env vars.
CREATE TABLE IF NOT EXISTS feature_flags (
    name TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
