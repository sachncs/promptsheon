-- 043_preconditions_updated_at.up.sql
-- Add an updated_at column so PUT /api/preconditions/:id has a
-- place to stamp edit time. Default to existing created_at
-- semantics by setting it to the row creation time and exposing
-- it via the existing precondition shape.

ALTER TABLE preconditions ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;
