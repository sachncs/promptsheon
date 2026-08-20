-- 034_release_state_machine.up.sql
-- Phase 1.17: extend Release.status with the full 6-state machine
-- (draft / review / approved / canary / active / rolled_back).
-- Existing rows are mapped to the closest new state:
--   pending    -> review
--   superseded -> rolled_back
--   rejected   -> rolled_back
--   active     -> active
--
-- 'superseded' and 'rejected' remain valid (legacy) so the
-- schema doesn't break historical views, but every code path
-- post-migration only writes the 6 new states.

PRAGMA foreign_keys = off;

CREATE TABLE releases_new (
    id                    TEXT PRIMARY KEY,
    capability_id         TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    capability_version    INTEGER NOT NULL,
    capability_version_id  TEXT REFERENCES capability_versions(id) ON DELETE CASCADE,
    manifest              TEXT NOT NULL DEFAULT '{}',
    environment           TEXT NOT NULL
                             CHECK (environment IN ('dev','staging','prod')),
    status                TEXT NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('draft', 'review', 'approved', 'canary',
                                               'active', 'rolled_back',
                                               'superseded', 'rejected')),
    approved_by           TEXT NOT NULL DEFAULT '[]',
    superseded_by         TEXT REFERENCES releases_new(id) ON DELETE SET NULL,
    replaces_release_id   TEXT REFERENCES releases_new(id) ON DELETE SET NULL,
    created_at            DATETIME NOT NULL,
    created_by            TEXT NOT NULL DEFAULT '',
    activated_at          DATETIME,
    superseded_at         DATETIME,
    canary_percent        INTEGER NOT NULL DEFAULT 0
                             CHECK (canary_percent >= 0 AND canary_percent <= 100),
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO releases_new (
    id, capability_id, capability_version, capability_version_id, manifest,
    environment, status, approved_by, superseded_by, replaces_release_id,
    created_at, created_by, activated_at, superseded_at, canary_percent, updated_at
)
SELECT
    id, capability_id, capability_version, capability_version_id, manifest,
    environment,
    CASE status
        WHEN 'pending'    THEN 'review'
        WHEN 'superseded' THEN 'rolled_back'
        WHEN 'rejected'   THEN 'rolled_back'
        ELSE status
    END,
    approved_by, NULL, replaces_release_id,
    created_at, created_by, activated_at, superseded_at, canary_percent,
    CURRENT_TIMESTAMP
FROM releases;

DROP TABLE releases;
ALTER TABLE releases_new RENAME TO releases;

CREATE INDEX idx_releases_capability                  ON releases(capability_id);
CREATE INDEX idx_releases_environment_status           ON releases(environment, status);
CREATE INDEX idx_releases_status                        ON releases(status);
CREATE INDEX idx_releases_capability_recent             ON releases(capability_id, created_at DESC);

PRAGMA foreign_keys = on;

-- Append-only transition log so the audit chain can replay the
-- lifecycle of every release.
CREATE TABLE release_transitions (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    from_status TEXT CHECK (from_status IN ('draft', 'review', 'approved', 'canary',
                                          'active', 'rolled_back')),
    to_status   TEXT NOT NULL CHECK (to_status IN ('draft', 'review', 'approved',
                                                  'canary', 'active', 'rolled_back')),
    actor_id    TEXT NOT NULL REFERENCES users(id),
    reason      TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_release_transitions_release ON release_transitions(release_id);
