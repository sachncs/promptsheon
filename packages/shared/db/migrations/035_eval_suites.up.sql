-- 035_eval_suites.up.sql
-- Phase 2.1: EvalSuite + EvalSuiteVersion tables. Suites carry
-- grader configuration per version; an EvalRun targets a
-- specific version so historical reruns are reproducible.

CREATE TABLE eval_suites (
    id TEXT PRIMARY KEY,
    capability_id TEXT NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
    repository_id TEXT REFERENCES repositories(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT,
    current_version INTEGER NOT NULL DEFAULT 1,
    pass_threshold REAL NOT NULL DEFAULT 0.92,
    borderline_band REAL NOT NULL DEFAULT 0.05,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (capability_id, name)
);

CREATE TABLE eval_suite_versions (
    id TEXT PRIMARY KEY,
    suite_id TEXT NOT NULL REFERENCES eval_suites(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    grader_config TEXT NOT NULL DEFAULT '[]',
    pass_threshold REAL NOT NULL DEFAULT 0.92,
    borderline_band REAL NOT NULL DEFAULT 0.05,
    k INTEGER NOT NULL DEFAULT 1,
    n INTEGER NOT NULL DEFAULT 1,
    notes TEXT,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (suite_id, version)
);

CREATE TABLE human_review_queue (
    id TEXT PRIMARY KEY,
    case_id TEXT NOT NULL,
    suite_id TEXT NOT NULL REFERENCES eval_suites(id) ON DELETE CASCADE,
    suite_run_id TEXT,
    submitted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewer_id TEXT REFERENCES users(id),
    decided_at DATETIME,
    decision TEXT CHECK (decision IN ('approve', 'reject')),
    notes TEXT
);

CREATE INDEX idx_eval_suites_capability ON eval_suites(capability_id);
CREATE INDEX idx_eval_suite_versions_suite ON eval_suite_versions(suite_id);
CREATE INDEX idx_human_review_queue_suite ON human_review_queue(suite_id);
