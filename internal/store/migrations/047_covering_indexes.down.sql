-- Migration 047 down: drop the new composite indexes and restore
-- the original single-column idx_audit_resource. This is a recovery
-- path; the down is idempotent.

DROP INDEX IF EXISTS idx_audit_user_time;
DROP INDEX IF EXISTS idx_schedules_enabled_due;
DROP INDEX IF EXISTS idx_executions_version_recent;
DROP INDEX IF EXISTS idx_versions_capability_version_desc;
DROP INDEX IF EXISTS idx_releases_capability_recent;
DROP INDEX IF EXISTS idx_alerts_rule_recent;
DROP INDEX IF EXISTS idx_datasets_capability_recent;
DROP INDEX IF EXISTS idx_preconditions_capability_created;
DROP INDEX IF EXISTS idx_eval_runs_release_started;

CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_entries(resource);
CREATE INDEX IF NOT EXISTS idx_audit_user      ON audit_entries(user_id);
