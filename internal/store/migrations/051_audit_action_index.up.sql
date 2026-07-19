-- Migration 051 (audit user+action+time index): add a composite
-- index for the dominant audit query — "events for user X with
-- action Y in the last hour" — that the existing (user_id,
-- timestamp) index from 047 cannot satisfy efficiently (a btree
-- probe on user_id produces a sorted-by-timestamp list; filtering
-- by action requires scanning the matched rows).
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit_entries.previous_hash / entry_hash / timestamp_str chain format
--   * audit_chain_state single-row layout
--   * harness tables: datasets, dataset_cases, preconditions, eval_runs, eval_results
--   * provider_keys (vault, AES-GCM ciphertext)
--   * releases.status enum and uniq_releases_active_capability_env partial unique index
--   * OpenAI / Anthropic provider contracts
--
-- The index covers the query path explicitly listed in the audit
-- as "show me every audit row that touched user X with action Y
-- in the last N hours". Adding action as the second index
-- column means SQLite's btree probe can seek directly to the
-- matching (user_id, action) prefix and then stream the
-- timestamp range in order.
--
-- The index is additive; the existing idx_audit_user_time
-- (added in 047) stays in place for queries that don't filter
-- by action.

CREATE INDEX IF NOT EXISTS idx_audit_user_action_time
  ON audit_entries (user_id, action, timestamp DESC);
