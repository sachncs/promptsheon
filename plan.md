# Product Readiness Remediation Plan

This plan converts the repository audit into actionable work. Every item defines the required change, verification method, success criteria, and relevant dependencies.

## Phase 0 — Establish the release baseline

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P0.1 | Decide whether the release is `v0.3.x` or `v1.0.0`. Align `VERSION`, README, roadmap, changelog, Helm chart, Docker tags, and release metadata. | Run repository-wide version/reference searches. Build binaries and run `--version`. | One authoritative version appears everywhere; no document claims contradictory release status. |
| P0.2 | Create `docs/operations/release-readiness.md` containing the release checklist, owners, commands, expected results, and escalation paths. | Review the checklist against CI, Docker, Helm, security, API, and documentation gates. | Every release gate has an owner, command, expected result, and escalation path. |
| P0.3 | Add a CI job that runs the complete release checklist. | Run the workflow on a branch and on a release tag. | A release cannot be published unless every required gate passes. |

## Phase 1 — Repair release and build infrastructure

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P1.1 | Migrate `.goreleaser.yml` to valid GoReleaser v2 syntax, including build hooks. | Run `goreleaser check`. | `goreleaser check` exits 0. |
| P1.2 | Replace stale `backend.*` linker targets with `buildinfo.*`. | Build with explicit `-ldflags`; run every binary with `--version`. | Version, commit, and build timestamp are present and correct in all binaries. |
| P1.3 | Update Docker’s Go builder image from 1.23 to the supported Go version. | Run `docker build --no-cache`. | Docker build succeeds from a clean checkout. |
| P1.4 | Add a container smoke test. Start the image, call `/health`, `/ready`, and one authenticated API route. | Run the smoke test in CI. | The published container starts as non-root and responds successfully. |
| P1.5 | Validate the GoReleaser release artifact. | Run `goreleaser release --snapshot --clean` or equivalent dry run. | All archives, checksums, SBOMs, signatures, and container artifacts are generated. |
| P1.6 | Ensure release artifacts are built with the Go version declared in `go.mod`. | Inspect release logs and artifact metadata. | No release path uses an unsupported Go toolchain. |

## Phase 2 — Repair CI gates

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P2.1 | Rewrite `scripts/check-coverage.sh` for the current `promptsheon/` layout. | Run it against a real coverage profile and synthetic self-tests. | Correct package floors are detected; obsolete `backend/` paths are absent. |
| P2.2 | Update fuzz workflow paths from `backend/*` to current package paths. | Run the fuzz workflow manually and on matching pull requests. | Fuzz jobs execute real fuzz targets and fail on fuzz regressions. |
| P2.3 | Replace or migrate `.golangci.yml`. | Run `golangci-lint check` and `golangci-lint run`. | The configuration is valid and the command passes. |
| P2.4 | Make `make check` reproducible on a clean machine. | Run in a clean container with only documented prerequisites. | No undeclared dependency, such as `goimports`, causes failure. |
| P2.5 | Pin or document versions for Go, Node, GoReleaser, Helm, gosec, govulncheck, and staticcheck. | Re-run CI using pinned versions. | Local and CI results are deterministic. |
| P2.6 | Make benchmark regression checking fail on missing or zero benchmarks. | Run with valid, missing, and renamed benchmark cases. | Missing benchmark coverage fails with an actionable message. |
| P2.7 | Remove `|| true` from load-test gates or explicitly mark load tests informational. | Run the load-test job with a failing scenario. | CI status accurately reflects whether load testing is required. |

## Phase 3 — Security hardening

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P3.1 | Resolve all current `gosec` findings. | Run `gosec ./...`. | Zero unreviewed findings; every suppression has a documented justification. |
| P3.2 | Restrict healthcheck and CLI HTTP destinations. | Add tests for invalid hosts, schemes, ports, and redirects. | Only explicitly permitted daemon destinations are reachable. |
| P3.3 | Harden user-controlled filesystem paths. | Add traversal tests using `../`, absolute paths, symlinks, and unexpected roots. | Paths cannot escape their intended directory. |
| P3.4 | Replace `sh -c` preconditions with an explicitly controlled execution model. | Test shell metacharacters, timeout, cancellation, environment, and process-group cleanup. | Arbitrary command execution is disabled or explicitly isolated and documented. |
| P3.5 | Replace or justify `math/rand` canary selection. | Add deterministic injectable selection for tests and production behavior tests. | Routing behavior is statistically correct, reproducible in tests, and threat-model approved. |
| P3.6 | Review all ignored errors in production code. | Add static analysis or a lint rule for ignored errors. | No persistence, serialization, randomness, shutdown, or security error is silently discarded. |
| P3.7 | Add request body limits and outbound HTTP timeouts. | Test oversized bodies, slow servers, cancelled contexts, and stalled providers. | Requests terminate within bounded resource limits. |
| P3.8 | Review secrets, cookies, logs, and error responses. | Run security tests and inspect HTTP responses and log output. | No credentials, provider secrets, tokens, or sensitive upstream errors leak. |
| P3.9 | Run `govulncheck` in a network-enabled CI environment. | Execute the actual CI security job. | Dependency vulnerability scanning completes successfully and is archived per release. |

## Phase 4 — Correct persistence and reliability behavior

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P4.1 | Handle JSON marshal/unmarshal failures in SQLite persistence. | Add malformed and unsupported schema tests. | Corrupt data returns a contextual error instead of silently becoming empty data. |
| P4.2 | Add checked conversions for audit row IDs. | Test maximum and invalid integer boundaries. | Audit ordering cannot silently wrap or become negative. |
| P4.3 | Propagate persistence failures from harness, recommendation, scheduler, and audit workers. | Inject repository failures into tests. | Callers receive errors or durable failure metrics; no false success is reported. |
| P4.4 | Make shutdown ownership explicit for every goroutine and resource. | Run race tests, repeated start/stop tests, and cancellation tests. | No goroutine, database, HTTP client, tracer, or worker leaks remain. |
| P4.5 | Add failure-injection tests for SQLite disk-full, locked DB, interrupted transaction, and corrupted migration state. | Run chaos tests in CI. | Failure behavior is documented, bounded, and recoverable where supported. |
| P4.6 | Verify backup and restore, not only backup creation. | Create a backup, destroy the test database, restore it, and verify the audit chain and records. | Restore produces a usable and integrity-verified installation. |

## Phase 5 — API and SDK contract completion

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P5.1 | Treat OpenAPI as the single API source of truth. | Generate the spec and compare it in CI. | No route or schema drift is possible without CI failure. |
| P5.2 | Confirm every documented operation has an SDK method or an explicit exclusion. | Extend `tests/contract/contract_test.go`. | Missing SDK methods fail the test; intentional exclusions are documented. |
| P5.3 | Update all SDK examples to `pkg/promptsheon`. | Build every Go example or run example tests. | No documentation references the removed `sdk` package. |
| P5.4 | Decide whether Python and TypeScript SDKs are supported. | If supported, generate and publish real clients. If unsupported, remove all claims and references. | The supported SDK list is accurate in README, docs, roadmap, and release metadata. |
| P5.5 | Add compatibility tests for authentication, error envelopes, pagination, idempotency, and context cancellation. | Run contract and integration tests. | Public API behavior is stable and documented. |

## Phase 6 — Documentation cleanup

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P6.1 | Fix broken links such as `docs/security.md`, `docs/configuration.md`, and `docs/operations.md`. | Run a repository-wide Markdown link checker. | No broken internal links remain. |
| P6.2 | Remove obsolete `backend/`, `sdk/`, and old package-layout references. | Search with `rg`. | No obsolete implementation path remains outside historical migration documents. |
| P6.3 | Separate current functionality from roadmap functionality. | Review README, architecture docs, roadmap, and Helm docs together. | Every feature is classified as shipped, experimental, planned, or removed. |
| P6.4 | Synchronize deployment documentation with Dockerfile and Helm behavior. | Follow the deployment guide from a clean checkout. | A new operator can build, deploy, configure, upgrade, back up, and restore successfully. |
| P6.5 | Add explicit limitations documentation. | Review against actual implementation. | SQLite-only storage, single-region behavior, scaling limits, and missing features are clearly stated. |

## Phase 7 — Google Go Style compliance

The Google Go Style Guide prioritizes clarity, simplicity, concision, maintainability, and consistency, and requires `gofmt` formatting and MixedCaps naming: <https://google.github.io/styleguide/go/guide>.

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P7.1 | Keep `gofmt` enforced in CI. | Run `gofmt -l .`. | Zero files are reported. |
| P7.2 | Remove duplicate and historical comments from production code. | Review changed files and run documentation lint. | Comments explain rationale and invariants, not obsolete ticket history. |
| P7.3 | Split oversized daemon/server responsibilities. | Review the package dependency graph and run tests after each extraction. | Lifecycle, configuration, HTTP, telemetry, storage, and workers have clear responsibilities. |
| P7.4 | Review generic names such as `Manager`, `Adapter`, `Producer`, and `Helper`. | Perform an API naming review. | Public names communicate domain behavior clearly. |
| P7.5 | Enforce error-handling conventions. | Enable `errcheck` or equivalent with justified exclusions only. | No unexplained ignored production errors remain. |
| P7.6 | Keep interfaces consumer-defined and minimal. | Review all exported interfaces. | Interfaces have one cohesive responsibility and no speculative methods. |
| P7.7 | Add GoDoc for every exported identifier. | Run a documentation linter or manual exported-symbol audit. | Exported APIs explain purpose, inputs, outputs, errors, and invariants. |
| P7.8 | Avoid unnecessary abstraction and duplication. | Review duplicate validation, serialization, route, and error logic. | Each business rule has one authoritative implementation. |

## Phase 8 — Coverage and test expansion

| ID | Change | Verification | Success criteria |
|---|---|---|---|
| P8.1 | Add unit tests for currently untested production packages. | Run per-package coverage reports. | Every production package has meaningful tests. |
| P8.2 | Add negative-path tests for every handler family. | Run HTTP handler tests with invalid auth, malformed input, missing resources, and dependency errors. | Error envelopes and status codes are stable. |
| P8.3 | Add race tests for workers, settings, audit, idempotency, election, and event bus. | Run `go test -race ./...` repeatedly. | No data races or flaky shutdown behavior. |
| P8.4 | Add fuzzing for parsers, validation, URLs, manifests, schedules, and audit serialization. | Run the fuzz workflow manually and nightly. | Fuzz targets execute against current package paths and retain useful corpus coverage. |
| P8.5 | Add end-to-end upgrade tests. | Start at the previous release, migrate to the current release, and verify data and APIs. | Supported upgrades are proven rather than documented only. |

## Final definition of done

The repository is product-ready only when all of the following are true:

1. `go test -race -count=1 ./...` passes.
2. `go vet -all ./...` passes.
3. `gofmt -l .` returns no files.
4. Staticcheck, errcheck, and security scans pass.
5. `gosec` has zero unexplained findings.
6. `govulncheck` completes successfully.
7. `goreleaser check` passes.
8. Docker build and container smoke tests pass.
9. Helm lint, schema validation, and rendered-manifest validation pass.
10. OpenAPI generation is clean.
11. SDK/API parity tests pass.
12. Coverage gates operate on the current repository layout.
13. Backup and restore tests pass.
14. All documentation links resolve.
15. Release version claims are consistent.
16. No documented feature is presented as shipped unless it is implemented and tested.
17. Every ignored production error is handled or explicitly justified.
18. Security, operations, upgrade, rollback, and incident procedures are documented and executable.
