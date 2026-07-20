# Phase 12 — Go Idiomatic

Code-quality and idiomatic Go improvements. Fast forward.

## Unused imports / dead workarounds

- [ ] **GO-1a** Remove `var _ = fmt.Sprintf` from `internal/api/handlers_capabilities.go:30`.
- [ ] **GO-1b** Remove `var _ = json.Marshal` from `internal/api/handlers_harness.go:286`.
- [ ] **GO-1c** Remove `var _ = context.Background` from `internal/api/handlers_workflow.go:45`.
- [ ] **GO-1d** Remove `var _ = context.Background` from `internal/api/idempotency.go:183`.

## Context propagation

- [ ] **GO-2** Add a `context.Context` parameter to `migrate()` and propagate cancellation through `applyMigration`.
  - **Where**: `internal/store/migrate.go:32-142`.

- [ ] **GO-3** Have `*trace.SQLite.Finish` honour the passed `ctx` (currently always uses `context.Background()`).
  - **Where**: `internal/trace/sqlite.go:93, 98`.

- [ ] **GO-CTX-1** Replace `context.Background()` with a cancellable context in `alerting/manager.go` (alert persistence).
  - **Where**: `internal/alerting/manager.go`.

- [ ] **GO-CTX-2** Replace `context.Background()` with a cancellable context in `webhook/webhook.go` (endpoint register/remove).
  - **Where**: `internal/webhook/webhook.go:184, 197`.

## Error wrapping

- [ ] **GO-4a** Wrap every DB error in the SQLite implementation with `%w` and a typed sentinel (`ErrNotFound`, `ErrConflict`).
  - **Where**: `internal/store/sqlite.go`, `sqlite_capabilities.go`, `sqlite_releases.go`, `sqlite_harness.go`.

- [ ] **GO-4b** Wrap handler-returned errors with `%w` at the handler boundary.
  - **Where**: `internal/api/handlers_*.go` (every `return err` after a `s.db.*` call).

## Cardinality

- [ ] **GO-5** Remove the dormant `LabeledCounter` and `LabeledHistogram` from the public metrics package.
  - **Where**: `internal/metrics/collector.go:420-469`.

## Fuzz harnesses

- [ ] **GO-6** Wire `-fuzz` into CI nightly. (Cross-ref Phase 7 TEST-4.)

## Stale comments

- [ ] **GO-DOC-1** Fix `// WithTracing attaches a trace store and metrics collector to the server.` duplicated at `internal/api/server.go:143-144`.
- [ ] **GO-DOC-2** Update `docs/algorithms.md:312-323` to describe the actual hash format used by `AppendAudit`.

## Naming

- [ ] **GO-NAME-1** Rename `findPriorActive` to `priorActiveFor(ctx, capabilityID)` and return `*Release` instead of `[]*Release`.
  - **Where**: `internal/release/service.go:248-258`.

- [ ] **GO-NAME-2** Rename `internal/manifest` to `internal/pluginmanifest` (collision with `capability.Manifest`).
  - **Where**: directory + all importers.

- [ ] **GO-NAME-3** Rename `rules.Observation` to `observation.Observation` and move it out of `rules/`.

## Reflection / unsafe

- [ ] **GO-REF-1** Replace `reflect.DeepEqual` in `eval/scorer.go:enumContains` with a type switch over `string`/`float64`.
  - **Where**: `internal/eval/scorer.go:276-282`.

- [ ] **GO-UNS-1** Search for `unsafe.` usage and remove (none expected).

## Build tags

- [ ] **GO-BUILD-1** Add `//go:build` constraint comments to all future `*_test.go` files that need OS/arch filtering.

## Generics

- [ ] **GO-GEN-1** Add a generic `Repository[T]` interface in `internal/store/repo.go` to reduce type repetition.
  - **Where**: `internal/store/repo.go`.

## Code organization

- [ ] **GO-ORG-1** Split `internal/api/handlers_test.go` (117 KB, 3853 LOC) into per-handler `*_test.go` files.
  - **Accept**: No `internal/api/*_test.go` exceeds 30 KB.

- [ ] **GO-ORG-2** Move `internal/api/handlers_metrics.go` (UsageTracker) to `internal/api/usage.go`.
- [ ] **GO-ORG-3** Move `internal/api/idempotency.go` to `internal/api/middleware/idempotency.go`.
- [ ] **GO-ORG-4** Move `internal/api/handlers_observation.go` (one handler) to `internal/api/handlers_workspace.go`.

## Linting

- [ ] **GO-LINT-1** Bump `golangci-lint` to latest in CI.
- [ ] **GO-LINT-2** Add `gocritic` "rangeValCopy" check.
- [ ] **GO-LINT-3** Add `gocritic` "hugeParam" check.
- [ ] **GO-LINT-4** Add `gocritic` "exitAfterDefer" check.

## Testing style

- [ ] **GO-TEST-1** Adopt table-driven tests everywhere a function returns a list of errors (the codebase already does this in most places; finish the job).

## Deprecation comments

- [ ] **GO-DEP-1** Remove every `// Deprecated:` comment in the public surface (fast forward).

## Vet / staticcheck

- [ ] **GO-VET-1** Run `go vet -all ./...` in CI; fail on warnings.
- [ ] **GO-VET-2** Run `staticcheck ./...` in CI.

## Documentation

- [ ] **GO-DOC-3** Add godoc to every exported type/function that lacks one. CI lint rule.
- [ ] **GO-DOC-4** Add `// Package X` doc comment to every package that lacks one.

## Concurrency primitives

- [ ] **GO-CONC-1** Replace `sync.Mutex` with `sync.RWMutex` in `metrics.Collector`'s read paths.
  - **Where**: `internal/metrics/collector.go`.

- [ ] **GO-CONC-2** Add a benchmark that pins `usageTracker.GetTopCapabilities` latency at p99 < 5 ms for 10k capabilities.
