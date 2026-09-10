-- Migration 024: Add updated_at column to releases table.
--
-- The base ReleaseRepo class (refactor 7437239d) uses BaseRepo.update()
-- which writes to an updated_at column. The original releases table
-- (migration 001) did not have this column, causing the canary update
-- route and any future update path to fail.
--
-- Forward-only: add the column with default CURRENT_TIMESTAMP for
-- existing rows, then a separate UPDATE to set it to created_at for
-- consistency.

ALTER TABLE releases ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE releases SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = '';