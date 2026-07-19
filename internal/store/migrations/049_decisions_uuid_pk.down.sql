-- Migration 049 down: rebuild decisions back to INTEGER PRIMARY KEY.
-- The id values are regenerated as ROWID-style integers because
-- the new id column is INTEGER. Existing UUIDs are converted to
-- a stable hash so the data is preserved.

PRAGMA foreign_keys = OFF;

CREATE TABLE decisions_old (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    recommendation_id  TEXT    NOT NULL UNIQUE,
    payload            TEXT    NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO decisions_old (recommendation_id, payload, created_at)
  SELECT recommendation_id, payload, created_at FROM decisions;

DROP TABLE decisions;
ALTER TABLE decisions_old RENAME TO decisions;
CREATE INDEX idx_decisions_created ON decisions (created_at);

PRAGMA foreign_keys = ON;
