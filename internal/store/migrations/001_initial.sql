-- Promptsheon schema v1
-- Stores metadata for prompts, agents, datasets, evaluations, audit, and reviews.

CREATE TABLE IF NOT EXISTS prompts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    content TEXT NOT NULL,
    variables TEXT DEFAULT '[]',
    tags TEXT DEFAULT '[]',
    model_hint TEXT DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    cas_hash TEXT DEFAULT '',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    metadata TEXT DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    steps TEXT NOT NULL DEFAULT '[]',
    tools TEXT DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'draft',
    cas_hash TEXT DEFAULT '',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS test_datasets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cases TEXT NOT NULL DEFAULT '[]',
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS eval_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    test_case_id TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,
    model TEXT DEFAULT '',
    dataset_id TEXT DEFAULT '',
    output TEXT DEFAULT '',
    score REAL DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    token_usage TEXT DEFAULT '{}',
    hallucination_score REAL DEFAULT 0,
    passed INTEGER DEFAULT 0,
    error TEXT DEFAULT '',
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    details TEXT DEFAULT '{}',
    timestamp DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS reviews (
    id TEXT PRIMARY KEY,
    resource_id TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    author TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    comments TEXT DEFAULT '[]',
    created_at DATETIME NOT NULL,
    resolved_at DATETIME
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'reader',
    expires_at DATETIME,
    last_used DATETIME,
    created_at DATETIME NOT NULL,
    revoked INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_prompts_status ON prompts(status);
CREATE INDEX IF NOT EXISTS idx_prompts_name ON prompts(name);
CREATE INDEX IF NOT EXISTS idx_eval_results_hash ON eval_results(prompt_hash);
CREATE INDEX IF NOT EXISTS idx_eval_results_dataset ON eval_results(dataset_id);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_entries(resource);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_entries(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_reviews_resource ON reviews(resource_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
