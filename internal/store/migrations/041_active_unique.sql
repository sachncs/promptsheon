-- Migration 041 (active release uniqueness): exactly one Active
-- release per (capability_id, environment) is now a database-level
-- invariant. The previous design enforced this in Go, leaving a
-- window for concurrent Activate calls to leave two active rows
-- in the same environment. The partial unique index turns the
-- invariant into a hard constraint: SQLite rejects the second
-- insert / update that would violate it.

CREATE UNIQUE INDEX IF NOT EXISTS uniq_releases_active_capability_env
  ON releases (capability_id, environment)
  WHERE status = 'active';
