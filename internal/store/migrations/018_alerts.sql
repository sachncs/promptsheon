-- Alert Rules
CREATE TABLE IF NOT EXISTS alert_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    severity TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    threshold REAL NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,
    window INTEGER NOT NULL DEFAULT 0,
    config TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Alerts
CREATE TABLE IF NOT EXISTS alerts (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL,
    rule_name TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    message TEXT NOT NULL,
    details TEXT,
    triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME,
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id)
);

-- Notification Groups
CREATE TABLE IF NOT EXISTS notification_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    channels TEXT NOT NULL
);
