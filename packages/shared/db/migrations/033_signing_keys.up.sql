-- 033_signing_keys.up.sql
-- Phase 1.15: per-organisation ed25519 public key registry. The
-- private key never leaves the operator; signatures are produced
-- off-band and posted via /api/commits/:oid/sign.

CREATE TABLE signing_keys (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    public_key_pem TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deactivated_at DATETIME,
    UNIQUE (organization_id, fingerprint)
);

CREATE INDEX idx_signing_keys_org ON signing_keys(organization_id);
