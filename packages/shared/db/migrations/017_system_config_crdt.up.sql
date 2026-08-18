-- 017: system_config CRDT — last-write-wins register per key
-- with a version vector, replica id, tombstone, and
-- monotonic timestamp.
--
-- The previous schema was key/value only (no CRDT metadata);
-- two replicas writing the same key would race and lose one
-- side of the update. This migration adds the columns needed
-- for conflict-safe merge and deterministic concurrent
-- tie-break (timestamp then replica id).
--
-- The columns are added with safe defaults so an existing
-- production DB upgrades in place without a destructive
-- rebuild. tombstone=0 means "live row", replica_id='init'
-- seeds an in-process replica placeholder, write_ts=0 is
-- the natural floor, and version_vector='{}' is the empty
-- vector.

ALTER TABLE system_config ADD COLUMN replica_id      TEXT    NOT NULL DEFAULT 'init';
ALTER TABLE system_config ADD COLUMN version_vector  TEXT    NOT NULL DEFAULT '{}';
ALTER TABLE system_config ADD COLUMN tombstone       INTEGER NOT NULL DEFAULT 0;
ALTER TABLE system_config ADD COLUMN write_ts        INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS system_config_replica_id_idx
    ON system_config(replica_id);