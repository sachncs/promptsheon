-- 010: SSE hub state singleton.
--
-- OBS-LOG-3: persist the Hub.nextID counter so SSE log IDs
-- survive a daemon restart. Without this, a restarted daemon
-- restarts at nextID=1 and could mint the same client IDs as
-- a previous run (collisions in the in-memory clients map).

CREATE TABLE ws_state (
    id              INTEGER PRIMARY KEY CHECK (id = 0),
    next_id         INTEGER NOT NULL DEFAULT 0,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
