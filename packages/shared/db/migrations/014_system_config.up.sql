-- 014: system_config (operator-tunable runtime settings).
--
-- The settings layer lets operators tune OTel / LLM / observability
-- settings at runtime without a daemon restart. Storage is
-- general-purpose key/value; the value is JSON-encoded so each
-- key carries structured data without a schema migration per
-- addition. updated_by records who made the change (user id or
-- "system" at boot).
--
-- Precedence (defined in internal/settings/resolver.go):
--   hardcoded default < env var / YAML < DB row
-- The DB row is the runtime ceiling; deleting it reasserts the
-- env-default. PROMPTSHEON_SETTINGS_MODE=env-only disables writes
-- (operator can read but not change); the mutable default
-- (covered by A1) ships as the production behavior.
--
-- updated_at is indexed so the periodic cleanup (or a future
-- janitor goroutine) can prune stale rows in O(rows-scanned)
-- without scanning the whole table.

CREATE TABLE IF NOT EXISTS system_config (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,                -- JSON-encoded
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by TEXT                            -- user id or "system"
);

CREATE INDEX IF NOT EXISTS system_config_updated_at_idx
    ON system_config(updated_at);
