-- Postgres migration 024: releases + approvals schema.
--
-- Mirrors internal/store/migrations/024_releases.sql with the
-- dialects adjusted for Postgres (TIMESTAMPTZ, JSONB).

CREATE TABLE IF NOT EXISTS releases (
    id                  TEXT PRIMARY KEY,
    capability_id       TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version  INTEGER NOT NULL,
    manifest            JSONB NOT NULL DEFAULT '{}'::jsonb,
    environment         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    approved_by         JSONB NOT NULL DEFAULT '[]'::jsonb,
    superseded_by       TEXT NOT NULL DEFAULT '',
    replaces_release_id TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    created_by          TEXT NOT NULL DEFAULT '',
    activated_at        TIMESTAMPTZ,
    superseded_at       TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_releases_capability ON releases(capability_id);
CREATE INDEX IF NOT EXISTS idx_releases_environment_status ON releases(environment, status);
CREATE INDEX IF NOT EXISTS idx_releases_status ON releases(status);

CREATE TABLE IF NOT EXISTS approvals (
    release_id  TEXT PRIMARY KEY REFERENCES releases(id) ON DELETE CASCADE,
    votes       JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL
);
