-- Migration 053 down: relax the decisions.id NOT NULL constraint.
-- The down is best-effort and not part of the production
-- recovery path.

PRAGMA foreign_keys = OFF;

CREATE TABLE decisions_old (
    id                 TEXT    PRIMARY KEY,
    recommendation_id  TEXT    NOT NULL UNIQUE,
    payload            TEXT    NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO decisions_old (id, recommendation_id, payload, created_at)
    SELECT id, recommendation_id, payload, created_at
      FROM decisions;

DROP TABLE decisions;
ALTER TABLE decisions_old RENAME TO decisions;
CREATE INDEX idx_decisions_created ON decisions (created_at);

PRAGMA foreign_keys = ON;
