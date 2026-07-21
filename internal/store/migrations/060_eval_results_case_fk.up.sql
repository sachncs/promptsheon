-- Migration 060 (eval_results case_id FK): add a foreign key
-- from eval_results.case_id to dataset_cases.id. ON DELETE SET
-- NULL keeps historical results readable even when a case is
-- removed from its parent dataset.
--
-- DO NOT MODIFY under this migration (or any subsequent):
--   * audit chain format
--   * audit_chain_state layout
--   * releases.status enum
--   * OpenAI / Anthropic provider contracts

PRAGMA foreign_keys = OFF;

CREATE TABLE eval_results_new (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
    case_id    TEXT REFERENCES dataset_cases(id) ON DELETE SET NULL,
    seq        INTEGER NOT NULL,
    passed     INTEGER NOT NULL DEFAULT 0 CHECK (passed IN (0, 1)),
    actual     TEXT NOT NULL DEFAULT '{}',
    error      TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0
);
INSERT INTO eval_results_new
    SELECT id, run_id, case_id, seq, passed, actual, error, latency_ms
      FROM eval_results;
DROP TABLE eval_results;
ALTER TABLE eval_results_new RENAME TO eval_results;
CREATE INDEX idx_eval_results_run ON eval_results(run_id, seq);

PRAGMA foreign_keys = ON;
