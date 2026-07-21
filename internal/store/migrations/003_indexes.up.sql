-- 004: Covering / performance indexes.

CREATE INDEX idx_schedules_enabled_due
    ON schedules(enabled, next_fire_at) WHERE enabled = 1;

CREATE INDEX idx_executions_version_recent
    ON executions(capability_version_id, timestamp DESC);

CREATE INDEX idx_releases_capability_recent
    ON releases(capability_id, created_at DESC);

CREATE INDEX idx_alerts_rule_recent
    ON alerts(rule_id, triggered_at DESC);

CREATE INDEX idx_datasets_capability_recent
    ON datasets(capability_id, created_at DESC);

CREATE INDEX idx_preconditions_capability_created
    ON preconditions(capability_id, created_at);

CREATE INDEX idx_eval_runs_release_status_started
    ON eval_runs(release_id, status, started_at DESC);

CREATE INDEX idx_audit_user_time
    ON audit_entries(user_id, timestamp DESC);

CREATE INDEX idx_audit_user_action_time
    ON audit_entries(user_id, action, timestamp DESC);

CREATE INDEX idx_audit_resource_kind_id_time
    ON audit_entries(resource_kind, resource_id, timestamp DESC);

CREATE INDEX idx_alerts_rule_triggered
    ON alerts(rule_id, triggered_at DESC);
