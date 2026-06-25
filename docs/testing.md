# Testing

Promptsheon is tested at five layers. Each layer has a clear purpose, a clear owner, and a clear pass/fail criterion. The full development workflow is in [Development](development.md); this page is the testing-specific reference.

## Layers

### 1. Unit tests

**What.** Pure functions, no I/O. The function under test, the inputs, the expected output.

**Where.** `internal/<pkg>/*_test.go`, alongside the code.

**How to run.**

```bash
go test -race -count=1 ./internal/llm/...
go test -race -count=1 -run TestRetry ./internal/llm/
```

**Style.** Table-driven, `t.Run` subtests, no test-only exports in the production code. Use `t.TempDir()` for any filesystem scratch space.

**Examples.** `internal/llm/retry_test.go`, `internal/search/bm25_test.go`, `internal/promptsheon/promptsheon_test.go`.

### 2. Integration tests

**What.** The full HTTP server, with a real SQLite database (in `t.TempDir()`), a `*slog.Logger` at error level, and a `llm.Mock` provider. No external network.

**Where.** `internal/api/api_test.go`, `internal/api/api_comprehensive_test.go`, plus per-handler `handlers_<topic>_test.go` files where they exist.

**How to run.**

```bash
go test -race -count=1 ./internal/api/...
```

**Style.** Use the helpers in `api_test.go` (`setupTestServer`, `setupTestServerWithDeps`). Always call `srv.StopAuditWorkers(ctx)` before the SQLite handle is closed.

**Example.**

```go
func TestCreatePrompt(t *testing.T) {
    srv, db := setupTestServer(t)
    t.Cleanup(func() { _ = srv.StopAuditWorkers(context.Background()); _ = db.Close() })

    body := strings.NewReader(`{"name":"greeting","content":"hi {{name}}"}`)
    req := httptest.NewRequest("POST", "/api/v1/prompts", body)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusCreated {
        t.Fatalf("got %d, body=%s", w.Code, w.Body.String())
    }
}
```

### 3. End-to-end tests

**What.** The CLI and the server together. Spins up a real `promptsheond` on a free port, runs `promptsheon` against it, asserts the result.

**Where.** `cmd/promptsheond/e2e_test.go`.

**How to run.**

```bash
go test -race -count=1 ./cmd/promptsheond/...
```

### 4. Load tests

**What.** A scripted workload against a running server, with k6 measuring latency, throughput, and error rate.

**Where.** `tests/load/`. The README in that directory is the source of truth for the scenarios.

**How to run.**

```bash
# Start the server first
PROMPTSHEON_AUTH=false ./promptsheond &

# Run the full suite
K6_BASE_URL=http://localhost:8080 k6 run tests/load/scenarios/*.js

# Or one scenario at a time
K6_BASE_URL=http://localhost:8080 k6 run tests/load/scenarios/03-prompt-write.js
```

**Style.** Thresholds are baked into the scenarios (e.g. `p(95) < 500ms`). The scenarios are also the regression baseline; if a change makes one slower, the threshold fails the build.

### 5. Security tests

**What.** Static analysis (`gosec`, `govulncheck`) plus the targeted tests for security-critical paths (audit chain tamper detection, vault zero-key rejection, SSRF policy).

**Where.** Scattered: `internal/store/store_comprehensive_test.go` for the audit chain, `internal/vault/` tests for the vault, `internal/webhook/` tests for the SSRF policy.

**How to run.**

```bash
make security
```

## Race detection

The default `make test` and CI run with `-race`. Every new test must be race-clean. The patterns that are most likely to trip the race detector:

- Audit writes. The chain head pointer is process-local; `auditMu` serialises appends.
- Auth last-used updates. Now fire-and-forget on a background goroutine — do **not** call `UpdateAPIKeyLastUsed` synchronously with `context.Background()`.
- Audit workers. The worker pool is stopped explicitly with `StopAuditWorkers`.

## Coverage

CI requires a coverage floor of 70%. The exact percentage is checked by parsing the output of `go tool cover -func=coverage.out` in `.github/workflows/ci.yaml`.

To see the per-function breakdown:

```bash
make coverage-raw
```

## What to test when you add a feature

| You added… | Test in… |
|---|---|
| A new public function in any `internal/<pkg>/` | A new `*_test.go` in the same package. |
| A new HTTP route | A new test in `internal/api/api_test.go` that exercises the route end-to-end. |
| A new handler | A new test in the relevant `handlers_<topic>_test.go` (or a new file). |
| A new audit entry shape | A new test in `internal/store/` that verifies the chain still verifies after your change. |
| A new env var | A new test in `internal/config/config_test.go` that exercises the default, the override, and any validation rule. |
| A new migration | A new test that creates a database at the previous version and applies the new migration, then runs a smoke query. |
| A new dependency | A new test that mocks the dependency and asserts the integration point. |

## What not to do

- Do not write tests that require network access. Mock the network. (`llm.NewMock` is the standard pattern.)
- Do not use `t.Parallel()` unless you are sure the test is independent. The audit chain is process-local; parallel tests that share a server can race.
- Do not use `time.Sleep` for synchronization. Use channel receives, condition variables, or the request lifecycle.
- Do not skip `t.Cleanup`. Every test that opens a server, a database, or a goroutine must close it.
- Do not commit commented-out tests. Delete them; git remembers.

## See also

- [Development](development.md) — Make targets, helpers, CI
- [Security](security.md) — the threat model that motivates the security tests
- [Modules](modules.md) — package boundaries that shape the test layout
