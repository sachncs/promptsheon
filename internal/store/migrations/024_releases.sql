-- Migration 024: releases + approvals schema.
--
-- Forward-only. Per ADR-0010 / ADR-0011 the Release is the approved
-- pointer from a Capability Version to a target Environment. Each
-- Release has zero or one Approval (one row per ReleaseID); the
-- Approval holds the trail of votes against the Release.

CREATE TABLE IF NOT EXISTS releases (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id),
    capability_version INTEGER NOT NULL,
    manifest TEXT NOT NULL DEFAULT '{}',
    environment TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    approved_by TEXT NOT NULL DEFAULT '[]',
    superseded_by TEXT DEFAULT '',
    replaces_release_id TEXT DEFAULT '',
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    activated_at DATETIME,
    superseded_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_releases_capability ON releases(capability_id);
CREATE INDEX IF NOT EXISTS idx_releases_environment_status ON releases(environment, status);
CREATE INDEX IF NOT EXISTS idx_releases_status ON releases(status);

CREATE TABLE IF NOT EXISTS approvals (
    release_id TEXT PRIMARY KEY REFERENCES releases(id) ON DELETE CASCADE,
    votes TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME NOT NULL
);
