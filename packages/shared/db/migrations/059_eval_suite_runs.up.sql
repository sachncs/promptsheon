CREATE TABLE eval_suite_runs (
    id TEXT PRIMARY KEY,
    suite_id TEXT NOT NULL REFERENCES eval_suites(id) ON DELETE CASCADE,
    suite_version_id TEXT NOT NULL REFERENCES eval_suite_versions(id) ON DELETE RESTRICT,
    n INTEGER NOT NULL,
    k INTEGER NOT NULL,
    pass_at_k REAL NOT NULL DEFAULT 0,
    raw_score REAL NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    borderline_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    error TEXT
);

CREATE TABLE eval_suite_trial_results (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES eval_suite_runs(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    case_id TEXT NOT NULL,
    passed INTEGER NOT NULL,
    weighted_score REAL NOT NULL,
    trial_json TEXT NOT NULL,
    grader_result_json TEXT NOT NULL,
    UNIQUE (run_id, seq)
);

CREATE INDEX idx_eval_suite_runs_suite ON eval_suite_runs(suite_id, started_at DESC);
CREATE INDEX idx_eval_suite_trial_results_run ON eval_suite_trial_results(run_id, seq);
