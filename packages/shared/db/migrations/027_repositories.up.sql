-- 027_repositories.up.sql
-- Phase 1.1: introduce repositories (auto-promoted from legacy projects).
-- Default branch 'main' is created on insert by application code.

CREATE TABLE repositories (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    default_branch TEXT NOT NULL DEFAULT 'main',
    visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private', 'internal', 'public')),
    min_approvers INTEGER NOT NULL DEFAULT 1
        CHECK (min_approvers >= 0),
    require_signed_releases INTEGER NOT NULL DEFAULT 0
        CHECK (require_signed_releases IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (workspace_id, slug)
);

CREATE INDEX idx_repositories_workspace ON repositories(workspace_id);

-- Auto-promote legacy project rows. project ids survive as repo ids
-- so downstream tables (capabilities.repo_id, etc.) keep their shape.
INSERT INTO repositories (
    id, workspace_id, name, slug, description,
    default_branch, visibility, min_approvers, require_signed_releases,
    created_at, updated_at
)
SELECT
    id,
    workspace_id,
    name,
    LOWER(REPLACE(name, ' ', '-')),
    description,
    'main',
    'private',
    1,
    0,
    created_at,
    updated_at
FROM projects;
