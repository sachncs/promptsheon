-- Migration 026 (Postgres): banditstore schema.
--
-- F-21 follow-on. The bandit recommender's arm-posterior
-- state lives in this table. The banditstore.Backend
-- contract (LoadAll / SaveAll) operates on the full map;
-- a wholesale-replace strategy is sufficient for v0.1.x
-- because the bandit recommender's per-arm count is small
-- (single-digit arms per Capability).

CREATE TABLE IF NOT EXISTS bandit_arm_posteriors (
    arm_id  TEXT PRIMARY KEY,
    alpha   DOUBLE PRECISION NOT NULL,
    beta    DOUBLE PRECISION NOT NULL
);
