# Backup and Disaster Recovery

Promptsheon is a single-binary service. Backups and recovery
are scoped to the data directory (default `./data/` in
development, `/var/lib/promptsheon` in production). The
following playbook covers the full lifecycle: snapshot, copy,
restore, and verification.

## What to back up

| Path | Why |
|------|-----|
| `promptsheon.db` | The SQLite database — every audit row, every workflow, every config change. |
| `promptsheon.db-shm`, `promptsheon.db-wal` | SQLite WAL mode files. Required for crash recovery; back up together with the main DB. |
| `*.crt`, `*.key` (TLS) | If the daemon terminates TLS, back up the cert + key. The vault key (`PROMPTSHEON_VAULT_KEY`) is in the operator's secret store, not on disk. |
| Helm values | The chart-rendered config (ConfigMap, Secret). Required to restore a matching deployment. |

The audit archive (`audit_archive` table) is part of the
SQLite DB; backing up the DB also backs up the archive. The
vault ciphertext (provider API keys) is in the DB; the
plaintext keys live only in process memory and are re-derivable
from the upstream provider when the operator re-issues them.

## Backup strategy

The `promptsheond backup <path>` subcommand writes a consistent
SQLite snapshot to the supplied path while the daemon is live.
It uses SQLite's online backup API (`.backup` / `VACUUM INTO`)
so a running daemon's writes don't split the snapshot.

Recommended schedule:

- **Hourly snapshot** to local fast storage (PVC, EBS gp3).
- **Daily offsite copy** (e.g. S3 with object-lock for WORM).
- **Weekly restore drill** to a non-production environment.

```bash
# Hourly cron: keep the last 24 snapshots, prune the rest.
0 * * * * /usr/local/bin/promptsheond backup /var/backups/promptsheon/hourly.db && \
  find /var/backups/promptsheon/hourly.db.* -mmin +1440 -delete

# Daily offsite: S3 with object-lock for compliance.
5 0 * * * aws s3 cp /var/backups/promptsheon/daily.db s3://promptsheon-backups/$(date +\%Y\%m\%d).db \
    --object-lock-mode COMPLIANCE --object-lock-retain-until-date 2030-01-01
```

## Restore

```bash
# 1. Stop the daemon.
systemctl stop promptsheond

# 2. Replace the live DB with the backup.
mv /var/lib/promptsheon/promptsheon.db /var/lib/promptsheon/promptsheon.db.broken
cp /var/backups/promptsheon/daily.db /var/lib/promptsheon/promptsheon.db

# 3. Verify the chain before serving traffic.
promptsheond &
curl -s http://localhost:8080/api/v1/audit/verify

# 4. If the chain verifies, leave the daemon up. If not,
#    the backup is corrupt; fall back to the previous day's
#    backup and call the operator's incident response.

# 5. Re-issue any provider API keys that were stored in the
#    broken DB; the backup is a point-in-time snapshot, so
#    anything added between backup and crash is lost.
```

## Disaster recovery targets

| Tier | RPO | RTO |
|------|-----|-----|
| Hot (SLA-bound) | 1 hour | 30 minutes |
| Standard | 24 hours | 4 hours |
| Cold (dev) | 7 days | next business day |

Runbook: see [docs/upgrade.md](upgrade.md) for the in-place
upgrade path (forward-only migrations) and the rollback-from-
backup path (the only way to "downgrade" a schema, since
migrations are forward-only).

## What we do NOT back up

- `promptsheon.db-journal` — the rollback journal is recreated
  on the next boot.
- Stale `-shm` / `-wal` files from a crashed daemon — recovery
  creates fresh ones.
- The compiled binary — re-deploy via your CI artifact, not
  from a backup.

## See also

- [docs/upgrade.md](upgrade.md) — in-place upgrade + rollback.
- [docs/slos.md](slos.md#rpo--rto) — the SLO targets.
- [docs/multi-region.md](multi-region.md) — multi-region
  replication story (and its non-goal status for v0.1.x).