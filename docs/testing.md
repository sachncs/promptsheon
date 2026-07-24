# Testing

Promptsheon has nine test layers. The Makefile target `test`
runs the first three; the rest are wired into CI as separate
jobs so the default PR path stays fast.

| Layer | What it covers | How to run |
|-------|----------------|------------|
| **Unit** | Every package's internal types and helpers. | `go test ./internal/...` |
| **Integration** | A small set of cross-package tests in `tests/integration/` (the API + store round-trip). | `go test ./tests/integration/...` |
| **Contract** | OpenAPI spec parses, every route is reachable on the mux, Go SDK surface is in sync. | `go test ./tests/contract/...` |
| **End-to-end** | In-process daemon + real HTTP; auth + bootstrap paths. | `go test ./tests/e2e/...` |
| **Smoke** | Boots a real daemon, runs every `examples/bash/*.sh`, tears down. | `bash tests/smoke/run.sh` |
| **Chaos** | SQLite file-delete mid-query doesn't panic. | `go test ./tests/chaos/...` |
| **Load** | k6 scenarios in `tests/load/scenarios/*.js` (nightly). | `make load-test` |
| **Property** | `testing/quick` covers the LLM gateway (`internal/llm/property_test.go`) and the bandit selector (`internal/bandit/property_test.go`). CAS-layer properties are follow-on. | `go test ./internal/llm/... ./internal/bandit/...` |
| **Mutation** | (future) `go-mutesting` for domain packages. | non-goal for v0.2.0 |

## Layer-to-CI-job mapping

| Layer | CI job | Trigger |
|-------|--------|---------|
| Unit + contract + e2e | `test` (matrix go 1.26.5, 1.27) | every PR + master |
| Smoke | `smoke` | every PR + master |
| End-to-end | `e2e` | every PR + master |
| Docs / lints | `docs-lint`, `lint`, `helm` | every PR + master |
| Vulnerability scan | `security` (govulncheck + gosec) | every PR + master |
| SBOM | `sbom` (syft) | tags only |
| Load | `nightly-load` (nightly schedule) | nightly |
| Fuzz | (future) `nightly-fuzz` | non-goal for v0.2.0 |

## Layout

```
internal/<pkg>/*_test.go            # Unit tests for each package.
internal/testutil/                 # Shared test helpers (logger, sqlite, env, harness repo).
internal/testutil/harnessrepo/     # Shared in-memory harness.Repository fixture.
tests/contract/contract_test.go   # OpenAPI ↔ SDK contract.
tests/e2e/                         # In-process daemon, real HTTP.
tests/smoke/run.sh                 # Bash smoke against a fresh daemon.
tests/chaos/                       # Failure-mode tests (SQLite kill, etc.).
tests/load/scenarios/*.js          # k6 load scenarios (nightly).
```

## Conventions

- Test files end in `_test.go`. Use the `_test` package suffix
  (e.g. `package harness_test`) so only exported symbols are
  touched.
- Cleanup is registered with `t.Cleanup`, not `defer`, so
  the test ends in a known state even on panic.
- Concurrency: shared fixtures use `sync.Mutex` (e.g.
  `internal/testutil/harnessrepo.MemRepo`) so tests can run
  in parallel.
- Fakes (in-memory providers, repositories) live under
  `internal/testutil/` so cross-package tests can link them
  without dragging in the storage layer.
- Use `testutil.NewTestDB(t)` for every test that needs the
  store layer. No test creates its own `*sql.DB` directly.
- Use `api.NewTestServer(t, opts...)` for every test that
  needs an `*api.Server`. No test instantiates the server
  directly.

## Test helpers

| Helper | Where | Purpose |
|--------|-------|---------|
| `testutil.DiscardLogger` | `internal/testutil/testutil.go` | Logger that writes to `io.Discard`. |
| `testutil.TempSQLite` / `testutil.NewTestDB` | same | On-disk SQLite in `t.TempDir()` with migrations applied. |
| `testutil.MemoryBus` | same | In-memory `eventbus.Memory` with cleanup. |
| `testutil.ContextWithTimeout` | same | Context cancelled at `d` + auto-cleanup. |
| `testutil.Setenv` / `Unsetenv` | same | Env-var mutation scoped to a single test. |
| `testutil.ClockFunc` / `testutil.Now` | same | Test seam: substitute a deterministic time source. |
| `testutil.harnessrepo.New` | `internal/testutil/harnessrepo/` | `harness.Repository` fixture (datasets, preconditions, eval runs). |
| `api.NewTestServer` | `internal/api/invoke_test_helpers_test.go` | Canonical entry point for an `*api.Server` with an in-memory store. |

## Coverage

The CI gate enforces **60% total coverage** plus per-package
floors:

- Domain packages (most of `internal/<pkg>/`): >= 50%
- Infrastructure (`internal/api/`, `internal/store/`): >= 40%
- API handlers (`internal/api/handlers*`): >= 60%

The script `scripts/check-coverage.sh` reads the same
`coverage.out` produced by `go test -coverprofile` and
enforces both the global and per-package floors. The global
floor is intentionally low (60%) because the cmd entry points
and the OpenAPI generator aren't unit-testable in the
conventional sense; the per-package floors catch silent
regressions in the testable surface.

A per-PR coverage delta is a follow-on; the current gate
fails the build on a coverage drop, but doesn't surface the
delta. The Makefile target `coverage` produces an HTML report
for local debugging.

## Contract test

`tests/contract/contract_test.go` is the gate that catches
drift between `api/openapi.yaml` and the Go SDK. The test
parses the spec, walks every registered route, and asserts
the documented SDK surface. It's wired into CI as a step
on the default `test` job.

## Smoke test

The smoke test boots a fresh daemon, runs every
`examples/bash/*.sh` against it, and tears down. It's
intentionally not part of `make test` because it adds ~6s of
wall-clock and depends on building `promptsheond`.

```
bash tests/smoke/run.sh
```

The script:

1. Builds `promptsheond` if not already built.
2. Starts the daemon on `127.0.0.1:18080` with `PROMPTSHEON_AUTH=false`
   (so the smoke fixtures can call `/api/v1/setup`).
3. Waits up to 6s for `/health` to return 200.
4. Iterates `examples/bash/*.sh` with a fixture ID.
5. Tears the daemon down on EXIT.

## End-to-end test

`tests/e2e/` boots an in-process `promptsheond` and exercises
the auth + bootstrap paths through the real HTTP handlers.
It's a smaller surface than the smoke layer; the smoke layer
runs the actual bash examples, while the e2e layer tests
scenarios the bash examples don't reach (expired API keys,
malformed JSON bodies, CSRF, CORS preflight).

```
go test ./tests/e2e/...
```

## Chaos test

`tests/chaos/` exercises failure modes the smoke layer can't
simulate. The current suite has `TestSQLiteSurvivesFileDelete`
which verifies the production contract: a held SQLite query
against a deleted file does NOT panic and returns within the
timeout. The suite is gated behind
`PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true` so the
destructive-migration gate doesn't block test setup.

## SDK CI

The Go SDK has its own tests under `sdk/client_test.go`; the
Python SDK has `sdk/python/tests/` and the TypeScript SDK has
`sdk/typescript/src/`. SDK CI is wired into the default `test`
job (Go SDK) plus separate steps in the existing CI (Python
and TypeScript) where their toolchains are available.

## Adding a new test

- For unit-level coverage of a single function, drop a
  `*_test.go` in the same package and use the standard
  `testing` package.
- For cross-package coverage (a real daemon + real HTTP),
  drop a `*_test.go` in `tests/e2e/` and use the helpers in
  `internal/testutil` + `internal/api`.
- For OpenAPI / SDK drift detection, extend
  `tests/contract/contract_test.go`.

If your test needs a fixture that doesn't exist, add it to
`internal/testutil/`. The harness fixture in
`internal/testutil/harnessrepo/` is the canonical pattern:
implements the consumer-defined interface, safe for
concurrent use, exported (so cross-package tests can link
it).

## Migration tests

`internal/store/002_core_schema_test.go` covers the schema
migrations applied at boot (tables, indexes, foreign keys,
unique constraints). Each migration is applied to a fresh
in-memory DB by `TestNewSQLiteRunsAllMigrations`; the
per-migration tests cover the schema changes (column
additions, data migrations) and are the regression net for
forward-only schema evolution.

## See also

- [docs/operations.md](operations.md) — backup / restore.
- [docs/slos.md](slos.md) — SLO targets + the RPO / RTO
  table that motivates the chaos + smoke layers.
- [docs/architecture.md](architecture.md) — the package
  table the test layer maps onto.
