-- Migration 058 (api_keys FK): add foreign key on api_keys.user_id
-- so deleting a user cascades to their keys.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts
--
-- Migration 043 added the FK in a transactional rebuild dance
-- but only on tables with foreign keys to users (audit_entries
-- etc.). api_keys was missed. Migration 058 adds the FK now
-- that UpdateUser/DeleteUser revoke keys transactionally (SEC-6).

PRAGMA foreign_keys = OFF;

CREATE TABLE api_keys_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    key_prefix  TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL DEFAULT 'reader',
    expires_at  DATETIME,
    last_used   DATETIME,
    created_at  DATETIME NOT NULL,
    revoked     INTEGER NOT NULL DEFAULT 0
);

INSERT INTO api_keys_new
    SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
    FROM api_keys;

DROP TABLE api_keys;
ALTER TABLE api_keys_new RENAME TO api_keys;
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

PRAGMA foreign_keys = ON;
