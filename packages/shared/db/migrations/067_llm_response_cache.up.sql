-- Restart-surviving gateway responses. Prompt text is deliberately excluded;
-- the content hash is sufficient for lookup and avoids persisting user input.

CREATE TABLE llm_response_cache (
    cache_hash TEXT PRIMARY KEY,
    model TEXT NOT NULL,
    temperature REAL NOT NULL,
    provider TEXT NOT NULL,
    base_url TEXT,
    content TEXT NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    cost_usd REAL NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_llm_response_cache_created
    ON llm_response_cache(created_at ASC);
