-- 005: Data cleanups and backfills (idempotent).
--
-- On fresh installs every UPDATE / DELETE is a no-op (the WHERE
-- clauses match zero rows). On upgrades from pre-v0.1.0 these
-- normalise data the application layer used to accept freely.

-- Alert M2M backfill: severity / type / default group.
INSERT OR IGNORE INTO alert_rule_notification_groups
    (alert_rule_id, notification_group_id, created_at)
SELECT r.id, ng.id, CURRENT_TIMESTAMP
FROM alert_rules r
JOIN notification_groups ng ON lower(ng.name) = lower(r.severity)
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rule_notification_groups arng
    WHERE arng.alert_rule_id = r.id
);

INSERT OR IGNORE INTO alert_rule_notification_groups
    (alert_rule_id, notification_group_id, created_at)
SELECT r.id, ng.id, CURRENT_TIMESTAMP
FROM alert_rules r
JOIN notification_groups ng ON lower(ng.name) = lower(r.type)
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rule_notification_groups arng
    WHERE arng.alert_rule_id = r.id
);

INSERT OR IGNORE INTO alert_rule_notification_groups
    (alert_rule_id, notification_group_id, created_at)
SELECT r.id, ng.id, CURRENT_TIMESTAMP
FROM alert_rules r
JOIN notification_groups ng ON lower(ng.name) = 'default'
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rule_notification_groups arng
    WHERE arng.alert_rule_id = r.id
);

-- Enum coercion (no-op on fresh installs).
UPDATE executions SET environment = 'dev'
    WHERE environment NOT IN ('dev','staging','prod','');
UPDATE releases SET environment = 'dev'
    WHERE environment NOT IN ('dev','staging','prod');
UPDATE alerts SET status = 'active'
    WHERE status NOT IN ('active','resolved');
UPDATE eval_results SET passed = 0
    WHERE passed NOT IN (0,1);

-- Decisions NULL ID purge (no-op on fresh installs).
DELETE FROM decisions WHERE id IS NULL OR id = '';

-- Release version backfill (no-op on fresh installs; defensive).
UPDATE releases SET capability_version_id = (
    SELECT cv.id FROM capability_versions cv
    WHERE cv.capability_id = releases.capability_id
      AND cv.version = releases.capability_version
)
WHERE capability_version_id IS NULL;
DELETE FROM releases WHERE capability_version_id IS NULL;
