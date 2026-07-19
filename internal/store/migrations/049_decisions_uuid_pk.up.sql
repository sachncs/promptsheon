-- Migration 049 (decisions UUID PK): rebuild the decisions table
-- without AUTOINCREMENT. The autoincrement counter is a single
-- hot row that serialises every decision write; at 1k decisions
-- per second realistic load is unaffected, but at 100k+
-- decisions per second the lock becomes a bottleneck.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- The rebuild dance is required because SQLite does not allow
-- changing a column from INTEGER PRIMARY KEY AUTOINCREMENT to
-- TEXT PRIMARY KEY via ALTER TABLE. The migration:
--   1. creates decisions_new with TEXT PRIMARY KEY
--   2. copies every row, generating a random 16-byte hex id
--   3. drops decisions, renames decisions_new
--   4. recreates the index
--
-- The semantic UNIQUE on recommendation_id is preserved: the
-- table still enforces one Decision per Recommendation. Callers
-- that read decisions.id (none in the current codebase; verified
-- by grep) continue to work — the column is just TEXT now.
--
-- The migration is forward-only. The down is a best-effort
-- rebuild back to INTEGER PRIMARY KEY with the same row count;
-- existing callers that depended on the TEXT id type will need
-- a code change to match. Treat down as documentation, not
-- production recovery.

PRAGMA foreign_keys = OFF;

CREATE TABLE decisions_new (
    id                 TEXT    PRIMARY KEY,
    recommendation_id  TEXT    NOT NULL UNIQUE,
    payload            TEXT    NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Copy every existing decision with a freshly generated UUID.
-- The down migration is unlikely to be exercised in production
-- (we run this only once, forward), but a no-op here would leave
-- rows orphaned; the rebuild copies them.
INSERT INTO decisions_new (id, recommendation_id, payload, created_at)
  SELECT lower(hex(randomblob(16))),
         recommendation_id,
         payload,
         created_at
    FROM decisions;

DROP TABLE decisions;
ALTER TABLE decisions_new RENAME TO decisions;
CREATE INDEX idx_decisions_created ON decisions (created_at);

PRAGMA foreign_keys = ON;
