-- 051_identity.up.sql
-- AG-4: Agent Identity (apikeys + SVIDs, opt-in).
--
-- The agent_identities table is the per-agent credential store.
-- A row is created at agent registration time (AG-1.5) and updated
-- when the agent rotates its credential. The `mode` column picks
-- which verifier (apikey or SVID) checks the credential.
--
-- apikeys are stored as sha256(token). The raw token is never
-- persisted; the bootstrap path returns the token once and the
-- agent is expected to persist it locally.
--
-- svid_revocations is a fast-fail list checked by the SVID
-- verifier before signature validation. Entries are added
-- via DELETE /api/identity/:id and removed via admin tooling;
-- the verifier's hot path does an O(1) lookup on the table.

CREATE TABLE agent_identities (
    id              TEXT PRIMARY KEY,
    agent_id        TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    mode            TEXT NOT NULL CHECK (mode IN ('apikey', 'svid')),
    -- apikeys: sha256(token). svids: payload signed by the
    -- operator signing key. The verifier knows which column to
    -- read by `mode`.
    credential      TEXT NOT NULL,
    scope           TEXT NOT NULL DEFAULT '{}',
    issued_at       TEXT NOT NULL,
    expires_at      TEXT NOT NULL,
    revoked_at      TEXT,
    UNIQUE (agent_id, mode)
);

CREATE INDEX idx_agent_identities_agent ON agent_identities(agent_id);
CREATE INDEX idx_agent_identities_org ON agent_identities(organization_id);

-- The svid_revocations table is a separate fast-path lookup
-- keyed by SVID id. The verifier short-circuits with 401 if
-- the SVID id is in this table, even if the signature is valid.
CREATE TABLE svid_revocations (
    svid_id     TEXT PRIMARY KEY,
    agent_id    TEXT NOT NULL,
    revoked_at  TEXT NOT NULL,
    reason      TEXT
);

CREATE INDEX idx_svid_revocations_agent ON svid_revocations(agent_id);