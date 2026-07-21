# Phase 0 — Critical Blockers

Ship-blockers. Fix these before anything else. Fast forward, no backward compat, breaking changes welcome.

## Migrations that block boot

- [x] **DB-1a** Rewrite migration 050 in SQLite-compatible form.
  - **What**: Replace `ALTER TABLE ... ADD CONSTRAINT ... CHECK(...)` (unsupported in SQLite) with `CREATE TABLE new_xxx AS SELECT ... FROM xxx; DROP TABLE xxx; ALTER TABLE new_xxx RENAME TO xxx;` and the CHECK baked into the new CREATE TABLE.
  - **Where**: `internal/store/migrations/050_status_checks.up.sql`.
  - **Accept**: A fresh DB seeded only with `001..049` runs `050` without error; `sqlite_master` shows the CHECK clauses; the matching `050_status_checks_test.go` runs the migration against a real DB.

- [x] **DB-1b** Update migration test 050 to actually execute 050, not inspect pre-seeded schema.
  - **Where**: `internal/store/050_status_checks_test.go`.
  - **Accept**: Test starts from a DB at v49, applies 050, then asserts CHECK rejections.

- [x] **DB-2a** Replace the file-name heuristic with a content-scan that catches `044_legacy_drop`.
  - **What**: `isDestructiveMigration` reads the file and looks for `DROP TABLE` or `DROP COLUMN` SQL tokens; or rename `044_legacy_drop.up.sql` to `044_destructive_legacy_drop.up.sql` and keep the file-name heuristic.
  - **Where**: `internal/store/migrate.go:144-152` and `internal/store/migrations/044_legacy_drop.up.sql`.
  - **Accept**: Booting without `PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true` refuses to run 044.

- [x] **DB-3a** Move FK-disable pragma outside the migration transaction.
  - **What**: `applyMigration` is split into `applyMigration(db, version, sql, runOutOfTx func)`; 043 uses the out-of-tx variant and runs `PRAGMA foreign_keys=OFF` before, `PRAGMA foreign_keys=ON` after.
  - **Where**: `internal/store/migrate.go:117-142` and `internal/store/migrations/043_fk_hygiene.up.sql`.
  - **Accept**: Running 043 on a DB with parent rows preserves all child rows; running it on a DB whose parents were deleted directly still cascades as expected.

- [ ] **DB-3b** Add a migration test that populates parent+child rows, applies 043, asserts no cascade loss.
  - **Where**: new test in `internal/store/043_fk_hygiene_test.go`.

## Trace write on hot path

- [x] **OBS-1a** Move `*trace.SQLite.Finish` off the request goroutine.
  - **What**: Introduce a bounded worker pool (size 8, queue 4096) that drains finished spans and writes them in batches every 250 ms or on overflow.
  - **Where**: `internal/trace/sqlite.go:93-105` and `internal/metrics/middleware.go:58-60`.
  - **Accept**: `go test -race -bench=BenchmarkHTTPMiddleware -benchtime=10s ./internal/metrics` shows no per-request synchronous DB write; p99 latency under load improves.

- [x] **OBS-1b** Export `auditDropped` and `traceDropped` counters to the Prometheus exposition.
  - **Where**: `internal/api/server.go:614, 668-697` and `internal/metrics/collector.go`.

## OTel wiring

- [x] **OBS-2a** Wire `*trace.OTelTracer` into the HTTP middleware alongside the SQLite tracer.
  - **What**: `api.NewServer` takes an optional `ot tracer.Tracer`; `metrics.HTTPMiddleware` wraps the SQLite tracer and forwards every span to OTel.
  - **Where**: `cmd/promptsheond/main.go:91-103, 440` and `internal/metrics/middleware.go:58-60`.
  - **Accept**: With `PROMPTSHEON_OTEL_ENDPOINT=otel:4317`, spans appear in the collector; without it, the SQLite tracer still works.

- [x] **OBS-3a** Fix `OTelTracer.Finish` to call `otelSpan.End()`.
  - **Where**: `internal/trace/otel.go:88-95`.
  - **Accept**: An OTel span created via `OTelTracer.Start` is recorded with the correct duration when `Finish` is called.

## SSE log stream wiring

- [x] **OBS-4a** Wrap `ws.streamHandler` into the slog chain in `setupLogger`.
  - **Where**: `cmd/promptsheond/main.go:178-193` and `internal/ws/hub.go:198-204`.
  - **Accept**: `curl /api/v1/logs/stream` receives every log line the daemon emits in real time.

## OAuth wiring

- [x] **OBS-OAuth-1** Construct `*auth.OAuthManager` from env vars in `buildServer`.
  - **What**: Read `PROMPTSHEON_OAUTH_GOOGLE_*` and `PROMPTSHEON_OAUTH_GITHUB_*`; build a manager with both providers; wire via `api.WithOAuth`.
  - **Where**: `cmd/promptsheond/main.go:365-388`, `internal/auth/oauth.go`, `internal/api/server.go:166-171`.
  - **Accept**: With OAuth env vars set, `GET /api/v1/auth/google/login` returns a valid Google authorize URL.

## ClickHouse wiring

- [ ] **OBS-CH-1** Wire `rollups/clickhouse.Writer` into the aggregator when `PROMPTSHEON_CLICKHOUSE_DSN` is set.
  - **What**: `cmd/promptsheond/main.go:300` constructs `clickhouse.Writer` if DSN is non-empty; passes it to `rollups.Aggregator` as the backend.
  - **Where**: `cmd/promptsheond/main.go:300`, `internal/rollups/clickhouse/writer.go`, `internal/rollups/rollups.go`.
  - **Accept**: With `PROMPTSHEON_CLICKHOUSE_DSN=clickhouse://...`, rollup rows land in the `workspace_rollups` table; without it, in-memory aggregation continues.

## genopenapi parser

- [x] **API-1a** Extend `scripts/genopenapi` to walk every `register*Routes()` method on `*Server`, not just the routes registered in `server.go`.
  - **What**: AST visitor finds every `s.mux.HandleFunc(...)` call inside `internal/api/*.go` and parses route patterns + handler names.
  - **Where**: `scripts/genopenapi/main.go` (921 LOC).
  - **Accept**: `make openapi && git diff` shows no route additions after the fix; `api/openapi.yaml` documents all 87 routes.

## OpenAPI version drift

- [x] **DOC-2a** Set `api/openapi.yaml:version` to `0.1.0` to match the product.
  - **Where**: `api/openapi.yaml:11`.
  - **Accept**: `make openapi-check` passes; the generator template emits `0.1.0` as a constant pulled from a single source of truth.

## Domain wiring

- [x] **OBS-12a** Wire `recommendation.Producer` against the observation aggregator.
  - **Where**: `cmd/promptsheond/main.go:318-323`.
  - **Accept**: After a successful invoke, a recommendation row appears in the `recommendations` table within one producer tick (5 s default).

- [ ] **OBS-10a** Add an SLO evaluator goroutine that periodically calls `Evaluate` against the metrics collector.
  - **Where**: new file `internal/slo/evaluator.go`; wired in `cmd/promptsheond/main.go`.
  - **Accept**: A breach on `promptsheon_review_pending` produces a `BreachEvent` within one tick; the bridge turns it into a Recommendation.

- [x] **OBS-Lineage-1** Implement `lineage.Repository` against SQLite.
  - **Where**: `internal/lineage/sqlite.go` (new) and `internal/store/migrations/053_lineage.sql` (new).
  - **Accept**: After a recommendation is approved, `lineage.Graph.GetGraph(capabilityID)` returns the prior→new edge.
