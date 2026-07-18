-- Migration 025: harness-engineering surface (datasets, preconditions, evals).
--
-- Per the OpenAI Harness Engineering article, evals are the new unit
-- tests for LLM work. This migration introduces the storage layer for:
--
--   - Datasets (test inputs + expected outputs) attached to a Capability
--   - Preconditions (named command hooks) attached to a Capability
--   - Eval runs against a Release (the canonical "is this version OK?" loop)
--   - Per-case eval results within a run
--
-- Forward-only. Existing rows are untouched.

CREATE TABLE IF NOT EXISTS datasets (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_datasets_capability ON datasets(capability_id);

CREATE TABLE IF NOT EXISTS dataset_cases (
    id TEXT PRIMARY KEY,
    dataset_id TEXT NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    inputs TEXT NOT NULL DEFAULT '{}',
    expected TEXT NOT NULL DEFAULT '{}',
    description TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dataset_cases_dataset ON dataset_cases(dataset_id, seq);

CREATE TABLE IF NOT EXISTS preconditions (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    timeout_sec INTEGER NOT NULL DEFAULT 60,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_preconditions_capability ON preconditions(capability_id);

CREATE TABLE IF NOT EXISTS eval_runs (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    dataset_id TEXT NOT NULL,
    scorer TEXT NOT NULL,
    score REAL NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    started_at DATETIME NOT NULL,
    finished_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_eval_runs_release ON eval_runs(release_id, started_at DESC);

CREATE TABLE IF NOT EXISTS eval_results (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    actual TEXT NOT NULL DEFAULT '{}',
    error TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_eval_results_run ON eval_results(run_id, seq);
