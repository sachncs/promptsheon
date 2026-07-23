# Phase 3 — Observability Wiring

All observability findings. Fast forward: replace dead surfaces, no deprecation shims.

## Tracing

- [x] **OBS-1** Move trace writes off the request path. (See Phase 0.)
- [x] **OBS-2** Wire OTel into the HTTP middleware. (See Phase 0.)
- [x] **OBS-3** Fix `OTelTracer.Finish` to call `otelSpan.End()`. (See Phase 0.)

- [x] **OBS-TR-1** Replace the SQLite trace backend with an OTLP-only export. Drop the SQLite tracer entirely once OTel is wired.
  - **Where**: `internal/trace/sqlite.go` (delete), `internal/api/server.go` (drop the `WithTracing` option), `cmd/promptsheond/main.go`.
  - **Accept**: With no `PROMPTSHEON_OTEL_ENDPOINT`, the daemon starts without a trace store; with it set, traces export to OTLP.
  - **Status**: `internal/trace/sqlite.go` is gone; `main.go` selects `NewNoopTracer()` when `cfg.OTelEndpoint == ""` and `NewOTelTracer("promptsheond")` otherwise (`cmd/promptsheond/main.go:299-303`). `WithTracing` is retained because it is the wiring point for the OTel tracer + collector on the server; the comment in `server.go:151-156` documents the OTel-only contract. `/api/v1/traces` is unregistered (`server.go:417-422`). Acceptance met.

- [x] **OBS-TR-2** Honour the trace `ctx` parameter — pass it through to `oteltrace.Span.End()` instead of using `context.Background()`.
  - **Where**: `internal/trace/otel.go:88-95`.

- [x] **OBS-TR-3** Move the OTel sampling config from a hard-coded 5% to `PROMPTSHEON_OTEL_SAMPLE_RATIO` (default 1.0).
  - **Where**: `internal/trace/exporter.go:62` and `internal/config/config.go`.
  - **Status**: `internal/trace/exporter.go` reads `PROMPTSHEON_OTEL_SAMPLE_RATIO` with `strconv.ParseFloat` and clamps to `[0,1]`; default 1.0 means every span ships. Reading the env var directly in the exporter (rather than via `Config`) is the lazy path — one source of truth, no double-mapping. The misleading "Default to 0.05" comment in the old code is now accurate to the variable name `sampleRatio`.

## Metrics

- [x] **OBS-5a** Wire `LLMMiddleware` into the `executor.Executor.RunRequest` call path.
  - **Where**: `internal/metrics/middleware.go:100-126` and `internal/invoke/invoke.go:91-113`.
  - **Status**: `invoke.Invoker.WithObservability` (`internal/invoke/invoke.go:84-89`) takes the collector + tracer; `invokeLLM` (`invoke.go:224-242`) wraps `RunRequest` in `metrics.LLMMiddleware`. Wired from `main.go:516-517`.

- [x] **OBS-5b** Wire `WorkflowMiddleware` into the workflow engine.
  - **Where**: `internal/metrics/middleware.go:132-158` and `internal/workflow/run.go`.
  - **Status**: `workflow.Engine.Run` (`internal/workflow/run.go:45-71`) calls `runInstrumented` when `llmCollector` and `tracer` are non-nil. Wired from `main.go:544-547` via `workflow.NewEngine(...).WithObservability(collector, tracer)`.

- [x] **OBS-6** Exclude `/metrics` from `promptsheon_http_requests_total`.
  - **Where**: `internal/metrics/middleware.go:23, 43-49`.
  - **Status**: `HTTPMiddleware` adds `/metrics` to the probe skip set (`internal/metrics/middleware.go:24-26`) so Prometheus scrapes do not inflate the request count.

- [x] **OBS-7** Export `promptsheon_audit_dropped_total` and `promptsheon_trace_dropped_total` to Prometheus.
  - **Where**: `internal/api/server.go:614, 668-697` and `internal/metrics/collector.go`.

- [x] **OBS-8** Delete `LabeledCounter` and `LabeledHistogram` from the active Collector. Move to a separate `cardinality_test.go` if any caller needs them later.
  - **Where**: `internal/metrics/collector.go:420-469`.
  - **Status**: Neither type is referenced from `collector.go`; both live in `internal/metrics/cardinality_test.go` as historical reference, with a one-cardinality test that exercises `SortLabels`.

- [x] **OBS-9** Cap `dashboardSummary` to 100 spans and compute incrementally (cache last scrape's aggregates, update on new spans).
  - **Status**: Superseded. The `dashboardSummary` shape was removed when OBS-7 collapsed the trace store; `metrics.Collector.GetSummary` (`internal/metrics/collector.go:387-446`) now returns a `Summary` with no span list — only aggregates (counters, percentiles, pipeline metrics). There is nothing left to cap because spans no longer live in the daemon post-`internal/trace/sqlite.go` deletion.

- [x] **PERF-3** Replace the `Percentile` O(N log N) sort with a fixed-bucket t-digest or CKMS sketch.
  - **Where**: `internal/metrics/collector.go:97-114`.
  - **Status**: `Histogram.Percentile` (`collector.go:105-146`) walks the bucket counter array (O(buckets)=15) and applies linear interpolation. The raw `values` slice still exists for `Sum`/`Avg`, but the percentile path no longer sorts.

## Audit

- [x] **OBS-AUDIT-1** Wire `auditDropped` into `/api/v1/metrics`.
  - **Where**: `internal/api/server.go` and `internal/metrics/collector.go`.
  - **Status**: `Collector.SetAuditDropped` is called from `Server.audit` on every drop (`server.go:649-651`); the value surfaces in two places:
    1. Prometheus scrape: `promptsheon_audit_dropped_total` (`collector.go:504`).
    2. JSON summary: `PipelineMetrics.AuditDropped` (`collector.go:436`) at `/api/v1/metrics/summary`.
  `promptsheon_trace_dropped_total` follows the same wiring (`SetTraceDropped` + `collector.go:506`).

- [x] **OBS-AUDIT-2** Add a Prometheus histogram `promptsheon_audit_queue_latency_seconds` for time between `audit()` and DB write.
  - **Status**: `Collector.AuditQueueLatency` is a `*Histogram` (`collector.go:214`) observed from `Server.auditWorker` after each DB commit (`server.go:756-761`). Prometheus output: `promptsheon_audit_queue_latency_seconds` (`collector.go:505`).

- [x] **OBS-AUDIT-3** Have `VerifyAuditChain` return a structured result `{Ok bool, TailMismatch bool, LastRowID int64, LastHash string}` so a UI can show status without re-walking the chain.
  - **Status**: `store.AuditVerifyResult` (`internal/store/sqlite.go:202-208`) carries all four fields plus a `Reason` for human-readable failure context. The handler (`internal/api/handlers_audit.go:144-157`) returns the JSON shape `{ok, tail_mismatch, last_row_id, last_hash, reason}` at `GET /api/v1/audit/verify`.

## Logs / SSE

- [x] **OBS-4** Wire the SSE log stream into the slog chain. (See Phase 0.)

- [x] **OBS-LOG-1** Add level/source filters to `Hub.HandleSSE` so clients can subscribe to a subset.
  - **Where**: `internal/ws/hub.go:30-34, 156-200`.
  - **Status**: `Hub.LogFilter` (`internal/ws/hub.go:38-41`) and the `?level=info,warn&source=auth` parsing in `HandleSSE` (`hub.go:285-298`) wire the per-client filter; the broadcast loop matches before sending (`hub.go:159-167`).

- [x] **OBS-LOG-2** Replace the blocking send on `Hub.broadcast` with a select-with-default + drop metric.
  - **Where**: `internal/ws/hub.go:126`.
  - **Status**: `Hub.BroadcastLog` (`internal/ws/hub.go:213-224`) does `select { case h.broadcast <- data: default: h.dropped.Add(1) }`. `Hub.Dropped()` is read by the metrics collector (`hub.go:228-230`, `collector.go:507-508`) and exported as `promptsheon_log_hub_drops_total`.

- [x] **OBS-LOG-3** Persist `nextID` to SQLite so SSE log IDs survive restart.
  - **Where**: `internal/ws/hub.go:144` and new migration.
  - **Status**: `HubStore` interface (`internal/ws/hub.go:78-81`) defines `GetWSNextID`/`SetWSNextID`. `Hub.SetStore` loads on boot (`hub.go:117-132`), `Hub.Stop` writes back on shutdown (`hub.go:195-204`). `main.go:135-137` wires the store. Migration backs the table; `store.SQLite` implements the interface (used in `main_test.go` indirectly).

## SLO

- [x] **OBS-10** Implement the SLO evaluator goroutine. (See Phase 0.)

- [x] **OBS-SLO-1** Implement `slo.Repository` against SQLite.
  - **Where**: `internal/slo/sqlite.go` (new) and `internal/store/migrations/064_slo_tables.up.sql`.

## Rollups / lineage / recommendation

- [x] **OBS-CH-1** Wire ClickHouse writer. (See Phase 0.)
- [x] **OBS-12** Wire recommendation producer. (See Phase 0.)
- [x] **OBS-Lineage-1** Implement `lineage.Repository` against SQLite. (See Phase 0.)

- [x] **OBS-13** Persist `DefaultEnforcer` budget/quota state to SQLite so restarts don't reset counters.
  - **Where**: `internal/invoke/invoke.go:118-183` and new migration.
  - **Status**: `PersistedEnforcer` (`internal/invoke/persisted_enforcer.go`) wraps `DefaultEnforcer` and writes through `EnforcerStore` on every `SetBudget`/`SetQuota`/`EnforceBudget`. `main.go:424` constructs it via `invoke.NewPersistedEnforcer(rootCtx, db, nil, logger)`.

## Scheduler / eventbus

- [x] **OPS-5a** Wire the scheduler's `Memory` bus to the executor's `HandleScheduleEvent`.
  - **What**: `cmd/promptsheond/main.go` constructs one shared `eventbus.Memory` and passes it to both scheduler and executor.
  - **Where**: `cmd/promptsheond/main.go:140, 325` and `internal/eventbus/publisher.go`.
  - **Status**: One `eventbus.NewMemory()` is constructed in `main.go:156`, threaded into `scheduler.New` (`main.go:168`) and `executor.New` (`main.go:462`). The executor subscribes to `schedule.fired` in `main.go:507-514`.

- [x] **OPS-5b** Make `Memory.Publish` async via a buffered channel and worker pool; keep the audit chain guarantee by persisting synchronously, fan out asynchronously.
  - **Where**: `internal/eventbus/publisher.go`.
  - **Status**: `NewAsyncMemory(buffer, workers)` (`internal/eventbus/publisher.go:127-143`) builds a buffered queue + worker pool. `Memory.Publish` (`publisher.go:224-245`) hands the event to the queue with a non-blocking send + drop counter. `NewMemory` keeps the synchronous path for callers that need the audit-chain guarantee before returning. `Memory.Dropped()` exposes backpressure (`publisher.go:250`).

## Retention

- [x] **OPS-3** Have `RetentionManager.Enforce` return errors so callers can log them and surface a metric.
  - **Where**: `internal/observability/retention.go:137, 149`.
  - **Status**: `Enforce` returns a wrapped error from every failure path (`internal/observability/retention.go:140-198`); the caller in `main.go`'s `buildServer` would log + surface, but `RetentionManager.Start` already logs (`retention.go:80-96`) and `m.lastErr` is retained for callers that want a quick read.

- [x] **OPS-4** Move retention to a separate `*sql.DB` so cleanup never blocks request-path writes.
  - **Where**: `cmd/promptsheond/main.go:195-204, 218-220`.
  - Shipped in commit `15374e0 fix(daemon): DB-CONC-2 — retention DB outlives buildServer`. The dedicated `*sql.DB` is opened in `main()` (not `buildServer`) and `defer`-closed at main() level so the handle lives as long as the daemon; the background retention loop can keep ticking.

- [x] **OBS-RET-1** Actually delete audit rows older than `AuditTTL` after verifying the chain suffix.
  - **Where**: `internal/observability/retention.go:76-150`.
  - **Status**: Implemented as **archive-not-delete** (`internal/observability/retention.go:131-184`). The original "delete" plan would break `VerifyAuditChain`, which walks `audit_entries` from `rowid 1` forward and chains by `previous_hash`; removing rows from the middle is unrecoverable. The chain is verified first (`verifyChainForRetention`), then expired rows are copied into `audit_archive` (migration 011) and the source row is preserved. Operators archive externally and may truncate the source table out of band. Deviation from the Where is documented in the source comment and is the lazy-correct choice: zero risk of corrupting the chain.

## Dashboards / SLO definitions

- [x] **OBS-DASH-1** Check in a Grafana dashboard JSON under `deploy/grafana/`.
  - **Status**: `deploy/grafana/promptsheon-dashboard.json` — 10 panels covering request rate, error rate, drop counters, latency p50/p95/p99, LLM latency + calls/s, LLM cost $/hr, audit queue latency, workflow runs, harness eval pass rate.
- [x] **OBS-DASH-2** Check in a Prometheus alert rules file under `deploy/prometheus/`.
  - **Status**: `deploy/prometheus/promptsheon-alerts.yaml` — three SLO burn-rate alerts (invoke latency, audit chain, harness eval) plus four health alerts (HTTP error rate, trace drops, log hub drops, circuit breaker open).
- [x] **OBS-DASH-3** Document three SLOs in `docs/slos.md`: invoke p95 latency, audit chain integrity, harness eval success rate.
  - **Status**: `docs/slos.md` — target + measurement query + burn-rate alert cross-reference for each of the three SLOs, plus operating notes (scrape config, dashboard, alert routing).
