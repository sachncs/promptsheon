-- Output snapshots for LLM input/output pairs
CREATE TABLE IF NOT EXISTS output_snapshots (
    id TEXT PRIMARY KEY,
    prompt_hash TEXT NOT NULL,
    prompt_text TEXT NOT NULL,
    model TEXT NOT NULL,
    response_text TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    token_usage TEXT NOT NULL DEFAULT '{}',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    hallucination_score REAL NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snapshots_prompt ON output_snapshots(prompt_hash);
CREATE INDEX IF NOT EXISTS idx_snapshots_model ON output_snapshots(model);
CREATE INDEX IF NOT EXISTS idx_snapshots_created ON output_snapshots(created_at);
