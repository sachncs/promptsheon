-- Webhook Endpoints: persistent storage so subscriptions survive a restart.
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    secret TEXT,
    events TEXT NOT NULL,            -- comma-separated list of event types
    active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_active ON webhook_endpoints(active);
