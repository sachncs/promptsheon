-- 037_vault_keyring.up.sql
-- Phase 7: a keyring records every encryption key version. The
-- vault encrypts with the active key and the reader falls back to
-- older key versions so rotation never invalidates ciphertext.

CREATE TABLE vault_keyring (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    ciphertext TEXT,
    active INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    rotated_at DATETIME
);

-- Single-row invariant: there is at most one active key. Enforced
-- via a partial UNIQUE index over the boolean 'active' column.
CREATE UNIQUE INDEX idx_vault_keyring_active
    ON vault_keyring(active) WHERE active = 1;

-- A key for the dev environment is pre-installed by the bootstrap
-- path. Production deployments register their first key through
-- /api/vault/keys.

INSERT INTO vault_keyring (label, fingerprint, ciphertext, active, created_at)
VALUES ('dev', 'dev-key-2026', 'aes-256-gcm-static-dev-key', 1, CURRENT_TIMESTAMP);
