-- 019_self_evolve.up.sql
-- Self-evolution closed loop. Per-capability opt-in: the daemon
-- monitors the running EvalRun score for an active release; if it
-- falls below self_evolve_min_score, SelfEvolver invokes a seeded
-- revision LLM to propose a new prompt, validates the candidate
-- against the same dataset, and promotes the validated version in
-- the configured env (typically dev) using a self-approve policy.
--
-- Schema: per-capability config (config) + per-cycle state
-- (state) to track cooldown, last revision attempt, last score, etc.
-- Cooldown survives daemon restarts because the state row is in
-- SQLite, not memory.

ALTER TABLE capabilities ADD COLUMN self_evolve_enabled        INTEGER NOT NULL DEFAULT 0;
ALTER TABLE capabilities ADD COLUMN self_evolve_min_score      REAL    NOT NULL DEFAULT 0.9;
ALTER TABLE capabilities ADD COLUMN self_evolve_max_revisions  INTEGER NOT NULL DEFAULT 10;
ALTER TABLE capabilities ADD COLUMN self_evolve_cooldown_sec   INTEGER NOT NULL DEFAULT 900;
ALTER TABLE capabilities ADD COLUMN self_evolve_target_env     TEXT    NOT NULL DEFAULT 'dev';
ALTER TABLE capabilities ADD COLUMN self_evolve_dataset_id    TEXT    DEFAULT '';

-- self_evolve_state: one row per (capability, target env) pair.
-- last_attempt_at drives the cooldown gate. cycle_started_at lets
-- a stuck cycle (no successful promote after max_revisions) be
-- retried after a daemon restart.
CREATE TABLE self_evolve_state (
    capability_id         TEXT    NOT NULL,
    target_env            TEXT    NOT NULL,
    last_attempt_at       DATETIME,
    last_promote_at       DATETIME,
    last_score            REAL,
    last_revision_index   INTEGER NOT NULL DEFAULT 0,
    cycle_started_at      DATETIME,
    last_status           TEXT    NOT NULL DEFAULT 'idle',  -- idle|detected|revising|validating|promoted|rejected
    last_error            TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (capability_id, target_env)
);

CREATE INDEX idx_self_evolve_state_attempt ON self_evolve_state(last_attempt_at);
