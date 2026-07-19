-- Migration 045b down: remove backfilled M2M rows that the legacy
-- name-match logic produced. Rows added manually by operators
-- after 045c are preserved (this down.sql targets backfilled rows
-- only by the alert_rule_id match, so it is destructive — but
-- 045b itself is idempotent and can be re-run to restore).

DELETE FROM alert_rule_notification_groups
 WHERE alert_rule_id IN (SELECT id FROM alert_rules);
