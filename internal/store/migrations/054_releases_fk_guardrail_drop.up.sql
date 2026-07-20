-- Migration 054 (releases FK + guardrail cleanup): tighten the
-- data model around the release/version linkage and remove
-- unused guardrail tables.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- 1. Releases now reference capability_versions directly. The
--    legacy (capability_id, capability_version) integer pair is
--    augmented with capability_version_id TEXT that resolves to
--    capability_versions.id. New inserts must populate both; the
--    resolver layer ensures consistency.
--
-- 2. guardrail_rules and guardrail_violations are dropped. The
--    guardrail manager is in-memory (internal/guardrail/manager.go);
--    the tables are dead schema. Migration 044 missed them; this
--    closes the gap.

-- ---------------------------------------------------------------------------
-- releases -> capability_versions(id) via capability_version_id
-- ---------------------------------------------------------------------------

ALTER TABLE releases ADD COLUMN capability_version_id TEXT REFERENCES capability_versions(id) ON DELETE CASCADE;
CREATE INDEX idx_releases_capability_version_id ON releases(capability_version_id);

-- Backfill capability_version_id from the legacy pair. The pair
-- (capability_id, capability_version) is unique, so the lookup is
-- unambiguous.
UPDATE releases SET capability_version_id = (
  SELECT cv.id FROM capability_versions cv
   WHERE cv.capability_id = releases.capability_id
     AND cv.version       = releases.capability_version
);

-- Defensive: every release must now have a non-null
-- capability_version_id.
DELETE FROM releases WHERE capability_version_id IS NULL;

-- ---------------------------------------------------------------------------
-- Drop unused guardrail tables
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS guardrail_violations;
DROP TABLE IF EXISTS guardrail_rules;
