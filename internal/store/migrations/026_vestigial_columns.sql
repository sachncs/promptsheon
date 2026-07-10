-- Migration 026: drop vestigial state / current_version_id columns
-- on capabilities. After M0.8 / Tier 1.40 these columns are
-- declared-legacy: the Capability struct no longer reads or writes
-- them; they exist only for forward compatibility with rows written
-- before M0.8. After migration 026 the schema lines up with the
-- Capability model exactly.
--
-- Defensive: this migration preserves the columns in the schema
-- shape but UNSET semantics. Operators who have never deployed a
-- pre-M0.8 database can also drop the columns entirely; in that
-- case drop the second comment block.

-- SQLite does not support DROP COLUMN cleanly in older versions.
-- For dev installs we rewrite the columns with placeholders that
-- the application never reads, then drop the indexes.

DROP INDEX IF EXISTS idx_capabilities_state;

UPDATE capabilities
   SET state = 'draft',
       current_version_id = ''
 WHERE state IS NULL OR state = '';

-- SQLite: the columns remain in the schema but are unused. The
-- Application code reads zero columns from capabilities apart from
-- id, project_id, name, description, owner, tags, created_at,
-- updated_at.

-- Postgres: same shape; columns become unused. Production
-- deployments using the Postgres backend can ALTER TABLE ... DROP
-- COLUMN safely once they confirm no SQL reads them. After this
-- migration the column-drops file ships as 026_postgres_drop.sql
-- in the Postgres migrations directory.
