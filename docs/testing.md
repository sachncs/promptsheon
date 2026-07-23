# Testing

Promptsheon has four test layers. The Makefile target `test`
runs the first three; the smoke layer runs against a freshly
booted daemon.

| Layer | What it covers | How to run |
|-------|----------------|------------|
| **Unit** | Every package's internal types and helpers. | `go test ./internal/...` |
| **Contract** | The OpenAPI spec parses, every registered route is reachable, the Go SDK surface is in sync. | `go test ./tests/contract/...` |
| **End-to-end** | A real daemon boots, the auth and bootstrap paths round-trip. | `go test ./tests/e2e/...` |
| **Smoke** | Boots a real daemon, runs every `examples/bash/*.sh` against it. | `bash tests/smoke/run.sh` |

## Layout

```
internal/<pkg>/*_test.go       # Unit tests for each package.
tests/contract/contract_test.go  # OpenAPI ↔ SDK contract.
tests/e2e/                     # In-process daemon, real HTTP.
tests/smoke/run.sh             # Bash smoke against a fresh daemon.
sdk/...                        # SDK has its own tests under each lang.
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

## Helpers

The test helpers below are the canonical entry points. Reach
for these before writing your own setup.

| Helper | Where | Purpose |
|--------|-------|---------|
| `testutil.DiscardLogger` | `internal/testutil/testutil.go` | Logger that writes to `io.Discard`. |
| `testutil.TempSQLite` | same | On-disk SQLite in `t.TempDir()` with migrations applied. |
| `testutil.ContextWithTimeout` | same | Context cancelled at `d` + auto-cleanup. |
| `testutil.MemoryBus` | same | In-memory `eventbus.Memory` with cleanup. |
| `testutil.Setenv` | same | Env-var mutation scoped to a single test. |
| `testutil.harnessrepo.New` | `internal/testutil/harnessrepo/` | `harness.Repository` fixture (datasets, preconditions, eval runs). |

## SDK contract test

The contract test in `tests/contract/contract_test.go` is the
guard between the OpenAPI spec and the Go SDK surface.

```
go test ./tests/contract/...
```

- `TestSpecIsValid` — `api/openapi.yaml` parses, has at least one
  path. Catches malformed YAML, missing schemas, and accidental
  schema regressions in the generator.
- `TestEveryRouteReachable` — every GET route is wired on the
  mux (not the mux fallback). Catches routes registered on a
  server that isn't built into the test mux.
- `TestSDKExposesMandatoryMethods` — the Go SDK exposes the
  documented method surface (Health, ListProviders, CreateWorkspace,
  CreateCapability, AddVersion, CreateRelease, GetRelease,
  ListReleases, Vote, Activate, Rollback, Invoke, Approval,
  ApproveAndInvoke, CreateDataset, ListDatasets, GetDataset,
  PutCases, DeleteDataset, CreatePrecondition, ListPreconditions,
  UpdatePrecondition, DeletePrecondition, RunEval, ListEvals,
  GetEval, CreateAPIKey, ListAPIKeys, RevokeAPIKey,
  OAuthLoginURL). Catches accidental deletions during refactors.
- `TestSDKEndpointsCovered` — every OpenAPI path is either wired
  in the SDK or on the `knownGapRoutes` list (with a justification
  comment). New gaps fail the build.

When you add a new SDK method, add it to `sdkMandatoryMethods`
in the contract test. When you add a new endpoint, either wire
it in the SDK or add it to `knownGapRoutes` with a comment
explaining why.

## Smoke test

The smoke test boots a fresh daemon, runs every
`examples/bash/*.sh` against it, and tears it down. It's
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

The smoke layer is wired into CI as a separate `smoke:` job so
the default `test` job stays fast.

## End-to-end test

The end-to-end layer (`tests/e2e/`) boots an in-process
`promptsheond` and exercises the auth + bootstrap paths
through the real HTTP handlers. It's a smaller surface than the
smoke layer — the smoke layer runs the actual bash examples,
while the e2e layer tests scenarios that the bash examples
don't reach (e.g. expired API keys, malformed JSON bodies,
CSRF, CORS preflight).

```
go test ./tests/e2e/...
```

## Adding a new test

- For unit-level coverage of a single function, drop a
  `*_test.go` in the same package and use the standard
  `testing` package.
- For cross-package coverage (a real daemon + real HTTP),
  drop a `*_test.go` in `tests/e2e/` and use the helpers in
  `internal/testutil`.
- For OpenAPI / SDK drift detection, extend
  `tests/contract/contract_test.go`.

If your test needs a fixture that doesn't exist, add it to
`internal/testutil/`. The harness fixture in
`internal/testutil/harnessrepo/` is the canonical pattern:
implements the consumer-defined interface, safe for
concurrent use, exported (so cross-package tests can link
it).