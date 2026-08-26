-- 050_cost_budgets.up.sql
-- T4-3 per-org cost budgets + the projection cache used by the
-- forecast endpoint. The forecast is computed on read, not stored;
-- this table is just the budget contract + the last-alerted
-- timestamp so we don't fire duplicate webhooks.

CREATE TABLE cost_budgets (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    label TEXT NOT NULL,
    period TEXT NOT NULL DEFAULT 'monthly'
        CHECK (period IN ('weekly', 'monthly')),
    -- Limit expressed in micros so we never round away a small
    -- over-budget. 5_000_000 = $5.00.
    limit_micros INTEGER NOT NULL,
    -- Threshold fraction in [0, 1] for firing an alert webhook.
    -- 0.8 means "fire when projected spend > 80% of limit".
    alert_threshold REAL NOT NULL DEFAULT 0.8
        CHECK (alert_threshold > 0 AND alert_threshold <= 1),
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_alerted_at TEXT,
    FOREIGN KEY (organization_id) REFERENCES orgs(id) ON DELETE CASCADE
);

CREATE INDEX idx_cost_budgets_org ON cost_budgets(organization_id);
CREATE UNIQUE INDEX idx_cost_budgets_org_label ON cost_budgets(organization_id, label);

-- Projected spend cache. Recomputed by the forecast endpoint on
-- read; persisted here so the dashboard can show a snapshot
-- without recomputing on every page load.
CREATE TABLE cost_forecast_snapshots (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    period_start TEXT NOT NULL,            -- ISO date for the start of the period
    period_end TEXT NOT NULL,              -- ISO date for the end (inclusive)
    spend_micros INTEGER NOT NULL,
    projected_micros INTEGER NOT NULL,
    band_low_micros INTEGER NOT NULL,      -- 95% CI lower bound
    band_high_micros INTEGER NOT NULL,     -- 95% CI upper bound
    window_days INTEGER NOT NULL,
    computed_at TEXT NOT NULL,
    FOREIGN KEY (organization_id) REFERENCES orgs(id) ON DELETE CASCADE
);

CREATE INDEX idx_cost_forecast_org_computed ON cost_forecast_snapshots(organization_id, computed_at DESC);