-- Migration 056 (lineage edges): persist the capability
-- lineage graph. The Graph value type lives in
-- internal/lineage/lineage.go; the schema lives here.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * audit_chain_state layout
--   * harness tables
--   * provider_keys
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts
--
-- lineage_edges stores one row per parent→child transition.
-- The source is "recommendation" (auto-adopted) or "manual"
-- (engineer-applied). recommendation_id is set for the former
-- only.

CREATE TABLE lineage_edges (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    capability_id    TEXT    NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    parent           TEXT    NOT NULL,
    child            TEXT    NOT NULL,
    source           TEXT    NOT NULL CHECK (source IN ('recommendation', 'manual', 'migration')),
    recommendation_id TEXT,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by       TEXT NOT NULL DEFAULT '',
    notes            TEXT NOT NULL DEFAULT '{}',
    UNIQUE (capability_id, parent, child)
);
CREATE INDEX idx_lineage_edges_capability_id ON lineage_edges(capability_id);
CREATE INDEX idx_lineage_edges_parent         ON lineage_edges(parent);
CREATE INDEX idx_lineage_edges_child          ON lineage_edges(child);
