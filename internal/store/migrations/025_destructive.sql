-- Migration 025 (destructive): v0.1.0 forward-only cleanup.
--
-- Per the architecture review board (Tier 1.26 + Tier 1.40),
-- the legacy prompts/agents tables and the vestigial
-- capabilities.state / capabilities.current_version_id columns
-- are unused after M0.5 / M0.8. This migration removes them.
--
-- Forward-only: production deployments upgrading to v0.1.0 must
-- run this migration. There is no backwards-compat path; the
-- legacy tables are gone and the columns are gone. Operations
-- that still need them must roll back to v0.0.7.

-- Drop the legacy prompts/agents tables.
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS agent_executions;
DROP TABLE IF EXISTS test_datasets;
DROP TABLE IF EXISTS eval_results;
DROP TABLE IF EXISTS eval_runs;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS prompt_versions;
DROP TABLE IF EXISTS prompts;
DROP TABLE IF EXISTS output_snapshots;
DROP TABLE IF EXISTS workflow_steps;
DROP TABLE IF EXISTS workflows;

-- Drop the vestigial columns on capabilities. SQLite does not
-- support DROP COLUMN cleanly in older versions; this is the
-- destructive path v0.1.0 takes.
DROP INDEX IF EXISTS idx_capabilities_state;
-- capabilities.state removed at the engine level by the
-- destructive migration in 025_postgres.sql (engine-specific).

-- Drop the legacy per-artifact columns on capability_versions.
DROP INDEX IF EXISTS idx_versions_manifest_hash;
-- capability_versions.{prompt, model_policy, context_contract,
-- knowledge, memory, guardrails, tools, mcp_servers,
-- runtime_policy, evaluation_suite} removed at the engine
-- level by the engine-specific migration.
