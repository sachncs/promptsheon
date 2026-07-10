# Development

This page is the contributor's handbook. It assumes you have read [Architecture](architecture.md) and [Modules](modules.md) and have a passing familiarity with the package layout.

## Prerequisites

- **Go 1.26 or later.** Earlier versions will not compile.
- **golangci-lint v1.60+.** Optional in the strictest sense (CI runs it) but expected on every developer machine.
- **k6.** For load testing. See [Load testing](#load-testing).
- **govulncheck.** For the `make security` target. `go install golang.org/x/vuln/cmd/govulncheck@latest`.

The Go module has one direct dependency: `modernc.org/sqlite`. There is no `go.sum` churn to manage by hand; `go mod tidy` is enough.

## Clone and build

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon
```

Both binaries are statically linked and need no C toolchain. Cross-compilation works with the standard `GOOS`/`GOARCH` env vars.

## Repository layout

```
promptsheon/
├── cmd/
│   ├── promptsheond/          # server daemon (main.go, e2e_test.go)
│   └── promptsheon/           # CLI client
├── internal/                  # server code, see docs/modules.md
│   ├── api/                   # HTTP handlers, middleware, server.go
│   ├── ...                    # see modules.md
│   └── promptsheon/           # CAS engine used by the CLI
├── sdk/                       # Go client SDK
├── scripts/
│   └── genopenapi/            # OpenAPI generator (AST-based)
├── api/
│   └── openapi.yaml           # generated, do not hand-edit
├── test/                      # integration tests
├── tests/
│   └── load/                  # k6 load tests
├── docs/                      # this documentation
├── .github/
│   └── workflows/ci.yaml      # CI
├── .golangci.yml
├── Makefile
└── go.mod
```

## Code style

- Follow [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- `gofmt -s` and `goimports` are mandatory. CI rejects PRs with unformatted code.
- `golangci-lint run` must pass. The enabled linters are listed in `.golangci.yml`.
- Every exported type and function must have a GoDoc comment.
- Tests are table-driven where it makes sense. Use `t.Run` subtests.
- Error wrapping: `fmt.Errorf("context: %w", err)`. Never `errors.New(fmt.Sprintf(...))` to wrap.

## Test helpers

The HTTP-server tests share a small set of helpers in `internal/api/`:

| Helper | Where | Purpose |
|---|---|---|
| `setupTestServer(t)` | `internal/api/api_test.go` | Builds a `*Server` backed by a temp-dir SQLite, with a `*slog.Logger` at error level and a `llm.Mock` provider. Starts the audit worker pool. |
| `setupTestServerWithDeps(t)` | `internal/api/api_comprehensive_test.go` | Same as `setupTestServer` but with extra collaborators (guardrails, search index) wired in. |
| `setupTestServerMinimal(t)` | `internal/api/api_comprehensive_test.go` | Bare-bones server for handler-only tests; no workers, no mock LLM. |
| `srv.StopAuditWorkers(ctx)` | `internal/api/server.go` | Drains the in-process audit queue and stops the worker goroutines. Call this before the SQLite handle is closed to avoid a race. |

A minimal test:

```go
func TestCreatePrompt(t *testing.T) {
    srv, db := setupTestServer(t)
    defer srv.StopAuditWorkers(context.Background())
    defer db.Close()

    body := `{"name":"greeting","content":"hi {{name}}"}`
    req := httptest.NewRequest("POST", "/api/v1/prompts", strings.NewReader(body))
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    if w.Code != http.StatusCreated { t.Fatalf("got %d", w.Code) }
}
```

## Testing strategy

We use a layered approach. The full taxonomy is in [Testing](testing.md); the short version:

| Layer | What | Where |
|---|---|---|
| Unit | Pure functions, no I/O | `internal/<pkg>/*_test.go` |
| Integration | Full server, in-memory DB | `internal/api/api_test.go` |
| End-to-end | CLI and server together | `cmd/promptsheond/e2e_test.go` |
| Race | All tests with `-race` | `make test` |
| Load | k6 scenarios | `tests/load/` |
| Security | govulncheck, gosec | `make security` |

The CI matrix runs `go test -race -count=1 -timeout 120s -coverprofile=coverage.out ./...` and fails if coverage drops below 70%.

## Make targets

| Target | What |
|---|---|
| `build` | Build both binaries. |
| `build-server`, `build-cli` | Build one binary. |
| `test` | `go test -race -count=1 ./...` |
| `test-verbose` | Same, with `-v`. |
| `test-integration` | `go test -v -race ./test/...` |
| `lint` | `golangci-lint run` |
| `fmt` | `gofmt -s -w .` and `goimports -w .` |
| `vet` | `go vet ./...` |
| `deps` | `go mod download` and `go mod verify` |
| `clean` | Remove built binaries and `*.db` files. |
| `coverage` | Generate HTML coverage report. |
| `coverage-raw` | Show coverage per function in the terminal. |
| `run` | `go run ./cmd/promptsheond` |
| `cli` | `go run ./cmd/promptsheon` |
| `openapi` | Regenerate `api/openapi.yaml` from the server routes. |
| `openapi-check` | Same, then `git diff --exit-code`. Used in CI. |
| `update-deps` | `go get -u ./...` and `go mod tidy` |
| `security` | `govulncheck ./...` |
| `help` | List targets. |

## OpenAPI generator

`scripts/genopenapi/` is a small Go program that walks the server's `internal/api/server.go` for `mux.HandleFunc` calls and the corresponding handler functions in `internal/api/handlers_*.go`. For each route, it emits an OpenAPI 3.0 path entry with a real request body schema (extracted from the handler's first argument via the `go/parser` AST).

### Usage

```bash
make openapi          # writes api/openapi.yaml
make openapi-check    # writes it and fails if the file is dirty

# Or run the tool directly
go run ./scripts/genopenapi -out api/openapi.yaml
go run ./scripts/genopenapi -dry-run
```

### What it does not do

- It does not read runtime routes. Only routes that are registered as `mux.HandleFunc("METHOD", "/path", handlerName)` are captured.
- It does not introspect response types. Each route gets a default `200` response with a generic `application/json` body. This is deliberate — response shapes are documented in [API Reference](api-reference.md) and verified by integration tests.
- It does not require running the server. The tool is a pure AST consumer and is trivially testable.

When you add a new route:

1. Register it in `internal/api/server.go`.
2. Implement the handler in the appropriate `handlers_<topic>.go` file.
3. Run `make openapi` and commit the regenerated `api/openapi.yaml`.
4. Add an integration test in `internal/api/api_test.go` or a `handlers_<topic>_test.go`.

CI runs `make openapi-check` on every PR. If the spec is out of date, the PR fails.

## Migrations

Database schema is in `internal/store/migrations/`, applied by the migration runner in `internal/store/sqlite.go` at server startup. Each migration is a single SQL file with the format `NNN_description.sql`. The runner records applied migrations in the `schema_migrations` table and skips already-applied ones.

To add a migration:

1. Create `internal/store/migrations/NNN_description.sql` with the next available number.
2. Use `IF NOT EXISTS` clauses for any `CREATE TABLE` / `CREATE INDEX` so re-running a partially-applied migration is safe.
3. For destructive changes (column drop, type change), add a new migration and keep both schemas working until the previous one is fully drained.
4. Never edit a migration that has shipped. Write a new one.

## Continuous integration

`.github/workflows/ci.yaml` runs on every push and PR:

1. `go vet ./...`
2. `go test -race -count=1 -timeout 120s -coverprofile=coverage.out ./...`
3. `gofmt -l .` — fails the build if any file is unformatted.
4. Coverage threshold check (≥ 70%).
5. `make openapi-check` — fails the build if the spec is out of date.
6. (Future) `make security` — `govulncheck ./...`.

## Release process

Releases are cut with GoReleaser (`.goreleaser.yml` at the repo root). The version is taken from the most recent semver tag. The `CHANGELOG.md` entry is the only manual step; everything else is automated.

## See also

- [Testing](testing.md) — full test taxonomy
- [Architecture](architecture.md) — system overview
- [Modules](modules.md) — package map
- [Security](security.md) — threat model and checklist
- ADR [0006](adr/0006-modernc-sqlite-no-cgo.md), [0007](adr/0007-slog-as-observability-foundation.md)
