-- 048_prompt_scans.up.sql
-- T2-3 prompt-security scanner results. Every save of a manifest
-- (or any other user-authored content) writes one scan row that
-- captures the verdict + findings list. The scanner lives in
-- packages/server/src/security/prompt-scanner.ts; this table is
-- its persistence.

CREATE TABLE prompt_scans (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    actor_id TEXT,
    resource_kind TEXT NOT NULL,             -- manifest | plan | skill | ...
    resource_id TEXT NOT NULL,
    verdict TEXT NOT NULL,                   -- clean | warn | block
    findings_count INTEGER NOT NULL DEFAULT 0,
    findings TEXT NOT NULL DEFAULT '[]',     -- JSON array of Finding
    created_at TEXT NOT NULL,
    FOREIGN KEY (organization_id) REFERENCES orgs(id) ON DELETE CASCADE
);

CREATE INDEX idx_prompt_scans_org ON prompt_scans(organization_id, created_at DESC);
CREATE INDEX idx_prompt_scans_resource ON prompt_scans(resource_kind, resource_id, created_at);
