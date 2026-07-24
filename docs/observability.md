# Observability

Promptsheon exposes three surfaces:

1. **Structured logs** on stderr (always JSON).
2. **Prometheus metrics** at `GET /metrics` (PermAuditRead-gated).
3. **OpenTelemetry traces** exported to an OTLP gRPC endpoint
   (when `PROMPTSHEON_OTEL_ENDPOINT` is set).

The audit chain (`GET /api/v1/audit`, `GET /api/v1/audit/verify`)
is a fourth surface — it lives in [docs/security.md](security.md)
because the chain is a security artefact.

## Logs

Every log line is JSON. The shape:

```json
{
  "time": "2026-07-23T10:11:12.345Z",
  "level": "INFO",
  "msg": "http request",
  "request_id": "...",
  "trace_id": "...",
  "user_id": "alice",
  "method": "POST",
  "path": "/api/v1/versions/v1/releases",
  "status": 201,
  "duration": "12.3ms"
}
```

`request_id` is either the inbound `X-Request-ID` header or a
generated random ID; `trace_id` comes from the OTel span context
when tracing is enabled.

The default level is `info`. Set `PROMPTSHEON_LOG_LEVEL=debug` for
verbose output during development.

## Metrics

The daemon emits `promptsheon_*` metrics under the
`internal/metrics` package. The full inventory:

### API

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_http_requests_total` | counter | Excludes `/health`, `/ready`, `/livez`, `/readyz`, `/metrics`. |
| `promptsheon_http_request_duration_seconds` | histogram | Same exclusions. |
| `promptsheon_http_errors_total` | counter | 5xx responses only. |

### LLM

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_llm_calls_total` | counter | Increments on every LLM call. |
| `promptsheon_llm_latency_seconds` | histogram | Wall-clock per call. |
| `promptsheon_llm_tokens_total` | counter | prompt + completion. |
| `promptsheon_llm_input_tokens_total` | counter | |
| `promptsheon_llm_output_tokens_total` | counter | |
| `promptsheon_llm_cost_usd_total` | counter | |
| `promptsheon_llm_ttft_seconds` | histogram | time-to-first-token (streaming). |

### Harness

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_eval_runs_total` | counter | |
| `promptsheon_eval_cases_total` | counter | Total per-case count (passed + failed). |
| `promptsheon_eval_cases_passed_total` | counter | Used by the SLO alert in `deploy/prometheus/promptsheon-alerts.yaml`. |
| `promptsheon_eval_cases_failed_total` | counter | Used by the SLO alert. |
| `promptsheon_eval_duration_seconds` | histogram | |

### Audit + trace pipelines

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_audit_dropped_total` | counter | Entries dropped because the worker queue was full. |
| `promptsheon_audit_queue_latency_seconds` | histogram | Time between `audit()` and DB write. |
| `promptsheon_audit_chain_verifications_total` | counter | Increments on every `/audit/verify` call. |
| `promptsheon_trace_dropped_total` | counter | Spans dropped (currently 0 in v0.2.0; reserved). |
| `promptsheon_log_hub_drops_total` | counter | SSE log stream drops. |

### Workflow

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_workflow_runs_total` | counter | |
| `promptsheon_workflow_duration_seconds` | histogram | |
| `promptsheon_workflow_active` | gauge | Currently running. |

### Guardrail

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_guardrail_violations_total` | counter | |
| `promptsheon_guardrail_blocks_total` | counter | |
| `promptsheon_guardrail_passes_total` | counter | |

### Review + hallucination

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_review_pending` | gauge | |
| `promptsheon_review_total` | counter | |
| `promptsheon_review_approved_total` | counter | |
| `promptsheon_review_rejected_total` | counter | |
| `promptsheon_review_duration_seconds` | histogram | |
| `promptsheon_hallucination_scores` | histogram | |

## SLOs and alerts

Three first-class SLOs ship with the project. The Prometheus
alert definitions live in `deploy/prometheus/promptsheon-alerts.yaml`
and the Grafana dashboard in `deploy/grafana/promptsheon-dashboard.json`.
See [docs/slos.md](slos.md) for the targets, queries, and
operating notes.

## Tracing

When `PROMPTSHEON_OTEL_ENDPOINT` is set, the daemon exports
spans over OTLP gRPC. The default sampler is `ParentBased(
TraceIDRatioBased(PROMPTSHEON_OTEL_SAMPLE_RATIO))` so upstream
sampling decisions are honoured and root spans are sampled at
the configured ratio (default 1.0).

Production deployments with high request volume typically set
`PROMPTSHEON_OTEL_SAMPLE_RATIO=0.1` (10%) or below; downstream
queries with `probability` samplers can upsample for specific
investigations.

## Audit chain

See [docs/security.md](security.md#audit-chain) for the chain
shape, the `VerifyAuditChain` endpoint, and the retention
enforcement (the chain is never deleted; rows that exceed
`PROMPTSHEON_AUDIT_TTL_DAYS` are copied to `audit_archive`
and the source row is preserved).

## Operator checklist

1. `curl /metrics | grep promptsheon_` confirms the daemon is
   exporting.
2. `curl /api/v1/audit/verify` returns `{ok: true, last_row_id,
   last_hash}`. Run on a schedule.
3. `PROMPTSHEON_LOG_LEVEL=debug` on a single replica during
   incident response — every log line carries `request_id` and
   `trace_id` so you can pivot from a trace into the request
   log.