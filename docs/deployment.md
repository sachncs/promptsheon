# Deployment

This page covers building, packaging, and running Promptsheon
in production. The dev walkthrough is in
[`docs/getting-started.md`](getting-started.md); the threat
model + auth is in [`docs/security.md`](security.md).

## Binaries

Three binaries ship from this repo:

| Binary | Path | Purpose |
|--------|------|---------|
| `promptsheond` | `cmd/promptsheond/` | Long-running server. |
| `promptsheon` | `cmd/promptsheon/` | CLI dispatcher. |
| `promptsheon-healthcheck` | `cmd/promptsheon-healthcheck/` | Container health probe. |

Build:

```bash
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon
go build -o promptsheon-healthcheck ./cmd/promptsheon-healthcheck

# ClickHouse rollup writer (optional build tag).
go build -tags clickhouse -o promptsheond ./cmd/promptsheond
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

The container healthcheck uses `promptsheon-healthcheck` to
hit `/health`:

```yaml
healthcheck:
  test: ["/usr/local/bin/promptsheon-healthcheck", "--addr=http://localhost:8080"]
  interval: 30s
  timeout: 5s
  retries: 3
```

## Helm chart

`deploy/helm/promptsheon/` ships a single-replica chart
(SQLite is the v0.1.x constraint; a Postgres parity follows).
The chart renders ConfigMap, Secret, Service, Deployment,
Ingress, and ServiceMonitor.

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
the header (set `RATELIMIT_TRUSTED_PROXIES` to the proxy's
CIDR to prevent header-spoofing bypasses).

## Health probes

| Probe | Endpoint | Use |
|-------|----------|-----|
| Liveness | `GET /health` (or `GET /livez`) | Restart the container if the daemon becomes unresponsive. |
| Readiness | `GET /ready` (or `GET /readyz`) | Stop sending traffic until the daemon's DB is reachable. |

`promptsheon-healthcheck <addr>` returns `0` for healthy and
`1` for unhealthy, so it works as a direct
`ExecStartCommand` argument.

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

v0.1.x is **single-replica** because of the SQLite constraint.
The leader-election work (ADR-0030) ships in v0.3.0; the
SLO dashboard has a "stale chain verification" alert that
catches multi-replica misconfigurations (the leader should
be running `/audit/verify` every minute).

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
- [docs/security.md](security.md) — auth, audit chain, vault.