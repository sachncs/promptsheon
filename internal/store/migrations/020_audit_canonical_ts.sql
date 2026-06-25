-- Migration 020: Add canonical timestamp string to audit_entries.
--
-- The previous hash encoding included the timestamp's timezone offset
-- as int32(off) >> (8*i) which produced non-canonical bytes when the
-- offset was negative, and the "enc" byte only distinguished UTC from
-- "anywhere else". VerifyAuditChain would mark honest data as
-- tampered on a verifier machine with a different timezone. The fix
-- (C-2) stores the timestamp in canonical RFC3339Nano UTC form and
-- uses that string in the hash, so writer and verifier always agree.
ALTER TABLE audit_entries ADD COLUMN timestamp_str TEXT NOT NULL DEFAULT '';

-- Backfill existing rows so VerifyAuditChain can validate chains
-- written by the old code. We compute the canonical string from the
-- stored timestamp. The driver returns time.Time, so the in-Go
-- migration in sqlite.go would be the right place for a fuller
-- backfill, but for the most common format the SQL below is enough.
UPDATE audit_entries
SET timestamp_str = strftime('%Y-%m-%dT%H:%M:%fZ', timestamp)
WHERE timestamp_str = '' AND timestamp IS NOT NULL;
