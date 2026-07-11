-- Migration 025 (destructive, Postgres): v0.1.0 forward-only cleanup.
--
-- Postgres supports ALTER TABLE ... DROP COLUMN directly. We
-- remove the vestigial columns on capabilities and the legacy
-- per-artifact columns on capability_versions in the same
-- transaction. The columns are documented unused since M0.8; the
-- v0.1.0 forward-only policy requires this migration.

BEGIN;

ALTER TABLE capabilities DROP COLUMN IF EXISTS state;
ALTER TABLE capabilities DROP COLUMN IF EXISTS current_version_id;
DROP INDEX IF EXISTS idx_capabilities_state;

ALTER TABLE capability_versions DROP COLUMN IF EXISTS prompt;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS model_policy;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS context_contract;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS knowledge;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS memory;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS guardrails;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS tools;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS mcp_servers;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS runtime_policy;
ALTER TABLE capability_versions DROP COLUMN IF EXISTS evaluation_suite;

-- Drop the legacy prompts/agents tables. CASCADE is required
-- because earlier migrations may have created foreign keys
-- that other code paths did not anticipate.
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS agent_executions CASCADE;
DROP TABLE IF EXISTS test_datasets CASCADE;
DROP TABLE IF EXISTS eval_results CASCADE;
DROP TABLE IF EXISTS eval_runs CASCADE;
DROP TABLE IF EXISTS reviews CASCADE;
DROP TABLE IF EXISTS prompt_versions CASCADE;
DROP TABLE IF EXISTS prompts CASCADE;
DROP TABLE IF EXISTS output_snapshots CASCADE;
DROP TABLE IF EXISTS workflow_steps CASCADE;
DROP TABLE IF EXISTS workflows CASCADE;

COMMIT;
