-- 049_execution_replay.up.sql
-- T3-1 time-travel debugging. Replays re-run an existing execution
-- with the same manifest, model, provider, environment, and inputs;
-- the new execution is linked to the original via `replay_of` and
-- the original's `replay_count` is incremented.
--
-- `inputs` was previously stored as a SHA-256 hex digest, which
-- prevented replay; this migration adds `input_hash` (the dedup
-- key) and keeps `inputs` available as a JSON blob going forward.
-- Pre-existing rows have `input_hash = NULL` and cannot be replayed.

ALTER TABLE executions ADD COLUMN replay_of TEXT REFERENCES executions(id) ON DELETE SET NULL;
ALTER TABLE executions ADD COLUMN replay_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE executions ADD COLUMN input_hash TEXT;

CREATE INDEX idx_executions_replay_of ON executions(replay_of);
CREATE INDEX idx_executions_input_hash ON executions(input_hash);

-- Replay lineage log. One row per replay attempt, regardless of
-- outcome (success, divergence, missing manifest, etc.). The
-- diff_summary captures what changed between original and replay.
CREATE TABLE execution_replays (
    id TEXT PRIMARY KEY,
    original_execution_id TEXT NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    replay_execution_id TEXT REFERENCES executions(id) ON DELETE SET NULL,
    outcome TEXT NOT NULL,                  -- started | completed | diverged | failed
    inputs_match INTEGER NOT NULL,          -- 1 if inputs were byte-identical to the original
    manifest_match INTEGER NOT NULL,        -- 1 if manifestHash matches
    model_match INTEGER NOT NULL,           -- 1 if model+provider match
    environment_match INTEGER NOT NULL,     -- 1 if environment matches
    diff_summary TEXT,                      -- JSON: per-node output delta vs the original
    created_at TEXT NOT NULL
);

CREATE INDEX idx_execution_replays_original ON execution_replays(original_execution_id, created_at DESC);
CREATE INDEX idx_execution_replays_outcome ON execution_replays(outcome);