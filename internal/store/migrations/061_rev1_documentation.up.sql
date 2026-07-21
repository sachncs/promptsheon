-- Migration 061 (REV-1 documentation migration): this is a
-- no-op forward migration that records in the schema that
-- legacy tables (prompts, agents, contexts, workflows,
-- workflow_steps, prompt_versions, reviews, execution_logs)
-- were dropped in 044 and are not part of the current product
-- surface. It exists to make the schema match the README
-- ("forward-only: the legacy bundle model and the v0.0.7
-- prompts/agents tables are gone").
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * audit_chain_state layout
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts

-- No DDL. The current state already matches the documented
-- schema; this migration's only effect is to take up a version
-- number so the audit log reflects that the legacy surface was
-- re-checked at the time of the v0.1.x hardening pass.
SELECT 1;
