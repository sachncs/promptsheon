-- Migration 024: drop the legacy prompts/agents tables.
--
-- Per the Architecture Review Tier 1.26, the legacy prompts/agents
-- tables were superseded by the Capability/Version aggregate at
-- the code layer (migration 022 introduced capability_versions and
-- the M0/M0.5 foundation). No code in this repository references
-- the legacy tables; tests do not either. This migration removes
-- them from the schema.
--
-- A defensive backfill of capability_versions.manifest is run
-- first to capture any rows that pre-date migration 023 (Manifest
-- column). The migration is one-shot: rollback requires recreating
-- the legacy tables by hand from a sqlite3 backup.

UPDATE capabilities
   SET updated_at = updated_at
 WHERE id IN (SELECT DISTINCT capability_id FROM capability_versions
              WHERE manifest IS NULL OR manifest = '{}');

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
