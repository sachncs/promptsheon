-- 011: audit_archive (cold storage for audit retention).
--
-- OBS-RET-1: Provide a destination table for the audit
-- retention path. RetentionManager.Enforce copies expired rows
-- here before considering deletion. The audit_entries chain
-- (linked by previous_hash / rowid order) cannot survive
-- deletion from the middle of the chain, so the default policy
-- is archive-only. Operators archive externally and may then
-- truncate the source table out of band.
--
-- Schema mirrors audit_entries plus an archived_at timestamp
-- so operators can see when each row was moved.

CREATE TABLE audit_archive (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    action        TEXT NOT NULL,
    resource      TEXT NOT NULL,
    details       TEXT DEFAULT '{}',
    timestamp     DATETIME NOT NULL,
    previous_hash TEXT DEFAULT '',
    entry_hash    TEXT DEFAULT '',
    timestamp_str TEXT NOT NULL DEFAULT '',
    resource_kind TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    archived_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_archive_archived_at ON audit_archive(archived_at);
CREATE INDEX idx_audit_archive_user_time ON audit_archive(user_id, timestamp DESC);
CREATE INDEX idx_audit_archive_resource_kind_id_time
    ON audit_archive(resource_kind, resource_id, timestamp DESC);
