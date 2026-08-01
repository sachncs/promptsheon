-- 020: down-migration drops the immutability trigger and the
-- updated_by_app marker column.

DROP TRIGGER IF EXISTS audit_chain_state_no_update;

-- SQLite ALTER TABLE DROP COLUMN is supported in 3.35.0+ which
-- the project requires (modernc.org/sqlite 1.27+). Keep the
-- downgrade path simple.

-- ALTER TABLE audit_chain_state DROP COLUMN updated_by_app;