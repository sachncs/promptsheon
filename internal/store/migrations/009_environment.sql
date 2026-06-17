-- Migration 009: Add environment scoping to prompts.
ALTER TABLE prompts ADD COLUMN environment TEXT DEFAULT 'dev';
CREATE INDEX IF NOT EXISTS idx_prompts_environment ON prompts (environment);
