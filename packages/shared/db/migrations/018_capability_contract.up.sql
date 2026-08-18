-- 018: capability contract + reputation source data.
--
-- Stores the typed CapabilityContract attached to a
-- Capability: input/output schema, SLO target, blast radius.
-- The contract is the unit of governance: a Capability
-- without a contract cannot be auto-promoted.
--
-- Reputation is derived from existing tables (executions,
-- eval_results, decisions) and is NOT persisted here; the
-- Reputation view aggregates on demand. Adding a column for
-- reputation would risk drift from the underlying sources.

CREATE TABLE capability_contracts (
    capability_id      TEXT PRIMARY KEY REFERENCES capabilities(id) ON DELETE CASCADE,
    blast_radius       TEXT NOT NULL,
    success_rubric     TEXT NOT NULL DEFAULT '',
    auto_promotable    INTEGER NOT NULL DEFAULT 0,
    input_schema       TEXT NOT NULL DEFAULT '{}',
    output_schema      TEXT NOT NULL DEFAULT '{}',
    slo_max_p95_ms     INTEGER NOT NULL DEFAULT 0,
    slo_min_success    REAL NOT NULL DEFAULT 0,
    slo_max_hallu      REAL NOT NULL DEFAULT 0,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_capability_contracts_blast ON capability_contracts(blast_radius);
