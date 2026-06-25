# Observability

The server exposes three orthogonal observability surfaces: **structured logs**, **traces**, and **metrics**. Each is independent; you can use any one without the others. A **hash-chained audit log** is the fourth, security-oriented surface — see [Security](security.md) and ADR [0003](adr/0003-hash-chained-audit-log.md).

## At a glance

| Surface | Source of truth | When to use | Where to read |
|---|---|---|---|
| Logs | `log/slog` JSON to stderr | Ad-hoc debugging, audit, alerting via log pipeline | `kubectl logs`, journald, CloudWatch, etc. |
| Traces | `internal/trace` (in-memory) + OTel SDK | Finding slow paths, correlating work across components | `GET /api/v1/traces/tree/{trace_id}` or your OTel collector |
| Metrics | `internal/metrics` (Prometheus) | Dashboards, SLOs, alerting | `GET /metrics` |
| Audit | `audit_entries` table, hash-chained | "Who did what, when?" — compliance | `GET /api/v1/audit`, `GET /api/v1/audit/verify` |
| Retention | `internal/observability/retention.go` | TTLs for trace / snapshot / audit rows | env vars below |

## Logging

One `*slog.Logger` per process, set up in `cmd/promptsheond/main.go`. The default handler is JSON to stderr. The log level is controlled by `PROMPTSHEON_LOG_LEVEL` (`debug`, `info`, `warn`, `error`).

```bash
# Pretty-print live logs
./promptsheond 2>&1 | jq .
```

### Fields

Every line carries the standard slog fields: `time`, `level`, `msg`. HTTP requests are logged with `method`, `path`, `status`, `latency`, `request_id`. LLM calls log `provider`, `model`, `latency`, `usage`, `cost_usd`, and a `trace_id` so they can be joined to a span.

### Correlation

Every request gets a request ID. The ID is propagated to the response (`X-Request-ID` header), to every log line written during the request, to every span, and to the audit entry. To trace a request end-to-end, take the `X-Request-ID` and grep your log aggregator.

## Tracing

Spans are recorded for every HTTP request and every LLM call. The default backend is the in-process `internal/trace` package, which keeps the last N spans in memory and exposes them via the REST API. Set `PROMPTSHEON_OTEL_ENDPOINT` to export to a real OTel collector instead.

### Span fields

| Field | Description |
|---|---|
| `trace_id` | The request's trace ID (also in `X-Request-ID`). |
| `span_id` | Unique per span within a trace. |
| `parent_id` | Empty for the root span; the parent's `span_id` otherwise. |
| `operation` | E.g. `http.request`, `llm.complete`, `db.query`. |
| `service` | The component (`api`, `llm`, `eval`, `workflow`, ...). |
| `status` | `ok`, `error`, or unset. |
| `attributes` | Free-form key-value pairs. |
| `error` | Error message, set when `status=error`. |

### API

```bash
# List recent spans
curl "http://localhost:8080/api/v1/traces?limit=100"

# Get a span by ID
curl "http://localhost:8080/api/v1/traces/{id}"

# Get the full tree for a trace
curl "http://localhost:8080/api/v1/traces/tree/{trace_id}"
```

### OpenTelemetry export

```bash
PROMPTSHEON_OTEL_ENDPOINT=otel-collector:4317
PROMPTSHEON_OTEL_INSECURE=true   # only for non-TLS collectors
```

Spans are exported via OTLP gRPC. The server keeps recording in-process even when the export is enabled, so the in-process API continues to work.

## Metrics

Prometheus-compatible. Exposed at `GET /metrics` (unauthenticated by default, but intended to be scraped from inside a trusted network). A JSON summary is also available at `GET /api/v1/metrics/summary`.

### Key metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | counter | `method`, `path`, `status` | Total HTTP requests. |
| `http_request_duration_seconds` | histogram | `method`, `path` | Request latency. |
| `llm_calls_total` | counter | `provider`, `model`, `status` | Total LLM calls. |
| `llm_call_duration_seconds` | histogram | `provider`, `model` | LLM call latency. |
| `llm_call_cost_usd_total` | counter | `provider`, `model` | Cumulative USD cost. |
| `llm_call_tokens_total` | counter | `provider`, `model`, `direction` | Token usage (`prompt`/`completion`). |
| `guardrail_violations_total` | counter | `rule_type`, `severity` | Total guardrail violations. |
| `guardrail_blocks_total` | counter | `rule_type` | Total blocked requests. |
| `eval_runs_total` | counter | `dataset_id` | Total evaluation runs. |
| `audit_chain_writes_total` | counter | – | Total audit entries written. |
| `audit_chain_verifications_total` | counter | `result` | Total verifications (`ok`/`fail`). |
| `circuit_breaker_state` | gauge | `provider` | `0`=closed, `1`=half-open, `2`=open. |
| `workflow_runs_total` | counter | `status` | Total workflow runs by terminal status. |
| `workflow_step_duration_seconds` | histogram | `step_id` | Per-step duration. |

### Scrape config

```yaml
scrape_configs:
  - job_name: promptsheon
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: /metrics
```

## Audit log

The audit log records every state-changing request. It is hash-chained; see ADR [0003](adr/0003-hash-chained-audit-log.md) and [Algorithms — Audit chain](algorithms.md#audit-chain).

### Querying

```bash
# List entries with filters
curl "http://localhost:8080/api/v1/audit?user_id=u-1&since=2024-01-01T00:00:00Z&limit=200"

# Export
curl "http://localhost:8080/api/v1/audit/export?format=csv" > audit.csv

# Verify the chain
curl "http://localhost:8080/api/v1/audit/verify"
# {"ok": true}  or  {"ok": false, "reason": "..."}
```

The verifier paginates the chain in chunks of 1000 rows. A long chain does not hold a single connection for the full duration.

## Retention

A background goroutine (`internal/observability/retention.go`) wakes every `CheckInterval` and deletes rows older than the per-table TTL.

### TTLs and env vars

| Resource | Default | Env var | Minimum |
|---|---|---|---|
| Trace spans | 30 days | `PROMPTSHEON_TRACE_TTL_DAYS` | 30 days (regulatory floor) |
| Snapshots | 30 days | `PROMPTSHEON_SNAPSHOT_TTL_DAYS` | 1 day |
| Audit entries | 90 days | `PROMPTSHEON_AUDIT_TTL_DAYS` | 1 day |
| Sweep interval | 60 minutes | `PROMPTSHEON_RETENTION_CHECK_MINUTES` | 1 minute |

The minimum trace retention of 30 days is enforced even if a smaller value is configured. This is a regulatory floor, not a knob.

### What is *not* affected

The retention sweep does not touch:

- Prompt, agent, or workflow definitions.
- API keys (in the vault).
- Webhook endpoints.
- Alert rules.

These are managed by the user and are not auto-deleted.

## SLOs and alerts

Suggested starter alerts when you set up your monitoring:

- **Error budget burn.** `rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01`
- **Latency.** `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 2`
- **Audit chain break.** `audit_chain_verifications_total{result="fail"} > 0`
- **Circuit breaker open.** `circuit_breaker_state > 1`
- **Cost spike.** `rate(llm_call_cost_usd_total[1h]) > 1.0` (adjust threshold to your budget)
- **Retention sweep stalled.** absence of writes for more than 2× the configured `CheckInterval` is suspicious; check the sweeper goroutine.

## See also

- [Security](security.md) — the threat model and the controls in this page
- [Algorithms — Audit chain](algorithms.md#audit-chain)
- [Algorithms — Retention sweep](algorithms.md#retention-sweep)
- [Deployment — Monitoring](deployment.md#monitoring)
- ADR [0003](adr/0003-hash-chained-audit-log.md), [0007](adr/0007-slog-as-observability-foundation.md)
