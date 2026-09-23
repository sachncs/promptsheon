-- Durable organization-scoped outgoing webhook subscriptions.
CREATE TABLE outgoing_webhooks (
    id              TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    label           TEXT NOT NULL,
    url             TEXT NOT NULL,
    events_json     TEXT NOT NULL,
    active          INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    created_by      TEXT NOT NULL
);

CREATE INDEX idx_outgoing_webhooks_org
    ON outgoing_webhooks (organization_id, created_at DESC);
