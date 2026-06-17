-- Migration 008: Add provider binding to prompts for per-prompt provider resolution.
ALTER TABLE prompts ADD COLUMN binding TEXT DEFAULT '';
