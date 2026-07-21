-- Migration 059 down: restore the dropped indexes and remove the
-- feature_flags table. Best-effort.

CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_entries(resource);
CREATE INDEX IF NOT EXISTS idx_eval_runs_release_started ON eval_runs(release_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_versions_capability_version_desc ON capability_versions(capability_id, version DESC);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_active ON webhook_endpoints(active);

DROP INDEX IF EXISTS idx_api_keys_user_created;
DROP INDEX IF EXISTS idx_audit_resource_kind_id_time;
DROP INDEX IF EXISTS idx_eval_runs_release_status_started;
DROP TABLE IF EXISTS feature_flags;
