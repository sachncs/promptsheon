CREATE TABLE IF NOT EXISTS eval_runs (
    id TEXT PRIMARY KEY,
    prompt_hash TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    model TEXT NOT NULL,
    status TEXT DEFAULT 'running',
    total_cases INTEGER DEFAULT 0,
    passed_cases INTEGER DEFAULT 0,
    pass_rate REAL DEFAULT 0,
    avg_score REAL DEFAULT 0,
    avg_latency_ms REAL DEFAULT 0,
    avg_hallucination REAL DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    started_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_eval_runs_prompt_hash ON eval_runs(prompt_hash);
CREATE INDEX IF NOT EXISTS idx_eval_runs_dataset_id ON eval_runs(dataset_id);
CREATE INDEX IF NOT EXISTS idx_eval_runs_model ON eval_runs(model);
