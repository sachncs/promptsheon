-- Migration 045a down: rebuild notification_groups without UNIQUE and
-- drop the M2M table. This is a recovery path; the M2M data is
-- not preserved because the rule→group routing has not yet been
-- rewired in the Go code at this point in the migration
-- sequence.

PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS alert_rule_notification_groups;

CREATE TABLE notification_groups_old (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    channels TEXT NOT NULL
);
INSERT INTO notification_groups_old (id, name, channels)
  SELECT id, name, channels FROM notification_groups;
DROP TABLE notification_groups;
ALTER TABLE notification_groups_old RENAME TO notification_groups;

PRAGMA foreign_keys = ON;
