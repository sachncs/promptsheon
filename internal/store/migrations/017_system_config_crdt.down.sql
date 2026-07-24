-- Roll back 017_system_config_crdt.
DROP INDEX IF EXISTS system_config_replica_id_idx;
ALTER TABLE system_config DROP COLUMN write_ts;
ALTER TABLE system_config DROP COLUMN tombstone;
ALTER TABLE system_config DROP COLUMN version_vector;
ALTER TABLE system_config DROP COLUMN replica_id;