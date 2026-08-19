-- 022_manifest_dag.up.sql
-- Manifest DAG persistence for the multi-agent capability platform.
--
-- The Manifest type (9 components + DAG) was added in phase 1.1. This
-- migration adds the storage tables. existing capability_versions.manifest
-- TEXT column is preserved for legacy fallback during cutover.
--
-- CAS-style uniqueness: manifest_hash is globally unique. The same
-- manifest content (same nodes/edges/tools/...) produces the same hash,
-- so we never store duplicates.
--
-- DAG structure: manifest_nodes holds per-node overrides (goal, dependsOn,
-- guardrails, observability, hooks, retry, conversationManager, state,
-- limits). manifest_edges holds the directed graph. manifest_dag holds the
-- top-level container per capability version.
--
-- lineage: parent_manifest_hash points to the manifest this one was
-- derived from (for goal-based evolution). nullable NULL for the original.

CREATE TABLE manifest_dag (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    manifest_hash TEXT NOT NULL UNIQUE,
    parent_manifest_hash TEXT,
    goal TEXT NOT NULL DEFAULT '',
    goal_metrics TEXT NOT NULL DEFAULT '{}',
    manifest_json TEXT NOT NULL,
    approved_by TEXT NOT NULL DEFAULT '',
    approved_at TEXT,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(capability_id, version),
    FOREIGN KEY (capability_id) REFERENCES capabilities(id) ON DELETE CASCADE
);

CREATE INDEX idx_manifest_dag_capability ON manifest_dag(capability_id);
CREATE INDEX idx_manifest_dag_hash ON manifest_dag(manifest_hash);
CREATE INDEX idx_manifest_dag_parent ON manifest_dag(parent_manifest_hash);

CREATE TABLE manifest_nodes (
    id TEXT PRIMARY KEY,
    manifest_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    goal TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    depends_on TEXT NOT NULL DEFAULT '[]',
    pre_guardrails TEXT NOT NULL DEFAULT '[]',
    post_guardrails TEXT NOT NULL DEFAULT '[]',
    observability TEXT NOT NULL DEFAULT '{}',
    hooks TEXT NOT NULL DEFAULT '{}',
    retry TEXT NOT NULL DEFAULT '{}',
    conversation_manager TEXT NOT NULL DEFAULT '{}',
    state TEXT NOT NULL DEFAULT '{}',
    storage TEXT NOT NULL DEFAULT '{}',
    limits TEXT NOT NULL DEFAULT '{}',
    UNIQUE(manifest_id, node_id),
    FOREIGN KEY (manifest_id) REFERENCES manifest_dag(id) ON DELETE CASCADE
);

CREATE INDEX idx_manifest_nodes_manifest ON manifest_nodes(manifest_id);

CREATE TABLE manifest_edges (
    id TEXT PRIMARY KEY,
    manifest_id TEXT NOT NULL,
    from_node TEXT NOT NULL,
    to_node TEXT NOT NULL,
    field_mapping TEXT NOT NULL DEFAULT '{}',
    UNIQUE(manifest_id, from_node, to_node),
    FOREIGN KEY (manifest_id) REFERENCES manifest_dag(id) ON DELETE CASCADE
);

CREATE INDEX idx_manifest_edges_manifest ON manifest_edges(manifest_id);

CREATE TABLE manifest_approvals (
    id TEXT PRIMARY KEY,
    manifest_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    vote TEXT NOT NULL CHECK(vote IN ('approve', 'reject')),
    comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(manifest_id, user_id),
    FOREIGN KEY (manifest_id) REFERENCES manifest_dag(id) ON DELETE CASCADE
);

CREATE INDEX idx_manifest_approvals_manifest ON manifest_approvals(manifest_id);

-- node_runs: per-execution telemetry per node. One row per node visit
-- during a Manifest execution.
CREATE TABLE node_runs (
    id TEXT PRIMARY KEY,
    manifest_hash TEXT NOT NULL,
    node_id TEXT NOT NULL,
    execution_id TEXT,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    latency_ms TEXT,
    cost_usd REAL DEFAULT 0,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending'
);

CREATE INDEX idx_node_runs_execution ON node_runs(execution_id);
CREATE INDEX idx_node_runs_manifest ON node_runs(manifest_hash);