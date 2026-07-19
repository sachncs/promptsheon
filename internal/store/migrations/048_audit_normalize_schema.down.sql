-- Migration 048a down: drop the new columns. The down is
-- destructive for the structural query path (existing rows
-- lose their kind / id), but the chain format is preserved.
-- Operators rolling back to before 048a must also stop
-- reading the new columns in the application code.

ALTER TABLE audit_entries DROP COLUMN resource_kind;
ALTER TABLE audit_entries DROP COLUMN resource_id;
