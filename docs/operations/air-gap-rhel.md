# Deploying promptsheon to an air-gapped RHEL host

This runbook covers taking the `scripts/build-offline-installer.sh`
artifact from a connected build machine and standing it up on a host
that has **no outbound internet** — government, defense, healthcare,
and air-gapped financial deployments.

The installer tarball is a single self-contained bundle: node_modules,
the built server + frontend, the SQLite migrations, the SBOM, and a
`bootstrap.sh` that wires systemd. Once the tarball is on the target
host, no network access is required to install.

> **Audience:** an SRE or sysadmin who is comfortable with RHEL,
> systemd, and SELinux. Not an LLM engineer.

---

## 1. Build the installer on a connected machine

The target host is offline, so the installer must be built elsewhere
and copied across. Run from the repo root:

```bash
# 1. Reproducible install of every dep from the lockfile.
pnpm install --frozen-lockfile

# 2. Build every workspace (shared schemas + Fastify server + Next.js app).
pnpm --dir packages build

# 3. Generate the SBOM if your auditor needs it (CycloneDX JSON).
bash scripts/build-sbom.sh
# → docs/compliance/sbom.json

# 4. Bundle everything into a dated tarball.
bash scripts/build-offline-installer.sh
# → dist/promptsheon-offline-YYYYMMDD-HHMMSS.tar.gz
```

The build script refuses to run if `node_modules/` is missing — it
relies on the dep tree you just produced and does **not** reach out
to the npm registry.

## 2. Copy the tarball to the air-gapped host

Use whatever air-gap transfer mechanism your environment provides
(approved USB, sneakernet, write-once optical, classified network
bridge). The tarball is ~150MB compressed for a fresh install with
no workspace data. Verify the SHA-256 on both sides:

```bash
# On the build host
sha256sum dist/promptsheon-offline-*.tar.gz

# On the target host, after transfer
sha256sum /tmp/promptsheon-offline-*.tar.gz
```

The hashes must match. A mismatched transfer is the single most
common source of "works in staging, breaks in prod" surprises in
air-gap environments.

## 3. Pre-flight on the RHEL host

Before extracting, check:

```bash
# RHEL 9 or compatible (Rocky / Alma)
cat /etc/redhat-release

# systemd is the init system
ps -p 1 -o comm=

# The bootstrap creates a `promptsheon` user — make sure that uid is free.
getent passwd promptsheon || echo "user promptsheon does not exist (good)"

# FIPS mode requires a FIPS-validated Node build. Verify OpenSSL provider.
node -e "console.log(require('crypto').getFips())"
# Expected: 1 when running against a FIPS-validated build.
# Anything else means you are running a stock Node, not FIPS Node.
```

Decide on the install root. The default is `/opt/promptsheon`; if your
site mandates a different path, set `PROMPTSHEON_ROOT` before running
`bootstrap.sh`.

## 4. Extract and install

```bash
sudo mkdir -p /opt/promptsheon
sudo tar xzf /tmp/promptsheon-offline-YYYYMMDD-HHMMSS.tar.gz -C /opt/promptsheon \
  --strip-components=1
sudo /opt/promptsheon/bin/bootstrap.sh
```

`bootstrap.sh` will (in order):
1. Copy the payload to `PROMPTSHEON_ROOT` (default `/opt/promptsheon`).
2. Generate a `0600`-permissioned `.env` with safe production defaults
   (port `8080`, host `127.0.0.1`, env `production`). **Replace
   `PROMPTSHEON_WEBHOOK_SECRET`** before any traffic.
3. If invoked with `--fips`, append `PROMPTSHEON_FIPS_MODE=true` to
   `.env` and write the FIPS-mode warnings to the install log.
4. Install a `promptsheon.service` systemd unit pointing at
   `/opt/promptsheon/share/server/index.js`.
5. `systemctl daemon-reload && systemctl enable --now promptsheon.service`.

The whole sequence is idempotent — re-running it after an upgrade
overwrites files but does **not** touch the SQLite DB at
`/var/lib/promptsheon/promptsheon.db`.

## 5. FIPS mode

If the customer is in a FIPS-mandated environment (US federal,
defense, certain financial regulators), pass `--fips` to
`bootstrap.sh`:

```bash
sudo /opt/promptsheon/bin/bootstrap.sh --fips
```

This writes `PROMPTSHEON_FIPS_MODE=true` to `.env` and emits the
required warnings. **The bootstrap does not install a FIPS-validated
Node build** — that is the operator's responsibility and is documented
separately by your distro's package maintainers. If the operator
forgets, the server will refuse to boot:

```
PROMPTSHEON_FIPS_MODE=true but Node is not running against a
FIPS-validated OpenSSL provider. Refusing to compute the audit hash.
```

That error is intentional and is the audit-chain gate firing — see
`packages/server/src/audit/chain.ts` and the FIPS gate test at
`packages/server/test/fips-gate.test.ts`.

## 6. Verify the install

After the first boot:

```bash
# systemd says it's up
systemctl status promptsheon.service

# /api/health responds
curl -s http://127.0.0.1:8080/api/health | jq

# The audit chain reports a valid linked hash chain
curl -s http://127.0.0.1:8080/api/audit/verify | jq '.valid'
# Expected: true
# If this is false, STOP — your install is broken. Do not put it into
# production. See §10 Troubleshooting.

# FIPS mode (if enabled)
curl -s http://127.0.0.1:8080/api/health | jq '.fipsMode'
# Expected: true on a FIPS install
```

If your auditor wants evidence:

```bash
# Generate a signed audit report covering the last 90 days
curl -s -X POST "http://127.0.0.1:8080/api/audit/report?from=...&to=..." \
  -H "authorization: Bearer $PROMPTSHEON_API_KEY" | jq
# The response is a JSON document signed with the per-org operator key.
```

## 7. Upgrades

```bash
# 1. Build the new tarball on a connected host and copy across.
# 2. Stop the running service.
sudo systemctl stop promptsheon.service

# 3. Back up the DB.
sudo sqlite3 /var/lib/promptsheon/promptsheon.db ".backup '/var/lib/promptsheon/promptsheon.db.bak'"

# 4. Extract over the install root (keeps .env + the DB intact).
sudo tar xzf /tmp/promptsheon-offline-NEW.tar.gz -C /opt/promptsheon --strip-components=1

# 5. Migrations run automatically on next start.
sudo systemctl start promptsheon.service
journalctl -u promptsheon.service -n 200 | grep -i migration
# Expected: a row per applied migration; nothing failing.

# 6. Verify.
curl -s http://127.0.0.1:8080/api/audit/verify | jq '.valid'
```

If a migration fails:
1. The service stays up on the **previous schema** — better-sqlite3
   refuses to commit a failing transaction.
2. The failing migration is logged with its number; check the
   `CHANGELOG.md` for breaking changes that need a manual step.
3. Restore the DB backup, contact support with the migration number.

## 8. Operations

| Path | Purpose |
|---|---|
| `/opt/promptsheon/` | Install root (configurable via `PROMPTSHEON_ROOT`) |
| `/var/lib/promptsheon/promptsheon.db` | SQLite database |
| `/var/lib/promptsheon/.promptsheon/` | Content-addressable store |
| `/var/log/promptsheon/` | stdout/stderr (or use `journalctl -u promptsheon.service`) |
| `/etc/systemd/system/promptsheon.service` | systemd unit (managed by `bootstrap.sh`) |

Useful commands:

```bash
# Tail the structured logs
journalctl -u promptsheon.service -f --output=cat | jq

# Rotate the audit chain (weekly recommended)
curl -s -X POST http://127.0.0.1:8080/api/audit/archive \
  -H "authorization: Bearer $PROMPTSHEON_API_KEY" | jq

# Trigger an offline backup (the DB + CAS) without a network round-trip
sudo -u promptsheon sqlite3 /var/lib/promptsheon/promptsheon.db ".backup '/var/lib/promptsheon/backups/$(date +%Y%m%d).db'"
```

## 9. Air-gap-specific gotchas

1. **Don't rely on external package mirrors.** Your node_modules is
   frozen; the bundle doesn't refresh from npm. If a CVE drops,
   you must rebuild the tarball on a connected host and ship the
   new one.
2. **OCSP and CRL checks fail offline.** If the customer has TLS
   egress turned on but nothing else, OCSP stapling will time out.
   Configure the Node process with `--use-strict-ca` and pin the
   CA bundle that ships with the OS, not anything fetched at
   runtime.
3. **LLM provider calls go over the network.** Even with the
   server itself air-gapped, the LLM gateway needs to reach the
   upstream provider (OpenAI, Anthropic, etc.). Confirm with the
   customer's security team which provider endpoints are
   permitted. The hardened on-prem play is to point promptsheon
   at a self-hosted inference endpoint (vLLM, TGI) that lives
   inside the perimeter — the gateway supports any
   OpenAI-compatible URL via `LLM_BASE_URL`.
4. **The webhook receiver is the only ingress.** All external
   traffic that needs to reach promptsheon must come through a
   webhook signed with `PROMPTSHEON_WEBHOOK_SECRET`. There is no
   anonymous ingress.
5. **Backups are your DR story.** A self-hosted install that loses
   its DB loses every audit entry, every release, every prompt.
   Schedule daily `sqlite3 ... ".backup ..."` to a separate volume
   and copy the resulting file off-host regularly.

## 10. Troubleshooting

**`PROMPTSHEON_WEBHOOK_SECRET` missing.** The server refuses to
boot without a webhook secret in production. Edit `.env` and
restart: `sudo systemctl restart promptsheon.service`.

**`FOREIGN KEY constraint failed` on startup.** A migration
applied a partial schema change. The fastest recovery is the
backup-restart-replay loop: restore from `promptsheon.db.bak`,
restart, replay the in-flight operations.

**Audit chain reports `valid: false`.** This means a row was
tampered with after the chain was extended. Possible causes:
- Operator accidentally edited the SQLite DB out-of-band.
- A backup-restore step corrupted a row's `entry_hash`.
- Filesystem corruption.

**Stop the service, restore from a known-good backup, and
contact the security team. Do not delete rows to "fix" the
chain — the verifier exists to detect tampering, not to hide it.**

**LLM calls hang.** Confirm `LLM_BASE_URL` (or `OPENAI_BASE_URL` /
`ANTHROPIC_BASE_URL`) is reachable from the host. A typical
air-gap install points at a self-hosted vLLM endpoint inside the
perimeter.

**`getFips()` returns 0 on a FIPS install.** Your Node binary
isn't FIPS-validated. Replace the binary per your distro's
FIPS-validated-Node procedure, then re-run `bootstrap.sh --fips`.
The server will refuse to boot until the provider is actually
active — see `packages/server/test/fips-gate.test.ts` for the
contract being enforced.

---

This runbook ships with every offline installer; if it diverges
from your operational reality, please open an issue against the
repo so the next installer ships an updated copy.