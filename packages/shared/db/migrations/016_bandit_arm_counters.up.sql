-- 016: bandit_arm_counters — per-(arm, replica) grow-only
-- counters that drive the bandit CRDT merge in
-- internal/banditstore.
--
-- The bandit recommender persists observations as a
-- (successes, failures) pair per (arm_id, replica_id). The
-- per-replica merge operator is component-wise MAX, so a
-- single INSERT ... ON CONFLICT DO UPDATE SET successes =
-- MAX(excluded.successes, successes) (and the same for
-- failures) is the conflict-safe equivalent of the in-memory
-- merge.
--
-- Load is SUM(successes), SUM(failures) GROUP BY arm_id so
-- the selector sees every replica's contribution aggregated
-- into the arm's effective posterior. Two replicas each
-- observing (2,1) and (3,4) for the same arm produce a
-- Load bucket of (5,5), not the component-wise max of (3,4).
--
-- Effective posterior is alpha = 1 + successes, beta = 1 +
-- failures; the Beta(1, 1) prior is implied by the missing-
-- row case. No prior rows are seeded because the bandit
-- recommender registers arms lazily (see
-- banditstore.Store.RegisterArms).

CREATE TABLE IF NOT EXISTS bandit_arm_counters (
    arm_id     TEXT    NOT NULL,
    replica_id TEXT    NOT NULL,
    successes  INTEGER NOT NULL DEFAULT 0,
    failures   INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (arm_id, replica_id)
);

CREATE INDEX IF NOT EXISTS bandit_arm_counters_updated_at_idx
    ON bandit_arm_counters(updated_at);