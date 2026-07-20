# Phase 11 — Performance

Hot-path and allocation improvements. Fast forward.

## Trace writes (Critical)

- [ ] **PERF-1** Move trace writes off the request path. (See Phase 0 OBS-1a.)

## Audit pipeline

- [ ] **PERF-2** Export `auditDropped` and `traceDropped` counters to Prometheus. (See Phase 3 OBS-7.)

## Histogram quantiles

- [ ] **PERF-3a** Replace `Percentile`'s O(N log N) sort with a fixed-bucket t-digest.
  - **Where**: `internal/metrics/collector.go:97-114`.

- [ ] **PERF-3b** Add a benchmark that pins the percentile cost at p99 of < 1 ms for 10k samples.
  - **Where**: `internal/metrics/collector_test.go`.

## Idempotency body hash

- [ ] **PERF-7a** Replace `buf.ReadFrom(r.Body)` with a streaming SHA-256 hasher so memory stays at O(64 bytes).
  - **Where**: `internal/api/idempotency.go:138-143`.

- [ ] **PERF-7b** Add a benchmark showing constant memory under 100 MB body loads.

## Lock contention

- [ ] **PERF-4a** Shard `DefaultEnforcer` by workspace hash so the global mutex becomes per-shard.
  - **Where**: `internal/invoke/invoke.go:118-183`.

- [ ] **PERF-4b** Replace `observation.Aggregator`'s global mutex with per-key locks.
  - **Where**: `internal/observation/observation.go:55-58`.

- [ ] **PERF-4c** Replace `UsageTracker.capabilityUsage`'s single map with a sync.Map or per-tenant map.
  - **Where**: `internal/api/handlers_metrics.go:15-53`.

## Bounded structures

- [ ] **PERF-5** Add an LRU eviction policy to `UsageTracker.capabilityUsage`.
- [ ] **PERF-5b** Add a per-tenant memory ceiling to the observation aggregator.

## Database

- [ ] **PERF-DB-1** Add prepared statements for hot read paths (`GetRelease`, `GetCapability`, `GetAPIKeyByHash`, `ListExecutions`).
  - **Where**: `internal/store/sqlite*.go`.

- [ ] **PERF-DB-2** Use SQLite `RETURNING` clauses to avoid round-trips on `INSERT`.
  - **Where**: `internal/store/sqlite*.go`.

- [ ] **PERF-DB-3** Add a benchmark for `eval` runs with 1000 cases; assert under 30 s on a 4-core box.

## Eval pipeline

- [ ] **PERF-EVAL-1** Parallelise `EvalRunner.Run` across cases via a worker pool (size = min(cases, NumCPU/2)).
  - **Where**: `internal/harness/runner.go:52-116`.

- [ ] **PERF-EVAL-2** Stream eval results to the DB instead of buffering the whole run before insert.

## LLM client

- [ ] **PERF-LLM-1** Add `http.Transport` with tuned `MaxIdleConnsPerHost` for the OpenAI / Anthropic clients.
  - **Where**: `internal/llm/openai.go`, `internal/llm/anthropic.go`.

- [ ] **PERF-LLM-2** Add `slog` debug log when the LLM call takes > 5 s.

## HTTP server

- [ ] **PERF-HTTP-1** Use `http.Server.ReadHeaderTimeout` correctly (already set). Verify the `IdleTimeout` and `MaxHeaderBytes`.
  - **Where**: `cmd/promptsheond/main.go:445-457`.

- [ ] **PERF-HTTP-2** Add `http2.ConfigureTransport` for h2c support in development.

## Benchmarks

- [ ] **PERF-BENCH-1** Run `go test -bench=. -benchmem ./...` in CI nightly; archive results.
  - **Where**: `.github/workflows/nightly-load.yaml`.

- [ ] **PERF-BENCH-2** Add a regression threshold: any benchmark that regresses > 20% fails the build.
  - **Where**: `scripts/check-bench-regression.sh` (new).

## Memory

- [ ] **PERF-MEM-1** Add `runtime.MemStats` snapshotting to `/api/v1/health` (debug-only).
- [ ] **PERF-MEM-2** Run `pprof` collection in the daemon when `PROMPTSHEON_PPROF=true`; expose `/debug/pprof/` on a separate loopback listener.
  - **Where**: `cmd/promptsheond/main.go`.

## Scheduler

- [ ] **PERF-SCH-1** Have `Scheduler.TickOnce` use a single bulk `UPDATE` instead of per-row.
  - **Where**: `internal/scheduler/scheduler.go:60-84`.

## Audit chain verification

- [ ] **PERF-AUDIT-1** Cache the last verified `(rowid, hash)` so `VerifyAuditChain` only walks new rows on repeat calls.
  - **Where**: `internal/store/sqlite.go:190-214`.

## Webhook delivery

- [ ] **PERF-WH-1** Use `sync.Pool` for the HTTP client transport in the webhook dispatcher.
  - **Where**: `internal/webhook/webhook.go:304-329`.

- [ ] **PERF-WH-2** Honour `Retry-After` from webhook receivers.
  - **Where**: `internal/webhook/webhook.go:364-396`.
