-- 025_user_org_team.up.sql
-- Multi-tenant foundation: users, orgs, teams, memberships.
-- Workspaces already belong to a single org via implicit relationship
-- (workspaces.org_id is added in 026 below).
--
-- Roles:
--   admin:    full org control, can manage members + workspaces
--   approver: can approve manifest reviews
--   editor:   can create + edit capabilities
--   viewer:   read-only
--
-- FK strategy: ON DELETE CASCADE for child rows; org deletion removes
-- everything tied to it. Workspace.org_id is added in 026.

CREATE TABLE orgs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE(org_id, name)
);

CREATE INDEX idx_teams_org ON teams(org_id);

CREATE TABLE org_members (
    org_id     TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'approver', 'editor', 'viewer')),
    joined_at  TEXT NOT NULL,
    PRIMARY KEY (org_id, user_id)
);

CREATE INDEX idx_org_members_user ON org_members(user_id);

CREATE TABLE team_members (
    team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at  TEXT NOT NULL,
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user ON team_members(user_id);

-- 026: Add workspaces.org_id for tenant isolation.
-- Existing rows get an empty-string placeholder; backfill is the
-- responsibility of the cutover script (Phase 9 wiring).
ALTER TABLE workspaces ADD COLUMN org_id TEXT DEFAULT '';
CREATE INDEX idx_workspaces_org ON workspaces(org_id);

-- 027: User email must be unique per org (allow same email across orgs).
-- Add org_id column with default 'legacy' (backfilled for existing rows).
-- Add org-scoped UNIQUE index on (org_id, email).
INSERT INTO orgs (id, name, slug, created_at, updated_at)
  VALUES ('legacy', 'Legacy', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
  ON CONFLICT DO NOTHING;
ALTER TABLE users ADD COLUMN org_id TEXT NOT NULL DEFAULT 'legacy';
CREATE UNIQUE INDEX idx_users_org_email ON users(org_id, email);