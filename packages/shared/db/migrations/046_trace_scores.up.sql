-- 046_trace_scores.up.sql
-- Eval-result rows attached to a trace_run. Auto-eval runs
-- after every execution when /api/eval/auto-eval runs; the
-- evaluator library reads the trace's spans + the run's
-- attributes and writes a row per (run, evaluator).

CREATE TABLE trace_scores (
    id TEXT PRIMARY KEY,
    trace_run_id TEXT NOT NULL,
    execution_id TEXT,
    evaluator TEXT NOT NULL,
    name TEXT NOT NULL,
    value REAL,
    label TEXT,
    rationale TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (trace_run_id) REFERENCES trace_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_trace_scores_trace ON trace_scores(trace_run_id, created_at);
CREATE INDEX idx_trace_scores_evaluator ON trace_scores(evaluator, name);
