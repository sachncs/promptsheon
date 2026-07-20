-- Migration 048b down: no-op. The 048a columns and the
-- backfill data are preserved. Operators rollback via the
-- 048a down (drops the columns) and the audit chain remains
-- intact because the columns are not part of the audit hash.

SELECT 1;
