-- 042_feature_flag_value.up.sql
-- Add a JSON `value` column to feature_flags so the frontend
-- feature-flag editor can store richer payloads than a boolean
-- toggled at the row level. Existing rows get the default 'null'
-- (JSON null), which the UI serializes as a JSON null.

ALTER TABLE feature_flags ADD COLUMN value TEXT NOT NULL DEFAULT 'null';
