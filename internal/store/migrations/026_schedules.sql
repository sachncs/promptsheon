-- Schedules table for the cron / webhook / manual Schedule
-- aggregate. The Scheduler (internal/scheduler) ticks this table
-- every few seconds and publishes schedule.fired events for due
-- rows.
CREATE TABLE IF NOT EXISTS schedules (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    release_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    cron TEXT NOT NULL DEFAULT '',
    webhook_path TEXT NOT NULL DEFAULT '',
    next_fire_at DATETIME NOT NULL,
    last_fire_at DATETIME,
    fired_count INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    created_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_schedules_next_fire ON schedules(next_fire_at);
CREATE INDEX IF NOT EXISTS idx_schedules_release ON schedules(release_id);