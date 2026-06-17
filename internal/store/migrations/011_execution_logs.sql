-- 011_execution_logs.sql
-- Execution logs for prompt run tracking

CREATE TABLE IF NOT EXISTS execution_logs (
    id              TEXT PRIMARY KEY,
    prompt_id       TEXT NOT NULL,
    prompt_name     TEXT NOT NULL DEFAULT '',
    prompt_version  INTEGER NOT NULL DEFAULT 1,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'success',
    variables       TEXT,  -- JSON
    system_prompt   TEXT,
    request_messages INTEGER NOT NULL DEFAULT 1,
    prompt_tokens   INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens    INTEGER NOT NULL DEFAULT 0,
    cost_usd        REAL NOT NULL DEFAULT 0.0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    trace_id        TEXT,
    error           TEXT,
    violations      TEXT,  -- JSON array of violation codes
    environment     TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_execution_logs_prompt_id ON execution_logs(prompt_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_status ON execution_logs(status);
CREATE INDEX IF NOT EXISTS idx_execution_logs_created_at ON execution_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_execution_logs_trace_id ON execution_logs(trace_id);
