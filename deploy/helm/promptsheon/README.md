# promptsheon

The Control Plane for AI Capabilities.

## TL;DR

Promptsheon is a single-binary HTTP daemon that versions, releases,
and observes AI capabilities the way engineers manage code: with
immutable Versions, content-addressed artifacts, an approval
workflow (MakerChecker by default), a CAS (`pkg/cas/`), and an
audit chain that detects tampering. v0.1.x is single-region and
SQLite-backed by design.

## Prerequisites

- Kubernetes 1.27+ (StatefulSet, PodDisruptionBudget, ServiceMonitor)
- Helm 3.16+
- A storage class for the StatefulSet's PVC (regional block
  storage recommended; gp3 or similar)
- `cert-manager` if you want the chart's Certificate resource
  to issue TLS certs
- An LLM provider (OpenAI or Anthropic) — at least one API key

## Install

```bash
helm repo add promptsheon https://sachncs.github.io/promptsheon
helm install promptsheon promptsheon/promptsheon \
  --set config.openaiApiKey="${OPENAI_API_KEY}" \
  --set config.anthropicApiKey="${ANTHROPIC_API_KEY}" \
  --set image.tag=0.1.0
```

The chart defaults to a single-replica StatefulSet with a
PVC-backed SQLite database at `/var/lib/promptsheon`. See
`values.yaml` for the full list of configurable settings.

## Configuration

The daemon reads its config from a combination of the chart's
`values.yaml` (mounted as a ConfigMap), a YAML file at
`$PROMPTSHEON_CONFIG` (if set), and environment variables
(`PROMPTSHEON_*`). Env vars win over the file; the file wins
over the chart's ConfigMap.

Key settings:

| Value | Default | Description |
|-------|---------|-------------|
| `config.addr` | `:8080` | Listen address. |
| `config.dbPath` | `/var/lib/promptsheon/promptsheon.db` | SQLite database file. |
| `config.auth` | `true` | Enable API key authentication. |
| `config.logLevel` | `info` | `debug`, `info`, `warn`, `error`. |
| `config.approvalPolicy` | `maker_checker` | `maker_checker` or `majority`. |
| `config.openaiApiKey` | (none) | OpenAI API key. |
| `config.anthropicApiKey` | (none) | Anthropic API key. |
| `config.otelEndpoint` | (none) | OTLP gRPC endpoint for trace export. |
| `config.readOnly` | `false` | When `true`, every non-GET request returns 503. Used during canary rollouts. |

## TLS

The chart supports two TLS paths:

- **Daemon-terminated**: set `tls.enabled=true` and mount the
  cert + key via `tls.certSecret` and `tls.keySecret`.
- **Reverse-proxy terminated**: leave `tls.enabled=false` and
  terminate TLS at the ingress / nginx / Envoy; the daemon
  serves plain HTTP on `127.0.0.1` inside the pod.

For non-loopback binds, the chart's `securityContext` drops all
capabilities and sets `no-new-privileges`.

## Persistence

The StatefulSet mounts a `PersistentVolumeClaim` for
`/var/lib/promptsheon`. The SQLite database lives there
alongside WAL + SHM files. **The PVC is required** —
recreating the StatefulSet without preserving the PVC
recreates the database from scratch and the audit chain
starts at row 1.

## Observability

- `GET /health` (liveness) and `GET /ready` (readiness) on
  the configured port.
- `GET /metrics` (Prometheus scrape) — gated by `PermAuditRead`.
- `GET /livez` / `GET /readyz` — K8s-style aliases.
- `ServiceMonitor` template included; pair it with the
  `deploy/grafana/promptsheon-dashboard.json` dashboard.

## Multi-replica

v0.1.x is single-replica by design. The leader-election layer
(ADR-0030) is gated behind `config.leaderElection=true`; with
it, only the leader applies migrations and writes to the audit
chain. Reads scale linearly.

## See also

- [docs/operations.md](../../docs/operations.md) — backup /
  restore playbook.
- [docs/upgrade.md](../../docs/upgrade.md) — in-place upgrade
  + rollback from snapshot.
- [docs/canary.md](../../docs/canary.md) — canary / blue-green
  with `PROMPTSHEON_READ_ONLY`.
- [docs/multi-region.md](../../docs/multi-region.md) — the
  multi-region story (and its non-goal status for v0.1.x).
