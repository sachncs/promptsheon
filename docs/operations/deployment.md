# Deployment

This page covers building, packaging, and running Promptsheon
in production. The dev walkthrough is in
[`docs/development/getting-started.md`](../development/getting-started.md); the threat
model + auth is in [`docs/security/security.md`](../security/security.md).

## Binaries

Three binaries ship from this repo:

| Binary | Path | Purpose |
|--------|------|---------|
| `promptsheond` | `bin/promptsheond` | Long-running server. |
| `promptsheon` | `bin/promptsheon` | CLI dispatcher. |
| `promptsheon-healthcheck` | `bin/promptsheon-healthcheck` | Container health probe. |

Build:

```bash
go build -o bin/promptsheond .
go build -o bin/promptsheon .
go build -o bin/promptsheon-healthcheck .

# ClickHouse rollup writer (optional build tag).
go build -o bin/promptsheond .
```

## Container

The Dockerfile is a multi-stage build:

1. `golang:1.26-alpine` — build stage.
2. `gcr.io/distroless/static-debian12:nonroot` — runtime.

Run:

```bash
docker run -d \
  --name promptsheond \
  -p 8080:8080 \
  -v /var/lib/promptsheon:/data \
  -e PROMPTSHEON_ADDR=:8080 \
  -e PROMPTSHEON_AUTH=true \
  -e PROMPTSHEON_TLS_CERT_FILE=/etc/promptsheon/tls.crt \
  -e PROMPTSHEON_TLS_KEY_FILE=/etc/promptsheon/tls.key \
  -e PROMPTSHEON_OPENAI_API_KEY="${OPENAI_API_KEY}" \
  -e PROMPTSHEON_ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  ghcr.io/sachncs/promptsheon:latest
```

The container's HEALTHCHECK is wired via the binary's env
vars (`PROMPTSHEON_HEALTHCHECK_HOST` /
`PROMPTSHEON_HEALTHCHECK_PORT`); see the Dockerfile for the
exact form. The binary polls `GET /health` and exits 0 on
200. A Kubernetes-style probe pointing at a different path
can pass the path as the first positional argument, e.g.:

```bash
/usr/local/bin/promptsheon-healthcheck /livez
```

## Helm chart

`deploy/helm/promptsheon/` ships a single-replica chart
(SQLite is the bundled store; a Postgres parity is tracked
in [docs/multi-region.md](multi-region.md)). The chart
renders ConfigMap, Secret, Service, Deployment, Ingress,
and ServiceMonitor.

Install:

```bash
helm repo add promptsheon https://sachncs.github.io/promptsheon
helm install promptsheon promptsheon/promptsheon \
  --set config.openaiApiKey="${OPENAI_API_KEY}" \
  --set config.anthropicApiKey="${ANTHROPIC_API_KEY}"
```

The chart ships a PodDisruptionBudget and a ServiceMonitor
for Prometheus scraping.

## systemd unit

```ini
[Unit]
Description=Promptsheon daemon
After=network.target

[Service]
Type=simple
User=promptsheon
Group=promptsheon
EnvironmentFile=/etc/promptsheon/env
ExecStart=/usr/local/bin/promptsheond
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

The unit reads `PROMPTSHEON_*` env vars from
`/etc/promptsheon/env`. Drop TLS cert + key into
`/etc/promptsheon/tls.{crt,key}` with mode 0640 owned by
`promptsheon:promptsheon`.

## Reverse proxy

Production tenants typically run the daemon behind a reverse
proxy (nginx, Caddy, Envoy) that terminates TLS and forwards
to the daemon over loopback. The daemon's
`PROMPTSHEON_TLS_CERT_FILE` and `PROMPTSHEON_TLS_KEY_FILE`
are only required when the daemon itself terminates TLS;
behind a reverse proxy they're omitted.

The reverse proxy must forward the client's source IP via
`X-Forwarded-For`; the rate limiter and audit chain honour
the header (set `PROMPTSHEON_TRUSTED_PROXIES` to the
proxy's CIDR list to prevent header-spoofing bypasses).

## Health probes

| Probe | Endpoint | Use |
|-------|----------|-----|
| Liveness | `GET /health` (or `GET /livez`) | Restart the container if the daemon becomes unresponsive. |
| Readiness | `GET /ready` (or `GET /readyz`) | Stop sending traffic until the daemon's DB is reachable. |

`promptsheon-healthcheck` honours `PROMPTSHEON_HEALTHCHECK_HOST`
and `PROMPTSHEON_HEALTHCHECK_PORT`, returns `0` for healthy
and non-zero for unhealthy, and accepts an optional path
argument (default `/health`). It works as a direct
`ExecStartCommand` argument or as the Docker HEALTHCHECK
target.

## Observability integration

The daemon's `GET /metrics` endpoint exposes the full
Prometheus inventory. Pair it with:

- `deploy/grafana/promptsheon-dashboard.json` — 10-panel
  dashboard for the live metrics.
- `deploy/prometheus/promptsheon-alerts.yaml` — three
  first-class SLOs + four health alerts.

The Grafana dashboard and the Prometheus rule file import
via:

```bash
# Grafana
curl -X POST http://grafana/api/dashboards/import \
  -H 'Content-Type: application/json' \
  -d @deploy/grafana/promptsheon-dashboard.json

# Prometheus
cp deploy/prometheus/promptsheon-alerts.yaml /etc/prometheus/
prometheus reload
```

## Multi-replica

Multi-replica deployments are supported via
`PROMPTSHEON_LEADER_ELECTION=true`; only the leader applies
migrations and writes to the audit chain. Reads scale
linearly across followers. SQLite + WAL handles small to
medium production loads; a shared-backend follow-on is
tracked in [docs/multi-region.md](multi-region.md).

## Upgrades

Tagged `vX.Y.Z` releases are produced by `.goreleaser.yml`:

- Multi-platform binaries (Linux, macOS, Windows; amd64, arm64).
- A Docker image published to the configured registry.
- `promptsheon_${VERSION}_checksums.txt` SBOM and `.deb` /
  `.rpm` packages (when enabled).
- A Git tag.

To upgrade:

1. Pull the new image / binary.
2. Run `./promptsheond` against a copy of the data directory;
   the next boot applies pending migrations.
3. Cut over by restarting the production daemon with the
   new binary.

Rollback is `git checkout` on the old image tag; migrations
are forward-only.

## Persistent volume

The daemon writes the SQLite database to
`$PROMPTSHEON_DB_PATH` (default `promptsheon.db`). Mount a
persistent volume at that path in containerised deployments:

```yaml
volumes:
  - name: db
    persistentVolumeClaim:
      claimName: promptsheon-db
```

The vault's master key and the LLM API keys are stored in
env vars, not in the database. Rotating the database
(volumes) does not lose the keys.

## More

- [docs/configuration.md](configuration.md) — full env-var
  reference.
- [docs/observability.md](observability.md) — metrics, logs,
  and traces.
- [docs/security.md](../security/security.md) — auth, audit chain, vault.