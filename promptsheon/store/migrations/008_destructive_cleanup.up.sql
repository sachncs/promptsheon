-- 008: Drop pre-v0.1.0 legacy tables.
--
-- This migration is GATED by the destructive-gate in migrate.go
-- (filename matches ^\d+_destructive), so the runner refuses to
-- apply it unless PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true.
--
-- The legacy per-artifact column drops (prompt, model_policy,
-- context_contract, etc.) were previously part of the 044 cleanup
-- but are NOT needed here: 001_core_schema creates capability_versions
-- and capabilities WITHOUT those columns, so on a fresh install
-- they never existed. Pre-v0.1.0 upgrades that have those columns
-- are handled by the operator shim documented in CHANGELOG and
-- migrations/README.md: the operator inserts versions 1..8 into
-- schema_migrations directly, skipping this migration entirely,
-- and the column drops happen in-place before the new daemon starts.
--
-- On fresh installs every DROP here is a no-op (tables don't exist)
-- because 001_core_schema creates the modern tables.

DROP TABLE IF EXISTS prompts;
DROP TABLE IF EXISTS prompt_versions;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS agent_executions;
DROP TABLE IF EXISTS agent_guardrail_configs;
DROP TABLE IF EXISTS contexts;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS workflow_steps;
DROP TABLE IF EXISTS output_snapshots;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS test_datasets;
DROP TABLE IF EXISTS execution_logs;
