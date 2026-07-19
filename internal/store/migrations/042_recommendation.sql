-- Migration 042 (recommendation loop): persist recommendations
-- and decisions so the optimisation loop survives restarts.
--
-- The Recommendation loop is the auto-tuner that turns Observation
-- rollups into concrete CapabilityVersion proposals (raise
-- max_tokens, drop a guardrail, change temperature). Without
-- this migration, the producer generates recommendations but they
-- evaporate on the next process restart.

CREATE TABLE IF NOT EXISTS recommendations (
    id                     TEXT    PRIMARY KEY,
    capability_version_id  TEXT    NOT NULL,
    type                   TEXT    NOT NULL,
    payload                TEXT    NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recommendations_version_created
  ON recommendations (capability_version_id, created_at);

CREATE TABLE IF NOT EXISTS decisions (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    recommendation_id  TEXT    NOT NULL UNIQUE,
    payload            TEXT    NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_decisions_created
  ON decisions (created_at);
