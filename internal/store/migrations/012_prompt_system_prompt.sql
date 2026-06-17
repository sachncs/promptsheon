-- 012_prompt_system_prompt.sql
-- Add system_prompt column to prompts table

ALTER TABLE prompts ADD COLUMN system_prompt TEXT NOT NULL DEFAULT '';
