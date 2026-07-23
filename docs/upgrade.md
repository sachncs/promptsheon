# Upgrade and Rollback

Promptsheon migrations are forward-only. Rolling back a
schema is not possible by reversing a migration; the only
correct rollback is "restore from a snapshot taken before the
upgrade". This page walks through both paths.

## In-place upgrade

Promptsheon ships schema migrations as
`internal/store/migrations/NNN_*.up.sql`. The next boot applies
every migration whose version is greater than the highest
applied version in the daemon's `migrations` table.

```bash
# 1. Snapshot the live DB. Even on a single-replica SQLite
#    deployment, an `sqlite3 .backup` is cheap insurance.
promptsheond backup /var/backups/promptsheon/pre-upgrade.db

# 2. Pull the new binary.
docker pull ghcr.io/sachncs/promptsheon:v0.2.0
# (or `go install github.com/sachncs/promptsheon/cmd/promptsheond@v0.2.0`)

# 3. Restart the daemon. The new binary applies pending
#    migrations on boot, then serves traffic.
systemctl restart promptsheond

# 4. Verify the chain.
curl -s http://localhost:8080/api/v1/audit/verify
# {"ok":true, "last_row_id":..., "last_hash":"..."}
```

Destructive migrations (filename contains `_destructive_`,
e.g. `008_destructive_cleanup.up.sql`) require
`PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true` to apply.
The daemon refuses to start with that gate unset so an
operator can't accidentally drop a pre-v0.1.0 table.

## Read-only upgrade (zero-downtime)

For zero-downtime upgrades, run two daemons side-by-side: the
old one with writes enabled, the new one with
`PROMPTSHEON_READ_ONLY=true`. Routes that need writes (POST /
PUT / DELETE) return 503 on the new daemon; reads continue to
work. Operators route read traffic to the new daemon first,
verify, then flip the read/write split.

```bash
# New daemon: read-only, no writes.
PROMPTSHEON_READ_ONLY=true \
PROMPTSHEON_ADDR=:8081 \
./promptsheond &

# Probe the new daemon.
curl -s http://localhost:8081/health # 200
curl -s -X POST http://localhost:8081/api/v1/releases/foo/votes  # 503

# Once reads are validated, drain the old daemon's write
# traffic by setting its PROMPTSHEON_READ_ONLY=true, wait for
# in-flight requests to finish, then restart as the new
# daemon. The whole cycle is a few seconds.
```

For production tenants, this is the recommended upgrade path:
no connection drops, the new code is exercised against live
read traffic first, and writes flip over only after the
operator confirms the new behaviour.

## Rollback (restore from snapshot)

When the new version is broken, the rollback is a restore
from the pre-upgrade snapshot:

```bash
# 1. Stop the daemon.
systemctl stop promptsheond

# 2. Replace the live DB.
mv /var/lib/promptsheon/promptsheon.db /var/lib/promptsheon/promptsheon.db.broken-$(date +%s)
cp /var/backups/promptsheon/pre-upgrade.db /var/lib/promptsheon/promptsheon.db

# 3. Downgrade the binary.
docker pull ghcr.io/sachncs/promptsheon:v0.1.0

# 4. Restart the daemon.
systemctl restart promptsheond

# 5. Verify the chain.
curl -s http://localhost:8080/api/v1/audit/verify
# {"ok":true, ...}

# 6. Re-issue any provider API keys created after the
#    snapshot timestamp.
```

A few notes:

- The audit archive (`audit_archive` table) lives inside
  the same DB; restoring the snapshot restores the archive
  too. New audit rows between snapshot and crash are lost —
  capture them before restoring.
- The vault ciphertext is also in the DB. Plaintext keys
  added between snapshot and crash are lost; the operator
  re-issues them from the upstream provider and re-saves
  via `POST /api/v1/vault/keys`.
- If the new daemon had bumped the migration version past
  what the old binary can read, the rollback is `git
  checkout` on the old binary; never restore a snapshot
  taken AFTER a destructive migration ran.

## See also

- [docs/operations.md](operations.md) — backup / restore.
- [docs/multi-region.md](multi-region.md) — multi-region
  replication story.
- [docs/canary.md](canary.md) — canary / blue-green deployment.