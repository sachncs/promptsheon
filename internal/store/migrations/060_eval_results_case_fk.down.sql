-- Migration 060 down: drop the eval_results -> dataset_cases FK
-- by rebuilding the table without the FK constraint.
PRAGMA foreign_keys = OFF;
CREATE TABLE eval_results_old (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL,
    case_id    TEXT NOT NULL,
    seq        INTEGER NOT NULL,
    passed     INTEGER NOT NULL DEFAULT 0,
    actual     TEXT NOT NULL DEFAULT '{}',
    expected   TEXT NOT NULL DEFAULT '{}',
    error      TEXT NOT NULL DEFAULT '',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO eval_results_old
    SELECT id, run_id, IFNULL(case_id, ''), seq, passed, actual, expected, error, latency_ms, created_at
      FROM eval_results;
DROP TABLE eval_results;
ALTER TABLE eval_results_old RENAME TO eval_results;
PRAGMA foreign_keys = ON;
