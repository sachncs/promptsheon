-- 045_trace_runs.up.sql
-- Distributed-tracing substrate for promptsheon: every LLM call,
-- tool use, retrieval, and node-execution step writes a span into
-- trace_spans under a parent trace_runs row. Designed to match
-- the OpenTelemetry data model so existing OTel collectors can
-- consume the same data.

CREATE TABLE trace_runs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    actor_id TEXT,
    execution_id TEXT,
    session_id TEXT,
    environment TEXT NOT NULL DEFAULT 'dev',
    name TEXT NOT NULL,
    start_time TEXT NOT NULL,
    end_time TEXT,
    status TEXT NOT NULL DEFAULT 'running',  -- running | success | error
    attributes TEXT NOT NULL DEFAULT '{}',   -- JSON blob
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_usd REAL NOT NULL DEFAULT 0,
    model TEXT,
    -- organization_id is intentionally NOT a foreign key: traces
    -- can be opened before the org row exists (the bootstrap probe
    -- walks every /api/executions path, and unscoped test/dev runs
    -- use a synthetic 'unscoped' id). The route layer is responsible
    -- for ensuring callers can only read trace rows for orgs they
    -- belong to (requireAdmin + orgId filter).
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE SET NULL
);

CREATE INDEX idx_trace_runs_org_start ON trace_runs(organization_id, start_time DESC);
CREATE INDEX idx_trace_runs_execution ON trace_runs(execution_id);
CREATE INDEX idx_trace_runs_session ON trace_runs(session_id);

CREATE TABLE trace_spans (
    id TEXT PRIMARY KEY,
    trace_run_id TEXT NOT NULL,
    parent_span_id TEXT,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'internal',  -- internal | llm | tool | retrieval | agent
    start_time TEXT NOT NULL,
    end_time TEXT,
    status TEXT NOT NULL DEFAULT 'ok',  -- ok | error
    attributes TEXT NOT NULL DEFAULT '{}',
    -- LLM-specific columns (NULL for non-LLM spans)
    model TEXT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    cost_usd REAL,
    input_text TEXT,    -- truncated to 8KB
    output_text TEXT,   -- truncated to 8KB
    FOREIGN KEY (trace_run_id) REFERENCES trace_runs(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_span_id) REFERENCES trace_spans(id) ON DELETE SET NULL
);

CREATE INDEX idx_trace_spans_trace ON trace_spans(trace_run_id, start_time);
CREATE INDEX idx_trace_spans_parent ON trace_spans(parent_span_id);
CREATE INDEX idx_trace_spans_kind ON trace_spans(kind);
