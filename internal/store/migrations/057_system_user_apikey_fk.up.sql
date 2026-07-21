-- Migration 057 (system user + api key FK): seed the
-- "api" system actor referenced by audit_entries.user_id
-- and add a foreign key from api_keys to users.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts

-- 1. Seed the system user. Audit rows that record an
-- unauthenticated actor (the LLM middleware before any caller
-- context is set) carry user_id="api". Without a corresponding
-- users row, the FK added by 043 rejects the insert.
INSERT OR IGNORE INTO users (id, email, name, role, created_at, updated_at)
VALUES ('api', 'system@promptsheon.internal', 'System Actor', 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- 2. Add FK on api_keys.user_id with ON DELETE CASCADE so
-- deleting a user revokes their keys. Existing rows are
-- coalesced onto the system user if they happen to reference
-- a missing user.
UPDATE api_keys SET user_id = 'api' WHERE user_id != 'api' AND user_id NOT IN (SELECT id FROM users);
