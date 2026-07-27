-- 002: Audit chain hash infrastructure.
--
-- "DO NOT MODIFY" — the hash format is part of the tamper-evidence
-- contract. Any change here invalidates audit_chain_state.last_hash
-- across all deployments.
--
-- The previous_hash / entry_hash / timestamp_str / resource_kind /
-- resource_id columns are declared directly in 001_core_schema
-- (so they exist from the start of v0.1.x). The UPDATE that
-- backfills timestamp_str from the DATETIME column is the only
-- pure data step here, and it is a no-op on fresh installs.

UPDATE audit_entries
SET timestamp_str = strftime('%Y-%m-%dT%H:%M:%fZ', timestamp)
WHERE timestamp_str = '' AND timestamp IS NOT NULL;

-- Append-only invariant. DELETE is unrestricted because the
-- audit worker writes a new chain state row before any delete;
-- the rowid mismatch cross-check at VerifyAuditChain (sqlite.go)
-- catches any unpaired delete.
CREATE TRIGGER audit_entries_no_update
BEFORE UPDATE ON audit_entries
BEGIN
    SELECT RAISE(ABORT, 'audit_entries are append-only');
END;
