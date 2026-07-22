# Phase 3 — Observability Wiring

All observability findings. Fast forward: replace dead surfaces, no deprecation shims.

## Tracing

- [x] **OBS-1** Move trace writes off the request path. (See Phase 0.)
- [x] **OBS-2** Wire OTel into the HTTP middleware. (See Phase 0.)
- [x] **OBS-3** Fix `OTelTracer.Finish` to call `otelSpan.End()`. (See Phase 0.)

- [ ] **OBS-TR-1** Replace the SQLite trace backend with an OTLP-only export. Drop the SQLite tracer entirely once OTel is wired.
  - **Where**: `internal/trace/sqlite.go` (delete), `internal/api/server.go` (drop the `WithTracing` option), `cmd/promptsheond/main.go`.
  - **Accept**: With no `PROMPTSHEON_OTEL_ENDPOINT`, the daemon starts without a trace store; with it set, traces export to OTLP.

- [x] **OBS-TR-2** Honour the trace `ctx` parameter — pass it through to `oteltrace.Span.End()` instead of using `context.Background()`.
  - **Where**: `internal/trace/otel.go:88-95`.

- [ ] **OBS-TR-3** Move the OTel sampling config from a hard-coded 5% to `PROMPTSHEON_OTEL_SAMPLE_RATIO` (default 1.0).
  - **Where**: `internal/trace/exporter.go:62` and `internal/config/config.go`.

## Metrics

- [ ] **OBS-5a** Wire `LLMMiddleware` into the `executor.Executor.RunRequest` call path.
  - **Where**: `internal/metrics/middleware.go:100-126` and `internal/invoke/invoke.go:91-113`.

- [ ] **OBS-5b** Wire `WorkflowMiddleware` into the workflow engine.
  - **Where**: `internal/metrics/middleware.go:132-158` and `internal/workflow/run.go`.

- [ ] **OBS-6** Exclude `/metrics` from `promptsheon_http_requests_total`.
  - **Where**: `internal/metrics/middleware.go:23, 43-49`.

- [x] **OBS-7** Export `promptsheon_audit_dropped_total` and `promptsheon_trace_dropped_total` to Prometheus.
  - **Where**: `internal/api/server.go:614, 668-697` and `internal/metrics/collector.go`.

- [ ] **OBS-8** Delete `LabeledCounter` and `LabeledHistogram` from the active Collector. Move to a separate `cardinality_test.go` if any caller needs them later.
  - **Where**: `internal/metrics/collector.go:420-469`.

- [ ] **OBS-9** Cap `dashboardSummary` to 100 spans and compute incrementally (cache last scrape's aggregates, update on new spans).

- [ ] **PERF-3** Replace the `Percentile` O(N log N) sort with a fixed-bucket t-digest or CKMS sketch.
  - **Where**: `internal/metrics/collector.go:97-114`.

## Audit

- [ ] **OBS-AUDIT-1** Wire `auditDropped` into `/api/v1/metrics`.
  - **Where**: `internal/api/server.go` and `internal/metrics/collector.go`.

- [ ] **OBS-AUDIT-2** Add a Prometheus histogram `promptsheon_audit_queue_latency_seconds` for time between `audit()` and DB write.

- [ ] **OBS-AUDIT-3** Have `VerifyAuditChain` return a structured result `{Ok bool, TailMismatch bool, LastRowID int64, LastHash string}` so a UI can show status without re-walking the chain.

## Logs / SSE

- [x] **OBS-4** Wire the SSE log stream into the slog chain. (See Phase 0.)

- [ ] **OBS-LOG-1** Add level/source filters to `Hub.HandleSSE` so clients can subscribe to a subset.
  - **Where**: `internal/ws/hub.go:30-34, 156-200`.

- [ ] **OBS-LOG-2** Replace the blocking send on `Hub.broadcast` with a select-with-default + drop metric.
  - **Where**: `internal/ws/hub.go:126`.

- [ ] **OBS-LOG-3** Persist `nextID` to SQLite so SSE log IDs survive restart.
  - **Where**: `internal/ws/hub.go:144` and new migration.

## SLO

- [x] **OBS-10** Implement the SLO evaluator goroutine. (See Phase 0.)

- [x] **OBS-SLO-1** Implement `slo.Repository` against SQLite.
  - **Where**: `internal/slo/sqlite.go` (new) and `internal/store/migrations/064_slo_tables.up.sql`.

## Rollups / lineage / recommendation

- [x] **OBS-CH-1** Wire ClickHouse writer. (See Phase 0.)
- [x] **OBS-12** Wire recommendation producer. (See Phase 0.)
- [x] **OBS-Lineage-1** Implement `lineage.Repository` against SQLite. (See Phase 0.)

- [ ] **OBS-13** Persist `DefaultEnforcer` budget/quota state to SQLite so restarts don't reset counters.
  - **Where**: `internal/invoke/invoke.go:118-183` and new migration.

## Scheduler / eventbus

- [ ] **OPS-5a** Wire the scheduler's `Memory` bus to the executor's `HandleScheduleEvent`.
  - **What**: `cmd/promptsheond/main.go` constructs one shared `eventbus.Memory` and passes it to both scheduler and executor.
  - **Where**: `cmd/promptsheond/main.go:140, 325` and `internal/eventbus/publisher.go`.

- [ ] **OPS-5b** Make `Memory.Publish` async via a buffered channel and worker pool; keep the audit chain guarantee by persisting synchronously, fan out asynchronously.
  - **Where**: `internal/eventbus/publisher.go`.

## Retention

- [ ] **OPS-3** Have `RetentionManager.Enforce` return errors so callers can log them and surface a metric.
  - **Where**: `internal/observability/retention.go:137, 149`.

- [x] **OPS-4** Move retention to a separate `*sql.DB` so cleanup never blocks request-path writes.
  - **Where**: `cmd/promptsheond/main.go:195-204, 218-220`.
  - Shipped in commit `15374e0 fix(daemon): DB-CONC-2 — retention DB outlives buildServer`. The dedicated `*sql.DB` is opened in `main()` (not `buildServer`) and `defer`-closed at main() level so the handle lives as long as the daemon; the background retention loop can keep ticking.

- [ ] **OBS-RET-1** Actually delete audit rows older than `AuditTTL` after verifying the chain suffix.
  - **Where**: `internal/observability/retention.go:76-150`.

## Dashboards / SLO definitions

- [ ] **OBS-DASH-1** Check in a Grafana dashboard JSON under `deploy/grafana/`.
- [ ] **OBS-DASH-2** Check in a Prometheus alert rules file under `deploy/prometheus/`.
- [ ] **OBS-DASH-3** Document three SLOs in `docs/slos.md`: invoke p95 latency, audit chain integrity, harness eval success rate.
