-- 039_experiments.up.sql
-- Phase 2.9: experiment tracking. A/B-style variants attached to
-- a release so the eval engine can compare two configurations
-- side-by-side. Each run produces a resistance + per-variant
-- score.

CREATE TABLE experiment_variants (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    config TEXT NOT NULL,
    weight REAL NOT NULL DEFAULT 0.5
        CHECK (weight >= 0 AND weight <= 1),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (release_id, label)
);

CREATE INDEX idx_experiment_variants_release ON experiment_variants(release_id);

CREATE TABLE experiment_assignments (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES experiment_variants(id) ON DELETE CASCADE,
    case_id TEXT NOT NULL,
    variant_id TEXT NOT NULL REFERENCES experiment_variants(id) ON DELETE CASCADE,
    outcome TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_experiment_assignments_experiment ON experiment_assignments(experiment_id);

-- Side-by-side summary view materialised by the API endpoint
-- (computed in code rather than SQL because Postgres-only FTS
-- features don't all exist in SQLite).
