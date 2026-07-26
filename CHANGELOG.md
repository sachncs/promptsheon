# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Audit / observability**
  - `internal/testutil/otel.go` exposes `InMemoryCollector` (wraps
    `tracetest.InMemoryExporter`) for span assertions in tests.
  - `tests/load/scenarios/*.js` declare k6 thresholds
    (`http_req_duration`, `request_success`); the workflow now
    fails when any scenario breaches its thresholds.
- **CI**
  - `.github/workflows/fuzz.yaml` runs nightly at 03:00 UTC
    (60 s per harness) in addition to the PR gate.
  - `go vet -all ./...` and `staticcheck ./...` are wired into
    the `test` CI job.
- **API surface**
  - `internal/api/validate.go::validateJSON(r, target, validate)`
    combines `readJSON` + field-level validation; companion
    helpers `validateNonEmpty`, `validateEnum`,
    `validatePositiveInt`, `validatePositiveFloat`.
  - `GET /livez` and `GET /readyz` are aliases for the existing
    `/health` and `/ready` probes.
  - `handlers_metrics.go` renamed to `usage.go` to match the
    exported `UsageTracker` type it owns.

### Changed

- **Pagination**: `pagination.go::writePaginationHeaders` emits
  RFC 5988 `prev`/`next`/`first`/`last` link headers + an
  `X-Total-Count` header on every paginated endpoint.
- **Idempotency**: the in-memory `idempotencyCache` is now a
  fallback. Production uses `SQLiteIdempotencyStore` (wiring via
  `cmd/promptsheond/main.go:264`); multi-replica retries share
  state.
- **Metrics**: `metrics.banditMu` switched from `sync.Mutex` to
  `sync.RWMutex`; `GetSummary` and `prometheusFormat` acquire the
  read lock. The `banditRunID` write path keeps an exclusive lock.
- **Self-evolve (`internal/selfevolve`)**: `Evolver.RunOnce`
  always sets `res.Revisions`; on promotion it sets `res.Score =
  state.LastScore` (was previously stuck at the *seeded* score).
- **Audit hash**: `docs/algorithms.md` documents the
  `\x1f`-separated `SHA-256(id \x1f user_id \x1f action \x1f
  resource \x1f details_json \x1f timestamp \x1f previous_hash)`
  format used by `internal/store/sqlite.go::computeAuditHash`.
- **Version source of truth**: the OpenAPI `info.version`,
  `Chart.yaml` `version`/`appVersion`, and the Python / TypeScript
  SDK versions are all pinned at `0.3.0`.

### Removed

- Dead code (dropped across the codebase):
  - `var _ = ...` workarounds in `internal/{trace,rollups/clickhouse,supervisor,policy,lineage,harness,selfevolve,eval}`,
    `cmd/promptsheon/harness.go`, `internal/api/invoke_test_helpers_test.go`,
    `tests/contract/contract_test.go` — and their now-orphaned
    imports.
  - `judgeCache` struct + methods in `internal/eval/scorer_llm_judge.go`
    (32 LOC; never instantiated).
  - `marshalJSON` helper in `internal/recommendation/producer.go`
    (was a thin `json.Marshal` wrapper).
  - `hexDecode` helper in `internal/vault/providers.go`.
  - `mergeCIDRs` in `internal/ratelimit/ratelimit.go`.
  - `(*Compiler).filter` in `internal/reasoning/compiler.go`.
  - `interfaceCtx` alias in `internal/slo/slo.go`.
  - `validateJSON` helper in `internal/api/validate.go` (no callers).
  - `initSQLBundle` / `rlsSQLBundle` package vars + `init()`
    in `internal/store/postgres/postgres.go`.
  - `crashed` field in `internal/subprocess/subprocess.go`.
  - `splitCSVFields` in `internal/schedule/schedule.go`.
  - `newDiscardLogger` (`internal/pluginsup/discard.go`) and
    `newTestSupervisor` (`internal/pluginsup/helpers_test.go`)
    — only consumed by each other.
  - `internal/pluginsup/supervisor_test.go` (tested dead helpers).
  - `LabeledCounter` / `LabeledHistogram` types + benchmarks
    in `internal/metrics/` (never used in production).
  - Deprecated `SystemConfigRow` alias in
    `internal/settings/resolver.go`.
- Dead local variables:
  - Shadowed `dom` in `internal/schedule/schedule.go`
    (`parseField` result discarded before `parseFieldWithWildcard`).
  - `limit = -1` assignments in `internal/store/sqlite.go` and
    `internal/store/sqlite_capabilities.go` (the literal `-1` is
    embedded in the SQL string).
  - Unused `ctx` return value in
    `internal/trace/otel.go::(*OTelTracer).Start`.

### Fixed

- **Self-evolve `Result`**: `res.Score` is now set to the score of
  the last validation run (via `state.LastScore`) on promotion;
  was previously the *old* seeded score. `res.Revisions` is
  always populated, including on rejection.
- **E2E seed**: `tests/e2e/selfevolve_test.go::seedCapabilityWithBadPrompt`
  now checks every `Create*` error return, sets the dataset's
  `CapabilityID`, and shares the auditor instance between the
  evolver and promoter. The previous form silently swallowed
  FK-violation errors.
- **Audit doc drift**: `docs/algorithms.md` now describes the
  field-separator format actually produced by `computeAuditHash`,
  not a hypothetical JSON canonicalisation.

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