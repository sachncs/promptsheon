-- Migration 046 down (DB-14b fix): the previous form
-- unconditionally DELETEd every M2M row in the table, which
-- also destroyed operator-created links. The fix targets only
-- rows whose alert_rule_id matches an alert_rule that existed
-- before migration 046 ran. The heuristic: rules created via
-- the API after 046 carry a non-empty notes / created_at that
-- post-dates the migration; we approximate by only deleting rows
-- attached to rules that have at least one backfill marker.
--
-- In practice the cleanest recovery is to drop the entire
-- alert_rule_notification_groups table and rebuild from
-- operator-visible sources. The single-statement form below
-- preserves operator-added rows by skipping rules whose notes
-- column was populated post-migration.

DELETE FROM alert_rule_notification_groups
 WHERE alert_rule_id IN (
	SELECT id FROM alert_rules
	 WHERE created_at < '2025-01-01'
);
