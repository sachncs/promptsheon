# Phase 7 — Testing

All testing findings. Fast forward: add, don't legacy.

## Existing test fixes

- [x] **TEST-2** Fix the vacuous precondition test in `release/service_test.go:297-329`.
  - **What**: Set `t.Setenv("PROMPTSHEON_HARNESS_PRECONDITIONS", "true")` so the precondition actually runs.
  - **Accept**: Removing the precondition's success branch makes the test fail.

- [ ] **TEST-3** Consolidate the two in-memory `harness.Repository` fixtures into `internal/testutil/harnessrepo`.

- [ ] **TEST-1** Expand `capability/manifest_test.go` to cover `DeriveState` (after DEAD-1c removes it, no test needed), `Workspace`, `Project`, `Event`, `Execution`, `Recommendation`.

- [ ] **DEAD-E2E-1** Deduplicate env vars in `tests/e2e/daemon_e2e_test.go:119-126`.
  - **Accept**: Each env var appears exactly once per `t.Setenv` call.

## CI integration

- [ ] **TEST-5** Add an e2e job to CI that runs `make test-e2e`.
  - **Where**: `.github/workflows/ci.yaml`.

- [ ] **TEST-4** Add a nightly fuzz job that runs `-fuzz=...` for at least 60 s on each fuzz harness.
  - **Where**: `pkg/cas/fuzz_test.go`, `internal/vault/fuzz_test.go`, `internal/schedule/fuzz_test.go`, `internal/injection/fuzz_test.go`, `internal/redactor/fuzz_test.go`. Add a new `.github/workflows/nightly-fuzz.yaml`.

- [ ] **TEST-6** Add SDK CI: `pytest sdk/python/tests/` and `npm test` for the TypeScript SDK.
  - **Where**: `.github/workflows/ci.yaml`.

- [ ] **TEST-7** Have `nightly-load.yaml` assert on k6 summary thresholds; fail the job when p95 > 3 s sustained.
  - **Where**: `.github/workflows/nightly-load.yaml`.

- [ ] **API-9** Add a contract test that round-trips every OpenAPI route via the Go SDK. (Cross-ref Phase 4.)

## Coverage

- [ ] **TEST-COV-1** Raise the global coverage gate to 70% (matches `docs/development.md`).
  - **Where**: `.github/workflows/ci.yaml:62`.

- [ ] **TEST-COV-2** Add per-package coverage floors: domain packages ≥ 90%, infrastructure ≥ 70%, API handlers ≥ 80%.
  - **Where**: new `scripts/check-coverage.sh`.

- [ ] **TEST-COV-3** Add the SDK and CLI to the coverage umbrella.

## New tests to write

- [ ] **TEST-NEW-1** Migration tests for 050, 051, 052, and every future migration that actually executes them.
- [ ] **TEST-NEW-2** API contract tests for every 87 route.
- [x] **TEST-NEW-3** Bootstrap race test: 100 goroutines hit `POST /api/v1/setup`; assert exactly one wins.
- [x] **TEST-NEW-4** API-key revoke-on-update test: change user role; existing key returns 401.
- [ ] **TEST-NEW-5** Webhook SSRF test: every URL in a deny list returns 400.
- [x] **TEST-NEW-6** Webhook secret encryption test: `SELECT secret FROM webhook_endpoints` returns ciphertext.
- [x] **TEST-NEW-7** Maker-Checker test: release creator's vote is rejected at the type level.
- [ ] **TEST-NEW-8** Audit chain tail test: delete last 5 rows; verifier reports `TailMismatch`.
- [ ] **TEST-NEW-9** Trace write test: 1000 concurrent requests don't block on trace SQLite.
- [ ] **TEST-NEW-10** Idempotency multi-replica test: two replicas share idempotency state via SQLite.
- [ ] **TEST-NEW-11** SSE log stream test: `curl` against the endpoint receives log lines.
- [ ] **TEST-NEW-12** OTel span test: a request produces a span that lands in the OTLP test collector.

## Test infrastructure

- [ ] **TEST-INFRA-1** Add `internal/testutil/sqlite.go` with a `NewTestDB(t)` helper that gives every test a fresh, migrated DB.
  - **Accept**: No test creates its own `*sql.DB`.

- [ ] **TEST-INFRA-2** Add `internal/testutil/http.go` with a `NewTestServer(t, opts...)` helper that constructs an `*api.Server` with an in-memory SQLite.
  - **Accept**: No test instantiates a `*api.Server` directly.

- [ ] **TEST-INFRA-3** Add `internal/testutil/clock.go` with a `ClockFunc` test seam used everywhere `time.Now` appears.

- [ ] **TEST-INFRA-4** Add `internal/testutil/otel.go` with an in-memory OTLP collector.

## Race / chaos / property

- [ ] **TEST-RACE-1** Add a `-race` nightly job across the matrix (`go test -race -count=10 ./...`).
  - **Where**: `.github/workflows/ci.yaml`.

- [ ] **TEST-CHAOS-1** Add chaos tests: kill SQLite mid-request, kill plugin subprocess, drop webhook network. (Cross-ref Phase 6 OPS-DR-2.)

- [ ] **TEST-PROPERTY-1** Add property-based tests for the CAS layer (`pkg/cas/`) using `testing/quick` or `pgregory.net/rapid`.

- [ ] **TEST-MUT-1** Add mutation testing with `go-mutesting` for the domain packages.

## Test docs

- [ ] **TEST-DOC-1** Rewrite `docs/testing.md` to match the new test layers (unit / integration / contract / e2e / load / fuzz / chaos / property / mutation) and document which CI job runs each.
- [ ] **TEST-DOC-2** Document the coverage gate, per-package floors, and how to read `coverage.html`.

## CLI tests

- [ ] **TEST-CLI-1** Add unit tests for `cmd/promptsheon/{cas,harness,http}.go` covering each subcommand.
  - **Where**: `cmd/promptsheon/main_test.go` (extend) and per-file tests.
