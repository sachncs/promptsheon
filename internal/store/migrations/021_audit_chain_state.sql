-- Migration 021: Audit chain state table.
--
-- The previous implementation read `SELECT entry_hash FROM
-- audit_entries ORDER BY rowid DESC LIMIT 1` on every AppendAudit
-- call to compute the next chain link. That single query became the
-- hot row that serialised all audit writes. This migration moves the
-- "last hash" pointer into a dedicated single-row table that the
-- AppendAudit transaction updates atomically. VerifyAuditChain still
-- walks the full table to detect tampering, so verification
-- semantics are unchanged.
CREATE TABLE IF NOT EXISTS audit_chain_state (
    id INTEGER PRIMARY KEY CHECK (id = 0),
    last_hash TEXT NOT NULL DEFAULT '',
    last_rowid INTEGER NOT NULL DEFAULT 0
);

-- Seed the state from the existing chain so we don't break the
-- verify path on already-deployed instances.
INSERT OR IGNORE INTO audit_chain_state (id, last_hash, last_rowid)
SELECT 0, entry_hash, rowid
FROM audit_entries
ORDER BY rowid DESC
LIMIT 1;
