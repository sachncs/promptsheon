# ADR 0007: Use `log/slog` as the single observability foundation

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

The project needs structured logging, metrics, and tracing. We could use `logrus`, `zap`, or another logger, plus a separate metrics library, plus a separate tracer. Or we could use the standard library.

Since Go 1.21, `log/slog` provides structured logging in the standard library. The OpenTelemetry SDK for Go is the de-facto standard for traces and metrics. The two are interoperable via `otelslog`.

## Decision

- We use `log/slog` for all logging. There is exactly one `*slog.Logger` per process, set at startup. The default handler is JSON to stderr.
- We use the OpenTelemetry SDK for traces and metrics, but only when `PROMPTSHEON_OTEL_ENDPOINT` is set. If it is not set, traces and metrics are recorded in memory and exposed through `/api/v1/traces` and `/metrics`.
- We expose Prometheus metrics at `/metrics` regardless of whether OTel is configured.

## Options considered

1. **Standard library `log/slog` + OTel SDK (chosen).** No log dependency, OTel interop, Prometheus works.
2. **`logrus` or `zap` + Prometheus client + OTel.** Three dependencies, three styles.
3. **Custom observability stack.** A maintenance burden we do not need.

## Consequences

Positive:

- No logging dependency in `go.mod`. Smaller dependency surface.
- The JSON output is predictable and parseable by every log aggregator.
- Correlation IDs are attached to the context and surface in every log line, every trace span, and every metric label.

Negative:

- The JSON handler is the only one we ship. Pretty-printed output requires a pipe through `jq` or similar. This is documented in [Deployment](../deployment.md).
- The OTel SDK is heavy. We pay the import cost even when OTel is disabled. This is a small price for a uniform API.

## References

- `internal/trace/` — in-memory tracer
- `internal/metrics/` — Prometheus-compatible collector
- `internal/observability/retention.go` — retention sweep
- [Deployment](../deployment.md) — production build, systemd, Docker, nginx, monitoring.
