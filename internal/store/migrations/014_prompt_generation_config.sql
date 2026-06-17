-- 014_prompt_generation_config.sql
-- Add generation config column to prompts table

ALTER TABLE prompts ADD COLUMN generation TEXT NOT NULL DEFAULT '';
