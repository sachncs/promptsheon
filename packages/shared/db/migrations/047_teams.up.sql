-- 047_teams.up.sql (rev 4)
-- SSO + SCIM provisioning + per-team RBAC. The teams table
-- already exists from migration 025 (user_org_team.up.sql).
-- We add the new columns without renaming so OrgRepo's existing
-- SQL (which uses org_id) keeps working.

ALTER TABLE teams ADD COLUMN organisation_id TEXT;
ALTER TABLE teams ADD COLUMN slug TEXT;
ALTER TABLE teams ADD COLUMN description TEXT;
ALTER TABLE teams ADD COLUMN updated_at TEXT;
ALTER TABLE teams ADD COLUMN id_legacy TEXT;

-- Backfill the new columns from existing rows so legacy data is
-- queryable through the new interface.
UPDATE teams SET organisation_id = org_id WHERE organisation_id IS NULL;
UPDATE teams SET slug = lower(replace(name, ' ', '-')) WHERE slug IS NULL;
UPDATE teams SET description = '' WHERE description IS NULL;
UPDATE teams SET updated_at = created_at WHERE updated_at IS NULL;
UPDATE teams SET id_legacy = id WHERE id_legacy IS NULL;

-- Now we want organisation_id NOT NULL but SQLite ALTER COLUMN
-- can't change nullability in older versions. Use the rebuild
-- recipe: create a new table with the desired shape, copy, drop,
-- rename.
PRAGMA foreign_keys = OFF;

CREATE TABLE teams_new (
    id              TEXT PRIMARY KEY,
    org_id          TEXT NOT NULL,
    name            TEXT NOT NULL,
    organisation_id TEXT NOT NULL,
    slug            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    id_legacy       TEXT,
    FOREIGN KEY (organisation_id) REFERENCES orgs(id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE,
    UNIQUE (organisation_id, slug)
);
INSERT INTO teams_new (id, org_id, name, organisation_id, slug, description, created_at, updated_at, id_legacy)
  SELECT id, org_id, name, organisation_id, slug, description, created_at, updated_at, id_legacy FROM teams;
DROP TABLE teams;
ALTER TABLE teams_new RENAME TO teams;
CREATE INDEX idx_teams_org ON teams(organisation_id);

PRAGMA foreign_keys = ON;

-- T2-2 RBAC: per-team role.
ALTER TABLE team_members ADD COLUMN role TEXT NOT NULL DEFAULT 'member';
ALTER TABLE team_members ADD COLUMN created_at TEXT;

-- SSO configuration per org: which IdP, which client_id, etc.
CREATE TABLE sso_configs (
    organization_id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    issuer TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT 'openid profile email groups',
    audience TEXT,
    groups_claim TEXT NOT NULL DEFAULT 'groups',
    email_claim TEXT NOT NULL DEFAULT 'email',
    name_claim TEXT NOT NULL DEFAULT 'name',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (organization_id) REFERENCES orgs(id) ON DELETE CASCADE
);

-- Provisioned identity: maps an OIDC subject → a promptsheon
-- user row, scoped to the org that owns the IdP.
CREATE TABLE sso_identities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL,
    email TEXT NOT NULL,
    raw_claims TEXT NOT NULL DEFAULT '{}',
    last_login_at TEXT,
    created_at TEXT NOT NULL,
    UNIQUE (provider, issuer, subject),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sso_identities_user ON sso_identities(user_id);
