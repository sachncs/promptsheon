-- 020: Immutability triggers for audit_chain_state.
--
-- 1.5 / CRIT-5: only audit_entries had a no_update trigger. A
-- privileged operator who UPDATEd audit_chain_state manually could
-- break the chain: the next AppendAudit would either commit on
-- top of a stale (rowid, hash) tuple (CAS would pass, but the
-- hash wouldn't match) or enter its infinite CAS retry loop.
--
-- audit_chain_state has exactly one row (id=0); INSERTs are
-- performed by the AppendAudit ON CONFLICT DO NOTHING path on
-- fresh installs. UPDATEs are only legal from the AppendAudit
-- transaction itself. We can't reliably distinguish that UPDATE
-- from a manual one inside the trigger without an in-band
-- marker, so we forbid ALL UPDATEs except via a SQL function
-- the AppendAudit code can call. The code path is updated to
-- use updateAuditChainState(...), which sets the marker.
--
-- For now: forbid all manual UPDATEs. AppendAudit already uses
-- INSERT ... ON CONFLICT(id) DO UPDATE for the first row; that
-- path is unaffected by this trigger.

CREATE TRIGGER audit_chain_state_no_update
BEFORE UPDATE ON audit_chain_state
WHEN NEW.updated_by_app != 1
BEGIN
    SELECT RAISE(ABORT, 'audit_chain_state is append-only; use updateAuditChainState()');
END;

-- Adding the marker column. SQLite ALTER TABLE ADD COLUMN
-- without DEFAULT NOT NULL would fail on existing rows; use a
-- DEFAULT 0 to fill the historical row, then we'll backfill
-- app-driven updates to 1 on next append.
ALTER TABLE audit_chain_state ADD COLUMN updated_by_app INTEGER NOT NULL DEFAULT 0;