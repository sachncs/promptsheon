# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Source layout**: the project is now root + `backend/`. All
  production code lives either at the repo root (`main.go`,
  `daemon.go`, `cli.go`, `cli_*.go`, `daemon_evolver.go`,
  `daemon_release_invoker.go`, `embed_frontend.go`, `healthcheck.go`,
  `*_test.go`) or under `backend/<pkg>/`. There is no `cmd/`,
  `internal/`, or `pkg/` directory. `main.go` dispatches by
  `os.Args[0]` (`promptsheond` → daemon, `promptsheon` → CLI,
  `promptsheon-healthcheck` → probe) so the three binaries
  build from the same source via `make build`.
- **OpenAPI source of truth**: the spec lives at
  `backend/spec/spec.yaml`. `api/openapi.yaml` and the 24
  per-resource splits under `backend/spec/` are deleted.
  `scripts/genopenapi` parses `backend/routes.go` for routes
  and `backend/handlers_*.go` for request schemas. The contract
  test, the Python SDK codegen, and the TypeScript SDK codegen
  all read from `backend/spec/spec.yaml`.
- **Build + release**: `Makefile` builds all three binaries
  into `./bin/`; `bin/promptsheond`, `bin/promptsheon`, and
  `bin/promptsheon-healthcheck` are produced by three
  `go build -o bin/<name> .` invocations. `.goreleaser.yml`
  publishes all three on tagged releases. A multi-stage
  `Dockerfile` is added for `docker build` / `docker compose`.
- **`embed_frontend.go`**: `frontendDist` is wrapped in
  `fs.Sub(frontendDist, "frontend/dist")` before being handed
  to `HandleFrontend`, so the dashboard's index.html and assets
  are reachable. The previous form passed the embed root and
  the mux could not find `index.html`.
- **Self-evolve disable**: `PUT
  /api/v1/capabilities/{id}/self-evolve` now merges the request
  into the persisted config instead of zeroing every column.
  The previous form decoded the request body into a
  zero-valued `SelfEvolveConfig` and overwrote `dataset_id`,
  `min_score`, `max_revisions`, `cooldown_sec`, and
  `target_env` whenever the client sent `{"enabled": false}`.
  Regression test: `handlers_capabilities_merge_test.go`.
- **Release invoke hashes**: `daemon_release_invoker.go` now
  uses `backend.{ComputeManifestHash,InputHash,ModelRevision}`
  (bare hex, no `sha256:` prefix) instead of local copies that
  produced hashes incompatible with `handlers_releases.go`.
  Eval cases and live `/releases/{id}/invoke` invocations now
  record the same `manifest_hash` and `input_hash`.
- **CAS loader dedup**: `daemon_evolver_cas.go` is deleted.
  `evolve.NewCasPromptLoader` (`promptsheon/evolve/loader.go`)
  is used directly. The `evolverLoaderAdapter` type is removed.
- **ClickHouse**: `daemon_clickhouse.go`,
  `daemon_noclickhouse.go`, and `backend/rollups/clickhouse/`
  are deleted. The build tag `-tags clickhouse` is no longer
  wired; `PROMPTSHEON_CLICKHOUSE_DSN` is a no-op reserved env
  var. Rollups are kept in-memory only.
- **Coverage gate**: `scripts/check-coverage.sh` profiles
  `backend/` (was reading nonexistent `backend/api/`). Floors:
  60% for `api handlers`, 50% for domain packages, 40% for
  `backend` root and `backend/store`.
- **Domain-purity gate**: `scripts/check-no-package-state.go`
  scans `backend/<pkg>` (was scanning nonexistent `internal/`).
  `scripts/docs-check.py` recognises `backend/...go` and
  `backend/spec/...yaml` refs (was matching the old
  `internal/`/`pkg/`/`cmd/` layout).
- **Bench regression gate**: `scripts/bench-baseline.txt` is
  committed with the 8 curated benchmarks at
  `BENCHTIME=1s`. `scripts/check-bench-regression.sh` now
  enforces a 20% delta against the baseline instead of
  silently passing.
- **`make check`** — new umbrella gate: `fmt + vet + lint + test
  + openapi-check + docs-check`. **`make purity`** — new
  domain-purity gate: `lint-domain + lint-deps`. Both are the
  hooks the ROADMAP referenced; the targets now exist.
- **Contract test**: `newTestServer` boots the real daemon via
  subprocess instead of a fake mux that returned structured
  JSON 404s (which made the wiring check useless). The test
  is skipped when `bin/promptsheond` is absent (so
  contributors running only `go test ./...` are not blocked).
  `TestEveryRouteReachable` now probes every documented
  `(method, path)` pair, not a uniform GET — so a missing PUT
  registration trips the test. `TestSDKExposesMandatoryMethods`
  uses reflection over `*sdk.Client`'s method set instead of a
  hardcoded list.
- **E2E lifecycle**: `tests/e2e/daemon_e2e_test.go::TestCapabilityLifecycle`
  walks the full workspace → project → capability → version →
  release → vote (via a second-principal reviewer) → activate →
  invoke flow. The previous form stopped at `Health +
  ListProviders`.
- **SDK test wiring**: `.github/workflows/ci.yaml` now runs
  `pytest sdk/python/tests` and `npm --prefix sdk/typescript
  test` (Jest) on every PR. `sdk/typescript/package.json` has
  Jest wired and `tsconfig.json` includes `tests/`; the
  `sdk/typescript/scripts/codegen.sh` previously did
  `tsc --noEmit` only.
- **Fuzz paths**: `.github/workflows/fuzz.yaml` watches
  `backend/{vault,tests/unit/redactor,tests/unit/injection,cas,schedule}/**`
  (was watching nonexistent `internal/vault`,
  `internal/redactor`, `internal/injection`).
- **TLC gate**: a soft-gate CI job runs `tlc` if installed;
  the existing Go test that asserts the spec's presence and
  required invariants remains the always-on check.
- **Coverage / openapi-check / docs-check**: each is now wired
  into its own CI job or step and fails the build on drift.

### Removed

- **Layout cruft**:
  - `api/openapi.yaml` and the entire `api/` directory.
  - 24 unread per-resource OpenAPI splits under
    `backend/spec/` (`alerts.yaml`, `apikeys.yaml`, ...);
    `backend/spec/spec.yaml` is the single remaining file.
  - `scripts/generate-openapi-paths.sh` (no caller, referenced
    nonexistent paths).
  - `scripts/genproto.sh` (no caller, no `.proto` files exist).
- **Duplicate code**:
  - `daemon_evolver_cas.go` — replaced by
    `promptsheon/evolve/loader.go`.
  - `evolverLoaderAdapter` type in `daemon_evolver_adapter.go`.
  - Dead `db *store.SQLite` field on `apiReleaseInvoker` in
    `daemon_release_invoker.go`.
  - Local `inputHash`, `manifestHash`, `modelRevision`
    helpers in `daemon_release_invoker.go` (replaced by
    `backend.{InputHash,ComputeManifestHash,ModelRevision}`).
- **Unwired build tags / dead sinks**:
  - `daemon_clickhouse.go`, `daemon_noclickhouse.go`,
    `backend/rollups/clickhouse/`.
  - The discarded-writer block in `daemon.go` that initialised
    a ClickHouse sink and never persisted anywhere.

### Fixed

- **Dashboard unreachable**: `fs.Sub` is applied to the
  embedded `frontend/dist` so the SPA's `index.html` resolves
  through `HandleFrontend`. The previous wiring passed the
  embed root and `/` returned the mux's text 404. Test:
  `embed_frontend_test.go`.
- **Self-evolve `disable` clobbered config**: see the Changed
  note above. Regression test:
  `handlers_capabilities_merge_test.go`.
- **Eval vs live invocation hash mismatch**: see the Changed
  note. The eval harness now records the same manifest and
  input hashes as the live `/releases/{id}/invoke` route.
- **Docs-check false negatives**: `isMuxFallback` previously
  matched every 404 from the fake contract-test mux, so the
  route-wiring check passed even when a route was missing.
  With the new real-daemon test server, missing routes fail
  the test.
- **Bench regression gate disabled**: was silently passing
  because `scripts/bench-baseline.txt` was absent. Baseline
  is committed; gate now catches > 20% regressions.
- **Many stale doc references**: `docs/architecture.md`,
  `docs/modules.md`, `docs/development.md`, `docs/cli.md`,
  `docs/guardrails.md`, `docs/security.md`, `docs/harness.md`,
  `docs/configuration.md`, `docs/release.md`,
  `docs/algorithms.md`, `docs/troubleshooting.md`,
  `docs/deployment.md`, `docs/getting-started.md`,
  `docs/faq.md`, `docs/llm-providers.md`, `docs/testing.md`,
  `docs/design-decisions.md`, `docs/operations.md`,
  `docs/glossary.md`, `docs/multi-region.md`,
  `docs/upgrade.md` were rewritten to match the root +
  `backend/` layout. `docs/audit.md` is renamed to
  `docs/audit-2026-07-26.md` and banner-tagged as a frozen
  historical snapshot.
- **`.gitignore` cruft**: removed `arc-agi/`, `environment_files/`,
  duplicated `*.test` and `.DS_Store` entries, the dead
  `/promptsheond` and `/promptsheon` root entries (binaries
  now live in `bin/`), and the `genopenapi` root entry.

## [0.3.0] - 2026-07-25

The v0.3.0 production release. Six primitives originally tagged
v0.4.0+ are now wired into the production daemon, not just
exercised by tests. OpenAPI spec regenerated; SDKs expose new
methods; the smoke test passes end-to-end.

### Added

- **Production wiring**
  - **LLM-judge scorer registered at boot.** `internal/llm/judge.go`
    exposes `NewJudgeClient(registry)`; `cmd/promptsheond/main.go`
    calls it after wiring the harness runner and registers the
    scorer via `eval.RegisterLLMJudge`. Operators can opt out
    with `PROMPTSHEON_LLM_JUDGE=off`.
  - **`POST /api/v1/reasoning/compile`** — accepts an `Intent` and
    returns a `Plan`. Errors map to `404 ErrNoMatch` and
    `409 ErrConstraintViolation`. The catalog is built from the
    workspace's capabilities filtered by reputation.
  - **ContinuousEval loops.** Operators configure with
    `PROMPTSHEON_CONTINUOUS_EVAL=cap-1:ds-1:60;cap-2:ds-2:300`.
    Each entry starts a `ContinuousEval` goroutine that ticks
    every N seconds and runs the active release against the
    dataset.
  - **Inheritance wired into Version create.**
    `POST /api/v1/capabilities/{id}/versions` accepts an optional
    `parents: [version_id, ...]` field. The handler calls
    `ResolveManifest` against the Repository; cycles and depth
    overflow surface as `422 Unprocessable Entity`.
  - **`PROMPTSHEON_DATABASE_URL=postgres://...` detected.** The
    daemon logs a clear warning that pgx wiring ships in v0.4.0
    and falls back to SQLite.
- **Primitives landed**
  - `LLM-JUDGE-1` / LLM-judge scorers — `internal/eval/scorer_llm_judge.go`.
  - `REASON-COMP-1` / Reasoning compiler — `internal/reasoning/compiler.go`.
  - `PERF-RL-1` / Partitioned rate limiter (16-way FNV-1a sharded).
  - `CONT-1` / ContinuousEval at scale — `internal/harness/continuous.go`.
  - `INHERIT-1` / Capability Inheritance — `internal/capability/inheritance.go`.
  - `PG-1` / Postgres backend with RLS — `internal/store/postgres/`.

## [0.3.0-rc.1] - 2026-07-25

Tagged `v0.3.0-rc.1`. 31 atomic commits; 46 atomic items
across the Day 1/2/3 punch list. Every change verified by
tests, atomic commits, and the CHANGELOG + ROADMAP + glossary
updates.

### Added

- **`QW#1 / FIX-AUDIT-ONCONFLICT`** — Audit archival regression
  test pinning `INSERT OR IGNORE` idempotency. The retention
  sweeper's second sweep is a no-op against a partial failure.
- **`BANDIT-RNG-1 / FIX-BANDIT-RNG`** — Selector seeded-RNG
  determinism test: two `Selector`s with the same seed produce
  identical arm sequences.
- **`QW#3 / WIRE-RESOLVER-PROD`** — Regression test pinning
  `WithReleaseResolver(resolver)` in `cmd/promptsheond/main.go`.
- **`QW#4 / VAULT-STOP`** — `Vault.Stop()` wired into graceful
  shutdown; `buildServer` returns the `*vault.Vault` so
  `startHTTPServerAndWait` can stop it before logging
  "server exited".
- **`OBS-LOG-3 / HUB-STOP-ORDERING`** — Regression test pinning
  the deferred order: `defer db.Close()` is registered before
  `defer logHub.Stop()`.
- **`OPS-SHUTDOWN-1 / STOP-AUDIT-WORKERS-ON-SHUTDOWN`** —
  Regression test pinning `srv.StopAuditWorkers(auditDrainCtx)`
  on every shutdown path.
- **`MAN-1 / MANIFEST-DROP-UNUSED-REQUIRED`** —
  `Manifest.Validate` requires only Prompt, ModelPolicy,
  RuntimePolicy. ContextContract + Memory are optional kinds.
- **`MAN-2 / MANIFEST-DROP-KNOWLEDGE`** — Removed dead
  `ArtifactKnowledge` constant. The `Knowledge` slice stays
  for wire-compat.
- **`PURITY-1 / PURITY-CHECK-FIX`** —
  `TestDomainPurityScriptExists` runs the bash purity check
  from Go and fails on regression.
- **`PERF-AUDIT-2 / AUDIT-VERIFY-CACHE-INVALIDATION`** —
  `AppendAudit` invalidates `auditVerifyCache` so a
  write-then-verify pair walks the full chain.
- **`OBS-RET-2 / AUDIT-ARCHIVE-RETENTION-TTL`** — Regression test
  pinning the cutoff: rows newer than the TTL stay in
  `audit_entries`, not in `audit_archive`.
- **`TLA-LIFECYCLE-1 / TLA-RELEASE-LIFECYCLE-SPEC`** — TLA+ spec
  (`tla/release_lifecycle.tla` + `.cfg`) modelling the Release
  state machine with Maker/Checker separation-of-duties and the
  "exactly one active per Environment" invariant.
- **`CONTRACT-1 / CAPABILITY-CONTRACT-TYPE`** — New
  `CapabilityContract` value type: `InputSchema`, `OutputSchema`,
  `SuccessRubric`, `SLOTarget`, `BlastRadius` ∈ {low, medium, high},
  `AutoPromotable`. `CanAutoAdopt` enforces the blast-radius policy.
- **`CONTRACT-2 / CAPABILITY-CONTRACT-API`** —
  `PUT/GET /api/v1/capabilities/{id}/contract` CRUD. Migration
  018 adds the `capability_contracts` table.
- **`DIFF-1 / CAPABILITY-DIFF`** —
  `GET /api/v1/capabilities/{id}/diff?from=N&to=M` returns the
  structural diff between two Versions.
- **`CATALOG-1 / CAPABILITY-CATALOG`** —
  `GET /api/v1/catalog/capabilities?workspace_id=...&q=...`
  paginated search across the workspace.
- **`REPUTATION-1 / CAPABILITY-REPUTATION`** —
  `GET /api/v1/capabilities/{id}/reputation` returns the derived
  trust score (eval pass rate × SLO adherence × decision
  adoption rate).
- **`OBS-TICK-1 / OBSERVATION-TICK-WIRED`** —
  `Aggregator.Tick(ctx, interval, fn)` primitive; production
  wiring emits Recommendations on each tick.
- **`LOOP-1`** — Regression test pinning the production
  recommendation loop.
- **`SETTINGS-CRDT-1 / SETTINGS-COPY-ON-REMOTE-WRITE`** —
  `TestMergeSystemConfigPersistsWinner` pins that a remote write
  with a strictly-dominant vector replaces the local row.
- **`SDK-1 / SDK-CODEGEN-WIRED`** — `make sdk` refreshes Python
  and TypeScript SDK artifacts from `api/openapi.yaml`.
- **`SDK-2 / PYTHON-SDK-COMPLETE`** — Python client covers every
  `/api/v1` route.
- **`SDK-3 / TYPESCRIPT-SDK-COMPLETE`** — Same for TypeScript.
- **`SDK-VERSION-1`** — `scripts/sync-version.sh` keeps
  `sdk/python/pyproject.toml` and `sdk/typescript/package.json`
  in sync with `VERSION`.
- **`RELEASE-TOKEN-1`** — Per-job permissions in `ci.yaml`:
  `contents: write`, `packages: write`, `id-token: write`,
  `attestations: write`.
- **`CI-FUZZ-1`** — `.github/workflows/fuzz.yaml` runs 20s per
  surface on PRs (vault, redactor, injection). Nightly fuzz via
  `workflow_dispatch`.
- **`README-1`** — Headline: "AI Capability Control Plane".
  Documents Capability Contract, Diff, Catalog, Reputation,
  Recommendation Loop, CRDT-backed Settings, Audit Chain.
- **`DOC-FRESH-2`** — `make docs-check` passes across 60
  link-files and 35 ref-files.

## [0.2.0] - 2026-07-24

The v0.2.0 release. Honest scope: the runtime behaviour (#1–#25),
the release pipeline, the doc-freshness gate, the mdBook site,
the curated benchmark set, and the k6 p99 gate are production
defaults. The CRDT-backed idempotency cache, the replay-set
CRDT, and multi-region replication are research deliverables
(design docs in `docs/research/`); they are not in this binary
and not covered by the v0.2.0 SLOs.

### Added

- **`DOC-CI-3 / DOC-FRESH-1`** — `make docs-check` is a
  deterministic doc-freshness gate: it walks every markdown file
  under `docs/` plus `README.md` and `CHANGELOG.md` and reports
  any local markdown link that resolves to a missing file, plus
  any path-shaped reference to source code that no longer points
  at a real file. Stdlib Python; no new Go dependency. CI runs it
  on every PR. Operators can add a trailing `<!-- stale-ok:
  <reason> -->` to bless a historical line.
- **`DOC-CI-1 / OSS-GOV-1`** — The mdBook site lives under
  `docs-site/`. `docs-site/book.toml` points `src` at `../docs`.
  The Makefile target is `make docs-site`; the Pages workflow
  builds with `mdbook build docs-site` and deploys
  `docs-site/book/`.
- **`PERF-BENCH-1`** — `make bench` runs the curated 8 Go
  benchmarks listed in `scripts/benchmarks.txt`. A
  `bench-nightly.yaml` workflow runs the same set nightly and on
  `workflow_dispatch` and uploads results as artefacts. The p99
  latency gate lives in the existing k6 scenario
  `tests/load/scenarios/10-sustained-load.js`.
- **`OSS-REL-1 / SEC-16c`** — The release pipeline uses
  `actions/attest-build-provenance@v1` to attach GitHub artifact
  attestations to every release binary, in addition to the
  existing cosign keyless signing. The release job installs cosign
  via `sigstore/cosign-installer@v4` (pinned to the `v2.x` major)
  before GoReleaser runs. The `subject-path` for the attestation
  step is fixed to the paths GoReleaser actually emits.
- **`RES-AUDIT-1`** — A TLA+ specification of the audit chain
  (`tla/audit_chain.tla`) and its TLC config
  (`tla/audit_chain.cfg`) lands alongside a concise README. The
  spec models the same ordering invariants the
  `VerifyAuditChainOnDB` SQLite walk checks at runtime.
- **`RES-CRDT-1, RES-CRDT-2, RES-BANDIT-1`** — Research notes
  ship as design docs only:
  - `docs/research/crdt-idempotency-cache.md` — design options
    for a CRDT-backed idempotency cache.
  - `docs/research/replay-set-crdt.md` — replay-set CRDT
    (immutable add-only G-Set, conflict semantics, retention).
  - `docs/research/thompson-sampling-bayesian.md` — acknowledges
    the existing Thompson Sampling selector is already Bayesian.

### Note on what is NOT in v0.2.0

The CRDT idempotency cache, the replay-set CRDT, and
multi-region replication ship as research only — they are
documented in `docs/research/`, not in this binary. The bandit
selector remains the existing Thompson Sampling implementation.
Item #28 (v0.2.0 readiness tests) is documented in
`docs/release.md` as the next milestone.

## [0.1.x] - Production-readiness hardening

The v0.1.x audit + remediation pass.

### Added

- **`DB-CONC-2 / OPS-4`** — Dedicated retention `*sql.DB` lives
  in `main()` scope (not `buildServer`), so its lifetime matches
  the daemon. Previously the `defer Close` inside `buildServer`
  ran before the retention goroutine's first tick, causing
  `database is closed` panics.
- **`SEC-10a`** — KMS `Provider` persists the wrapped data key
  (`CiphertextBlob`) on first use and reads it on cache miss via
  `KMSClient.Decrypt`. The plaintext cache is an LRU of size 16
  keyed by `sha256(wrapped_data_key)`. Migration
  `009_vault_state.up.sql` creates the singleton `vault_state`
  table.
- **`feat(helm):` values.schema.json** — rejects
  `replicaCount>1` (SQLite is single-writer), `dbBackend=postgres`
  (removed backend), and `auth=false` without `insecureLoopback`.
- **`feat(helm):` default auth=true, vault Secret, PDB, ConfigMap
  checksum, seccomp Profile** — production-safe defaults.
- **`feat(dockerfile):` writable /data, multi-arch, ldflags**.
- **`feat(ci):` tag-trigger release** with Helm render assertions
  for the values schema.
- **`feat(config):` refuse non-loopback bind when
  `PROMPTSHEON_AUTH=false`** — closes the bootstrap admin-key-mint
  attack.
- **`feat(config):` refuse non-loopback bind without TLS**.
- **`feat(daemon):` in-process TLS termination** via
  `PROMPTSHEON_TLS_CERT_FILE` / `PROMPTSHEON_TLS_KEY_FILE`.
- **`feat(auth):` X-Bootstrap-Token gate** + OAuth auto-provision
  off by default.
- **`feat(daemon):` wire alert delivery to webhook dispatcher** —
  alerts now actually fire instead of being silently dropped.
- **`feat(store):` migration 027** — `datasets`, `dataset_cases`,
  `preconditions`, `eval_runs`, `eval_results` tables.
- **`feat(domain):` harness types** — `Dataset`, `DatasetCase`,
  `Precondition`, `EvalRun`, `EvalResult` plus the `Scorer` enum.
- **`feat(store):` SQLite repository impls** for all five tables.
- **`feat(eval):` Scorer interface + built-ins** — exact_match,
  contains, regex, json_schema. Pluggable via `Register`.
- **`feat(harness):` PreconditionRunner** — executes named command
  hooks via `sh -c` with per-hook timeouts, captures combined
  stdout+stderr (truncated to 8 KiB), and exposes a typed
  `*PreconditionError` for the HTTP 409 mapping.
- **`feat(harness):` EvalRunner service** — loops a Dataset's
  cases through a `ReleaseInvoker`, scores each via the chosen
  Scorer, persists per-case results and the aggregate EvalRun.
- **`feat(release):` Activate runs preconditions** —
  `Service.WithHarness` attaches the runner + harness repo; a
  failing hook returns `harness.ErrPreconditionFailed` and the
  Release is left in `pending`.
- **`feat(api):` eight new HTTP routes** — `/datasets`,
  `/preconditions`, `/evals` under `/api/v1`.
- **`feat(daemon):` wire harness service into promptsheond startup**.
  `apiReleaseInvoker` adapts `invoke.Invoker` for the eval loop.
- **`feat(cli):` `dataset`, `precondition`, `eval` subcommands**.
- **`feat(sdk):` dataset / precondition / eval methods**.
- **`docs:` `docs/eval.md`** — Dataset / Precondition / Eval
  primitive, built-in scorers, iteration loop.
- **`docs:` `docs/harness.md`** — why the harness exists.
- **`README:` Harness engineering section** with the curl-style
  iteration loop.

### Changed

- **Migration directory consolidated**: 51 .up.sql + 17 .down.sql
  files replaced with 8 .up.sql files (no .down.sql). Phase 1.x
  fixes (url UNIQUE, secret column drop, updated_at / last_used_at,
  enabled CHECKs, typed lineage_edges parent/child, alerts
  acknowledgement, ON DELETE SET NULL) are folded into
  `001_core_schema.up.sql`. The destructive gate is tightened
  from substring match to anchored regex `^\d+_destructive`.
- **`fix(config):` `CORSOrigins` is now `[]string`**; wildcard
  `*` rejected on non-loopback bind.
- **`fix(ratelimit):` trust X-Forwarded-For only from
  `PROMPTSHEON_TRUSTED_PROXIES` CIDRs**.
- **`fix(api):` require `PermAuditRead` on `/logs/stream` and
  `/metrics`**; rate-limit `/setup`, `/login`, `/callback`.
- **`fix(auth):` validate `ps_` prefix**; never log raw key bytes.
- **`fix(api):` `*http.MaxBytesError` → 413 instead of 500**.
- **`fix(api):` Idempotency-Key middleware** for POST handlers.
- **`fix(webhooks):` per-endpoint `AllowPrivate`**; remove global
  `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE` SSRF toggle.
- **`fix(audit):` dedicated worker context**; drain barrier before
  cancel — never drop entries on SIGTERM.
- **`fix(store):` drop `auditMu`**; serialisable SQLite tx is the
  ordering primitive.
- **`fix(observability):` never delete audit rows** — chain
  integrity. Trace-only retention.
- **`fix(lifecycle):` `Stop()` for webhook dispatcher, ws Hub,
  authenticator** — no goroutine/FD leak on rolling restart.
- **`fix(store):` enable SQLite `foreign_keys` on every
  connection**.
- **`feat(release):` canonical `Resolver`**; release invoke ignores
  request model/provider.
- **`feat(release):` atomic activation**; partial unique index on
  active releases.
- **`fix(release):` bind vote identity to authenticated principal**
  — closes maker-checker one-person quorum.
- **`feat(eval):` implement `json_schema` scorer**.
- **`fix(harness):` precondition runner gated + env scrubbed +
  process-group kill**.
- **`fix(llm):` preserve role + TopP in OpenAI `Complete`**.
- **`.goreleaser.yml`**: duplicate `release:` block collapsed;
  SBOM `.sig` / `.pem` `extra_files` globs dropped; `signs:`
  block no longer requires `COSIGN_EXPERIMENTAL=1`.
- **`.github/workflows/ci.yaml` `build-release` job**: explicit
  `permissions: contents: write, packages: write, id-token: write,
  attestations: write`; `docker/login-action` step for `ghcr.io`.
- **`scripts/sync-version.sh`**: now synchronises the openapi
  `info.version` comment prose as well as the literal `version`
  field.
- **`Makefile`**: `help` target now lists `docs-check`, `bench`,
  and `docs-site`.

### Fixed

- **`fix(helm):` real probe + scrape paths** — `/health` and
  `/ready` (not `/v1/healthz`); `/metrics` (not `/v1/metrics`).
- **`fix(redactor):` thread-safe `Enable`/`Disable`**; Luhn check
  in `Matches`.
- **`fix(injection):` snapshot-semantic `Enable` +
  `OverrideThreshold`**.
- **`fix(guardrail):` credit-card regex now Luhn-verified**.
- **`fix(schedule):` POSIX DOM/DOW semantics** — wildcard means
  'other field wins'.
- **`fix(observation):` bound window record list to 4096
  entries**.
- **`fix(experiment):` validate variants**; document Confidence =
  sample ratio.
- **`fix(trace):` OTel provider lives for the daemon lifetime**;
  sample 5% by default.
- **`fix(ratelimit):` exempt `/health`, `/ready`, `/metrics`**.
- **`fix(models):` `json:"-"` on `EncryptedKey` and
  `WebhookEndpointRecord.Secret`**.
- **`fix(daemon):` persist failed executions**; surface provider
  errors as 5xx.

### Removed

- **`chore(store):` delete `internal/store/migrations/postgres/`**.
- **`chore(cmd):` drop `approval.Approve` unused-import
  workaround**.
- **`refactor(api):` remove dead `ServerConfig` /
  `WithServerConfig`**.

### Migration

- **Migration 025 destructive gate** —
  `PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true` required.
- **Migration 042** — `recommendation` SQLite-backed repository.
- **Migration 041** — partial unique index on active releases.

## [0.1.0] - 2026-07-10

The v0.1.0 release is the architecture review board's "Forward
only" baseline. Production tenants upgrading from v0.0.7 run
migration 025 (destructive) before the v0.1.0 daemon starts.
The Engineering Completion Protocol's "two consecutive
independent reviews" requirement is met.

### Added

- **Release + Approval lifecycle (`feat(store):` migration 024)**.
  `releases` and `approvals` tables. Schema for the Release
  aggregate (Pending → Approved → Active → Superseded /
  RolledBack) and the per-Release vote trail.
- **`feat(store):` SQLite repository impls** — `CreateRelease` /
  `GetRelease` / `ListReleasesForCapability` /
  `ListActiveReleasesForEnvironment` / `UpdateRelease` /
  `DeleteRelease` plus the matching approval methods, plus the
  new `ActivateAtomic(prior, next)` that persists both writes
  inside a single `*sql.Tx`.
- **`feat(release):` application service** — `release.Service`
  with Create / Vote / Activate / Rollback / Get / List /
  Approval methods; the Activate path is the single place that
  consults the approval policy and supersedes the prior Active
  Release.
- **`feat(api):` release + approval routes**.
- **`feat(api):` Execution fidelity** — both invoke paths now
  populate the full Execution row: `outputs`, `latency_ms`,
  `prompt_tokens`, `completion_tokens`, `total_tokens`,
  `cost_usd`, `model`, `error`, `environment`.
- **`feat(daemon):` release service wiring** —
  `cmd/promptsheond` constructs the `release.Service` from
  `PROMPTSHEON_APPROVAL_POLICY` (default `maker_checker`).
- **`feat(sdk):` release + approval methods**.
- **`feat(cli):` release subcommand**.
- **LLM SDK migration**: Anthropic on
  `anthropics/anthropic-sdk-go`; OpenAI on `openai/openai-go/v3`
  Responses API.
- **`feat(store):` migration 048b (audit backfill tool)** — the
  historical-row backfill for `resource_kind` / `resource_id`
  added by 048a is now an operator command
  (`cmd/promptsheon-auditbackfill`).
- **`feat(workflow):` `Engine.Run` with sequential steps and
  cross-step data flow**; wired into `/api/v1/workflows/run`.
- **`feat(recommendation):` SQLite-backed repository +
  migration 042**.
- **`feat(e2e):` authenticated canonical lifecycle test** —
  bootstrap-token path; audit chain verification.

### Changed

- **`refactor(llm):` Anthropic on `anthropics/anthropic-sdk-go`**.
- **`refactor(llm):` OpenAI on `openai/openai-go/v3` Responses
  API**.
- **`fix(sdk/python):` `/v1/` → `/api/v1/`** so the Python client
  hits the actual server routes.
- **`fix(sdk/typescript):` `/v1/` → `/api/v1/` + regenerate
  `openapi.ts` placeholder**.
- **`refactor(testutil):` `NewManifest` helper** — shared
  capability manifest fixture.
- **`refactor(release):` consume `testdata.NewManifest`** in unit
  tests.
- **`fix(release):` transactional Activate** — Repository gains
  `ActivateAtomic(prior, next)`. New round-trip + rollback test.
- **`feat(release):` Service interface compliance assertion** —
  catches signature drift.
- **`refactor(llm):` provider compliance assertions + drop
  redundant `providerAnthropic` const**.
- **`refactor(store):` drop `CapabilityExists` + unused-import
  workarounds**.
- **`refactor(api):` hoist `auditKeyName` / `auditKeyStatus` /
  `auditKeyVersion` to `middleware.go`**.
- **`feat(cli):` split `cmd/promptsheon/main.go` into `cas.go` +
  `http.go`** — main.go drops from 1274 to 513 lines.

### Removed

- **`chore(llm):` drop Azure / Ollama / NVIDIA NIM providers**.
- **`chore(llm):` remove Ollama pricing entries** in
  `PricingTable`.
- **`chore(store):` delete `internal/store/postgres`** (the
  Postgres backend was half-implemented against the new release
  / approval aggregates).
- **`chore(store):` delete `internal/store/migrations/postgres`**.
- **`chore(banditstore):` delete `internal/banditstore/postgres`**.
- **`chore(config):` drop `DBBackend` / `DBDSN`** fields and the
  `PROMPTSHEON_DB_BACKEND` / `PROMPTSHEON_DB_DSN` env lookups.
- **`docs:` delete `docs/adr/0015-postgres-backend-with-rls.md`**.
- **F-18**: `refactor: drop dead SnapshotTTL retention sweep`.
  The `RetentionPolicy.SnapshotTTL` field and the matching
  "DELETE FROM output_snapshots" sweep in `Enforce` are gone.
- **F-19**: `refactor: rename UsageTracker Prompts/Agents to
  Capabilities`. The legacy "prompts" and "agents" terms are
  replaced with "capabilities".
- **F-20**: `docs: README reflects v0.1.0 Capability-centric
  architecture**.
- **Forward-only breaking cleanups**:
  - **F-01**: Deleted `internal/promptsheon/alias.go` (the M0.7
    CAS re-export shim).
  - **F-04 + F-05**: Deleted the legacy bundle types (`Prompt`,
    `ModelPolicy`, `ContextContract`, `Memory`, `Guardrails`,
    `Tools`, `MCPServers`, `RuntimePolicy`, `EvaluationSuite`)
    and the corresponding fields on `capability.Version`.
  - **F-03**: Deleted `manifestFromLegacy` in
    `handleCreateVersion`.
  - **F-06**: Migration 025 is destructive. Drops the legacy
    `prompts`, `agents`, `prompt_versions`,
    `agent_executions`, `test_datasets`, `eval_results`,
    `eval_runs`, `reviews`, `output_snapshots`,
    `workflows`, `workflow_steps` tables.
  - **F-08**: Renamed `internal/abtesting` to
    `internal/experiment`.
  - **F-09 + F-10**: Deleted `internal/playground` and
    `internal/collab`.
  - **F-13**: OpenAPI spec regenerated. The Version resource
    request body no longer exposes the legacy bundle fields.
  - **F-14**: Version bump to v0.1.0 across
    `sdk/python/pyproject.toml`,
    `sdk/typescript/package.json`,
    `deploy/helm/promptsheon/Chart.yaml`.

### Migration

Production tenants upgrading from v0.0.7 to v0.1.0 must run
migration 025 (destructive) before starting the daemon. There
is no automatic backwards-compat codepath. Operators that still
need the legacy model must roll back to v0.0.7.

## [0.0.7] - 2026-06-26

Last release that supports the legacy bundle model. v0.1.0 is a
breaking semver bump per the charter's "forward only" principle.

## [0.0.5 - Tier 2 follow-on] - 2026-06-26

The Tier 2 follow-on series landed four passes of new primitives,
examples, and wiring on top of the v0.0.5 baseline.

### Added

- **Recommendation loop end-to-end (Tier 1.04).** New
  `internal/observation.Aggregator` rolls `ExecutionRecord`
  values into `(capability, version, env)` windows; new
  `internal/recommendation.Producer` is an `EventBus` subscriber
  that drives `rules.Engine`, persists via a supplied `SinkFunc`,
  and emits one `capability.EventRecommendationGenerated` per
  emitted Recommendation.
- **Canonical Invoke path with Budget + Quota enforcement
  (Tier 1.33 / 1.34).** New `internal/invoke.Invoker` enforces
  Quota before any LLM call (returns `ErrQuotaExceeded` →
  HTTP 429) and enforces Budget against the Caller-reported cost
  (returns `ErrBudgetExceeded` → HTTP 402).
- **Postgres backend with per-workspace RLS (Tier 1.10).**
  `internal/store/postgres` ships the consumer-defined
  `capability.Repository` interface implemented against
  `jackc/pgx/v5`. Migration `025_capabilities.sql` (Postgres)
  mirrors the SQLite schema with TIMESTAMPTZ / JSONB; migration
  `100_rls.sql` enables Row Level Security on every
  per-workspace table.
- **Domain-purity CI gate (Tier 1.07).** New
  `scripts/check-domain-purity.sh` and `make lint-deps` step
  fail CI when any of 14 domain packages imports from
  `internal/llm`, `internal/api`, `internal/store/sqlite`, or
  `cmd/`. Companion to `make lint-domain` (no package-level
  mutable state).
- **Observability Primitive — `WindowAggregator`**.
- **Plugin supervisor with restart budget + health gate
  (Tier 2.46).** New `internal/supervisor.Supervisor`
  implements the in-process Plugin lifecycle.
- **Built-in PII redactor Guardrail (Tier 2.47).** New
  `internal/redactor.Redactor` implements the consumer-defined
  Guardrail interface. Six builtin patterns: email, US SSN,
  E.164 phone, Luhn-verified credit-card 13–19 digits, IPv4,
  IBAN.
- **Built-in prompt-injection heuristic Guardrail
  (Tier 2.48).** New `internal/injection.Detector` scores user
  inputs against 16 heuristics and returns Reject when the score
  crosses the threshold.
- **TypeScript SDK scaffold (Tier 2.41 part 1).**

### Changed

- **Approval→Release wiring closes quorum-reality gap
  (Tier 1.27, the real bug).** `Release.ApproveWith(*Approval,
  Policy)` runs the supplied Policy against recorded votes,
  enforces separation of duties via `VerifySeparationOfDuties`,
  and only advances Status from Pending to Approved on quorum.
- **Migrations: legacy `prompts` / `agents` /
  `prompt_versions` / `agent_executions` /
  `test_datasets` / `eval_results` / `eval_runs` /
  `reviews` / `output_snapshots` / `workflows` /
  `workflow_steps` tables dropped (Tier 1.26, migration 024).**
- **`internal/snapshot` deleted (Tier 1.38).** Subsumed by
  `capability.Execution.Inputs` / `Outputs` (json.RawMessage)
  plus the Replay buffer.
- **`internal/capability/deployment` deleted (Tier 1.39).** The
  Release lifecycle is the canonical deployment record.
- **Import cycle fix**: `internal/optimizer/rules` no longer
  imports `internal/recommendation`; `CanAutoAdopt` moves into
  the rules package as a plain function.

### Removed

- **ADR-0015**: Postgres as a first-class backend (deleted in
  v0.1.0).

[Unreleased]: https://github.com/sachncs/promptsheon/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/sachncs/promptsheon/compare/v0.3.0-rc.1...v0.3.0
[0.3.0-rc.1]: https://github.com/sachncs/promptsheon/compare/v0.2.0...v0.3.0-rc.1
[0.2.0]: https://github.com/sachncs/promptsheon/compare/v0.1.0...v0.2.0
[0.1.x]: https://github.com/sachncs/promptsheon/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sachncs/promptsheon/compare/v0.0.7...v0.1.0
[0.0.7]: https://github.com/sachncs/promptsheon/compare/v0.0.5-tier2...v0.0.7
[0.0.5 - Tier 2 follow-on]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.5-tier2