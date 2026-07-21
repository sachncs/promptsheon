-- Migration 058 down: drop the FK by rebuilding the table without it.
PRAGMA foreign_keys = OFF;
CREATE TABLE api_keys_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    key_prefix  TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL DEFAULT 'reader',
    expires_at  DATETIME,
    last_used   DATETIME,
    created_at  DATETIME NOT NULL,
    revoked     INTEGER NOT NULL DEFAULT 0
);
INSERT INTO api_keys_old
    SELECT id, user_id, name, key_hash, key_prefix, role, expires_at, last_used, created_at, revoked
    FROM api_keys;
DROP TABLE api_keys;
ALTER TABLE api_keys_old RENAME TO api_keys;
PRAGMA foreign_keys = ON;
