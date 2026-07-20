-- Migration 053 (decisions NOT NULL id): tighten the decisions
-- table so id is explicitly NOT NULL. SQLite's TEXT PRIMARY KEY
-- silently allows NULL on insert because TEXT affinity does not
-- coerce NULL to a default; the writer must supply a non-empty
-- id. This migration adds the explicit NOT NULL constraint via
-- the rebuild dance to ensure any legacy NULL id rows surface as
-- a constraint violation rather than a quiet successful insert.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- This migration is idempotent: any DB at version 53 or higher
-- is unchanged by re-running.

PRAGMA foreign_keys = OFF;

CREATE TABLE decisions_new (
    id                 TEXT    PRIMARY KEY NOT NULL,
    recommendation_id  TEXT    NOT NULL UNIQUE,
    payload            TEXT    NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Reject any legacy rows with NULL or empty id before copying.
DELETE FROM decisions WHERE id IS NULL OR id = '';

INSERT INTO decisions_new (id, recommendation_id, payload, created_at)
    SELECT id, recommendation_id, payload, created_at
      FROM decisions;

DROP TABLE decisions;
ALTER TABLE decisions_new RENAME TO decisions;
CREATE INDEX idx_decisions_created ON decisions (created_at);

PRAGMA foreign_keys = ON;
