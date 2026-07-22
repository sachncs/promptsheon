-- 012: enforcer_state (budget + quota persistence).
--
-- OBS-13: persist DefaultEnforcer budget and quota state so a
-- daemon restart doesn't reset counters. Rows are keyed by
-- (workspace_id, kind) where kind is 'budget' or 'quota'. The
-- payload is the JSON-marshalled *budget.Budget / *quota.Quota
-- value. updated_at tracks the most recent mutation.

CREATE TABLE enforcer_state (
    workspace_id TEXT    NOT NULL,
    kind         TEXT    NOT NULL CHECK (kind IN ('budget', 'quota')),
    payload      TEXT    NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, kind)
);
