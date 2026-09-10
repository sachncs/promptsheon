-- 006: Seed data for fresh installs.
--
-- The system user is required because api_keys.user_id has
-- ON DELETE CASCADE and a FK to users(id). Any pre-existing
-- api_keys row with a stale user_id would have been repaired by
-- migration 058 in the old sequence; on a fresh install this seed
-- creates the user that audit FKs (RESTRICT) reference.

INSERT OR IGNORE INTO users
    (id, email, name, role, created_at, updated_at)
VALUES ('api', 'system@promptsheon.internal', 'System Actor',
        'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Default feature flags (idempotent).
INSERT OR IGNORE INTO feature_flags
    (name, enabled, description, updated_at)
VALUES
    ('webhook_secret_ciphertext_v2', 0,
     'Next-gen webhook secret encryption',
     CURRENT_TIMESTAMP),
    ('executions_partitioning', 0,
     'Per-day partitioning of executions',
     CURRENT_TIMESTAMP),
    ('audit_archive', 0,
     'Cold-storage archive of audit rows older than 6 months',
     CURRENT_TIMESTAMP);
