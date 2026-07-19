-- Migration 051 down: drop the new composite index. The
-- underlying query falls back to idx_audit_user_time (added in
-- 047) for user-scoped audit queries.

DROP INDEX IF EXISTS idx_audit_user_action_time;
