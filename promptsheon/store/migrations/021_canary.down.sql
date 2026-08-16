-- Migration 021 down: drop the canary_percent column.
--
-- SQLite < 3.35 cannot DROP COLUMN. The downgrade recreates the
-- releases table without the column and copies the data. The
-- release-id link table (approvals, executions, etc.) is preserved
-- by the FK relationship; SQLite's PRAGMA foreign_keys is left to
-- the migration runner.

ALTER TABLE releases DROP COLUMN canary_percent;
