-- Migration 021: Canary Release primitive (PR-6 / v0.4.0).
--
-- Adds canary_percent to the releases table so an Active release can
-- be promoted in front of a stable release for a fraction of
-- traffic. The value is a percentage in [0, 100]:
--
--   0     → release is fully superseded (no traffic)
--   100   → release is the new stable (full traffic)
--   1..99 → release shares traffic with the prior stable at that
--           percentage
--
-- Per docs/research/audit-fixes-plan.md PR-6 the value defaults to 0;
-- existing rows continue to behave as a normal non-canary release.

ALTER TABLE releases ADD COLUMN canary_percent INTEGER NOT NULL DEFAULT 0
    CHECK (canary_percent >= 0 AND canary_percent <= 100);

-- The resolver path looks up the stable counterpart by
-- (capability_id, environment, status=active, canary_percent=0). The
-- existing idx_releases_environment_status covers the env+status
-- predicate; the canary_percent filter is applied after. No new
-- index is justified at v0.4.0; revisit if EXPLAIN QUERY PLAN shows
-- a regression on workspaces with many active releases.

-- Audit chain event types are documented but not enforced by SQLite
-- schema. The Go audit package (promptsheon/audit) registers the
-- event kinds "canary.create", "canary.promote", and "canary.retire"
-- alongside the existing "release.*" events; see pkg/promptsheon
-- CHANGELOG for the v0.4.0 entry.
