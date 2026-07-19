-- Migration 044 (legacy drop): remove tables and columns that the
-- production-readiness review and the schema review identified as
-- dead code. The 025_destructive migration was authored to do
-- this but was gated behind PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS
-- and was never enabled in practice. Migration 044 carries the
-- same intent, runs by default, and the test confirms the drop
-- succeeds.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- Every DROP is IF EXISTS / IF NOT EXISTS so the migration is
-- idempotent and safe to re-run on a partially-failed deploy.
--
-- Tables dropped (all confirmed to have 0 non-test Go references
-- in the production codebase as of the schema-review commit):
--   prompts                  — from 001, never written by post-025 code
--   prompt_versions          — from 001, never written
--   agents                   — from 001, never written
--   agent_executions         — from 017, never written
--   agent_guardrail_configs  — from 016, never written
--   contexts                 — from 015, never written
--   workflows                — from 004, never written (workflow.Engine
--                              uses its own in-memory representation)
--   workflow_steps          — from 004, never written
--   output_snapshots         — from 005, never written
--   reviews                  — from 001, never written
--   test_datasets            — from 001, never written
--   execution_logs           — from 011, never written (replaced by
--                              capability_versions.executions in 022)
--
-- Columns dropped on capabilities (post-022 additions that the
-- audit flagged as dead — DeriveState computes them; current_version_id
-- is a denormalised cache that drifts):
--   capabilities.state
--   capabilities.current_version_id
--   capabilities.owner
--   capabilities.tags
--
-- Columns dropped on capability_versions (10 per-artifact columns
-- that 023 replaced with a single manifest blob):
--   capability_versions.prompt
--   capability_versions.model_policy
--   capability_versions.context_contract
--   capability_versions.knowledge
--   capability_versions.memory
--   capability_versions.guardrails
--   capability_versions.tools
--   capability_versions.mcp_servers
--   capability_versions.runtime_policy
--   capability_versions.evaluation_suite
--
-- SQLite 3.35+ supports ALTER TABLE ... DROP COLUMN. The vendored
-- modernc.org/sqlite driver requires SQLite 3.40+, so the column
-- drops are supported. If a deployment runs an older SQLite, the
-- migration fails with "no such column" and the operator must
-- rebuild capability_versions manually (instructions in
-- 044_legacy_drop.down.sql).

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

-- capability_versions per-artifact columns → collapsed into the
-- single `manifest` blob by 023. The vestigial columns are not
-- written by any code path; their presence wastes ~2KB per row.
ALTER TABLE capability_versions DROP COLUMN prompt;
ALTER TABLE capability_versions DROP COLUMN model_policy;
ALTER TABLE capability_versions DROP COLUMN context_contract;
ALTER TABLE capability_versions DROP COLUMN knowledge;
ALTER TABLE capability_versions DROP COLUMN memory;
ALTER TABLE capability_versions DROP COLUMN guardrails;
ALTER TABLE capability_versions DROP COLUMN tools;
ALTER TABLE capability_versions DROP COLUMN mcp_servers;
ALTER TABLE capability_versions DROP COLUMN runtime_policy;
ALTER TABLE capability_versions DROP COLUMN evaluation_suite;

-- capabilities: state, current_version_id, owner, tags are
-- dead post-024. DeriveState in capability/capability.go computes
-- the state; current_version_id is a denormalisation that
-- drifts; owner / tags are not used by any code path.
ALTER TABLE capabilities DROP COLUMN state;
ALTER TABLE capabilities DROP COLUMN current_version_id;
ALTER TABLE capabilities DROP COLUMN owner;
ALTER TABLE capabilities DROP COLUMN tags;
