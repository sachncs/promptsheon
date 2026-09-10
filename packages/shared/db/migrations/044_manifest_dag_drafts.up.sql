-- 044_manifest_dag_drafts.up.sql
-- Capability drafts saved via POST /api/manifests can be
-- authored without yet being attached to a capabilities row —
-- e.g. the DAG editor's "Save" before the user has picked a
-- capability from the picker. Make capability_id nullable
-- and drop the FK so drafts can be persisted.

PRAGMA foreign_keys = OFF;

CREATE TABLE manifest_dag_draft AS SELECT * FROM manifest_dag WHERE 0;
DROP TABLE manifest_dag;
CREATE TABLE manifest_dag (
    id TEXT PRIMARY KEY,
    capability_id TEXT,
    version INTEGER NOT NULL,
    manifest_hash TEXT NOT NULL UNIQUE,
    parent_manifest_hash TEXT,
    goal TEXT NOT NULL DEFAULT '',
    goal_metrics TEXT NOT NULL DEFAULT '{}',
    manifest_json TEXT NOT NULL,
    approved_by TEXT NOT NULL DEFAULT '',
    approved_at TEXT,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, parent_manifest_hash, goal, goal_metrics, manifest_json, approved_by, approved_at, created_by, created_at)
  SELECT id, capability_id, version, manifest_hash, parent_manifest_hash, goal, goal_metrics, manifest_json, approved_by, approved_at, created_by, created_at
    FROM manifest_dag_draft;
DROP TABLE manifest_dag_draft;

PRAGMA foreign_keys = ON;
CREATE INDEX idx_manifest_dag_capability ON manifest_dag(capability_id);
CREATE INDEX idx_manifest_dag_hash ON manifest_dag(manifest_hash);
CREATE INDEX idx_manifest_dag_parent ON manifest_dag(parent_manifest_hash);

