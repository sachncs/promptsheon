# Development

This page covers the day-to-day development workflow: how to
build, test, regenerate the OpenAPI spec, and run the daemon
locally. The contribution policy is in
[`CONTRIBUTING.md`](../CONTRIBUTING.md).

## Requirements

- **Go 1.26+** (see `go.mod`).
- A POSIX shell (the smoke test is bash).
- `make` (the Makefile is the entry point for the common
  workflows).
- `golangci-lint` (see `.golangci.yml`).

## Build

```bash
# Build all three binaries.
make build

# Or build individually.
go build -o promptsheond     ./bin/promptsheond
go build -o promptsheon      ./bin/promptsheon
go build -o bin/promptsheon-healthcheck .

# Build with the optional ClickHouse rollup writer.
go build -o bin/promptsheond .
```

## Test

```bash
# Run unit + contract + e2e tests.
make test

# Run with coverage.
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run the contract test alone.
go test ./tests/contract/...

# Run the smoke test (boots a real daemon).
bash tests/smoke/run.sh
```

The CI pipeline runs `go test -race -count=1 -coverprofile` plus
the smoke layer. Coverage floor is 60%; PRs that drop below are
flagged.

## Lint

```bash
# Format, vet, and lint individually.
make fmt
make vet
make lint
```

The project enforces a gofmt-clean tree (`gofmt -l .` must
return no files) and a `go vet` pass on every commit. Domain
packages are kept infra-free (no imports from `backend/store`
or anything that pulls in `package main` at the repo root);
`make lint-domain` and `make lint-deps` enforce that.

## Run the daemon locally

```bash
# Run on the default address.
make run

# Or with explicit env vars.
PROMPTSHEON_ADDR=":8080" \
PROMPTSHEON_AUTH=false \
PROMPTSHEON_LOG_LEVEL=debug \
./promptsheond
```

The first run creates `promptsheon.db` in the current
directory. `rm -f promptsheon.db*` to start fresh.

## Regenerate the OpenAPI spec

```bash
make openapi
# Equivalent:
go run ./scripts/genopenapi
```

The generator walks every `register*Routes()` method on
`api.Server` and emits a per-route entry with the request
struct fields (when extractable from the AST). The output is
deterministic; running it twice produces byte-identical
output. CI fails if the committed `backend/spec/spec.yaml` is
stale.

When you add a new handler:

1. Implement the handler.
2. Register it in the appropriate `register*Routes()`.
3. Run `make openapi`.
4. Commit the regenerated `backend/spec/spec.yaml` alongside the
   handler.

The contract test (`tests/contract/contract_test.go`) catches
the case where the spec is out of date.

## Project structure

```
promptsheon/
├── main.go, daemon.go, cli.go, ...   # Single Go package; bin name picks mode
├── backend/                          # Server-side implementation
│   ├── capability/, release/, harness/, eval/, vault/, llm/, ...
│   ├── handlers_*.go                 # HTTP handlers (one domain per file)
│   ├── routes.go                     # Mux registration (single entry point)
│   ├── cas/                          # Content-addressable store
│   ├── store/                        # SQLite init + sqliteimpl repositories
│   └── spec/spec.yaml                # Generated OpenAPI 3.0 spec
├── sdk/                              # Go SDK (plus python/, typescript/)
├── deploy/                           # Helm chart, Grafana dashboard, Prometheus alerts
├── docs/                             # Markdown documentation + ADRs
├── tests/                            # contract/, e2e/, smoke/, chaos/, load/
├── scripts/                          # genopenapi, check-coverage, check-domain-purity, ...
├── Makefile
└── go.mod / go.sum
```

## Adding a new handler

1. Add the handler function in `backend/handlers_*.go`
   (match the existing file's domain — capability, release,
   harness, webhooks, etc.).
2. Register the route in the corresponding `register*Routes()`
   method on `backend/server.go`.
3. Add validation rules if the body has required fields
   (see `backend/validate.go`).
4. Wire error responses to `HTTPError` so the daemon's
   consistent shape (`{error, details?}`) is preserved.
5. Run `make openapi` to regenerate the spec.
6. Add a contract test entry if the SDK exposes the new
   endpoint.

## Adding a new SDK method

1. Add the method to `sdk/client.go`.
2. Add the method name to `sdkMandatoryMethods` in
   `tests/contract/contract_test.go`.
3. If the method hits an endpoint that wasn't previously
   covered, either add it to the SDK or add it to
   `knownGapRoutes` with a justification comment.

## Adding a new metric

1. Add the field to `backend/metrics/collector.go` (counter,
   histogram, or gauge).
2. Initialise it in `NewCollector()`.
3. Emit it in `prometheusFormat()`.
4. Wire any counter/histogram increments in the producer code.
5. If a new SLO alert depends on the metric, add it to
   `deploy/prometheus/promptsheon-alerts.yaml`.
6. If the metric is user-facing, add it to
   `docs/observability.md`.

## Adding a new migration

1. Drop a `020_your_migration.up.sql` (and a `.down.sql`) in
   `backend/store/migrations/`.
2. Update the migration count in
   `backend/store/migrate_test.go` (`TestNewSQLiteRunsAllMigrations`).
3. Update the migration table in
   `docs/architecture.md`.
4. The next `./promptsheond` boot applies it.

## Troubleshooting

- **`go vet ./...` fails on a fresh checkout** — make sure
  you're on Go 1.26+ (`go version`).
- **The OpenAPI spec is out of date after a route change** —
  run `make openapi` and commit the regenerated file.
- **The contract test fails with a missing method** — add the
  method to `sdk/client.go` (if the SDK should expose it) or
  add it to `knownGapRoutes` with a comment explaining why
  it's intentionally absent.
- **The smoke layer fails to bind to `:18080`** — another
  process is using the port. Stop it or set
  `PROMPTSHEON_SMOKE_PORT=28080` before running.
