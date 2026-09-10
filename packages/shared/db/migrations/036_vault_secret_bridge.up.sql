-- 036_vault_secret_bridge.up.sql
-- Phase 3.1: vault_secrets table — resolvable reference store.
-- Replaces raw API-key storage in system_config with a
-- $vault://<org>/<name> lookup table.

CREATE TABLE vault_secrets (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ciphertext TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    rotated_at DATETIME,
    UNIQUE (organization_id, name)
);

CREATE INDEX idx_vault_secrets_org ON vault_secrets(organization_id);

-- Audit-export snapshots — tamper-evident tarballs.
CREATE TABLE org_exports (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by TEXT NOT NULL REFERENCES users(id),
    row_count TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    blob_path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_org_exports_org ON org_exports(organization_id);

-- Cost rollups — populated by the analytics job.
CREATE TABLE capability_cost_rollups (
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    day TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_micros INTEGER NOT NULL DEFAULT 0,
    executions INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (capability_id, day)
);

CREATE INDEX idx_cost_rollups_day ON capability_cost_rollups(day);

-- FTS5 search index over manifests and audit entries.
CREATE VIRTUAL TABLE search_index USING fts5(
    kind UNINDEXED,
    resource_id UNINDEXED,
    title,
    body,
    tokenize = 'unicode61 remove_diacritics 2'
);
