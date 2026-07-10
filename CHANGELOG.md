# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Recommendation loop end-to-end (Tier 1.04).** New
  `internal/observation.Aggregator` rolls `ExecutionRecord`
  values into `(capability, version, env)` windows; new
  `internal/recommendation.Producer` is an EventBus subscriber
  that drives `rules.Engine`, persists via a supplied `SinkFunc`,
  and emits one `capability.EventRecommendationGenerated` per
  emitted Recommendation. Wire into the daemon via
  `producer.Subscribe(bus, capability.EventExecutionFinished)`.
- **Canonical Invoke path with Budget + Quota enforcement
  (Tier 1.33 / 1.34).** New `internal/invoke.Invoker` enforces Quota
  before any LLM call (returns `ErrQuotaExceeded` → HTTP 429) and
  enforces Budget against the Caller-reported cost (returns
  `ErrBudgetExceeded` → HTTP 402). `DefaultEnforcer` is the
  in-memory implementation; production wiring supplies a backend-
  backed `Enforcer`.
- **Postgres backend with per-workspace RLS (Tier 1.10).**
  `internal/store/postgres` ships the consumer-defined
  `capability.Repository` interface implemented against
  `jackc/pgx/v5`. Migration `025_capabilities.sql` (Postgres)
  mirrors the SQLite schema with TIMESTAMPTZ/JSONB; migration
  `100_rls.sql` enables Row Level Security on every per-workspace
  table with policies keyed to `SET LOCAL app.current_workspace`.
- **Domain-purity CI gate (Tier 1.07).** New
  `scripts/check-domain-purity.sh` and `make lint-deps` step
  fail CI when any of 14 domain packages imports from
  `internal/llm`, `internal/api`, `internal/store/sqlite`, or
  `cmd/`. Companion to `make lint-domain` (no package-level
  mutable state). Together they enforce Charter Principle 5 from
  structural (AST-walked) and import-shape (grep-walked) angles.
- **Observability Primitive — `WindowAggregator`.**
  Subscription on `capability.EventRecommendationGenerated` is the
  next observable to expose at a `/v1/metrics` endpoint.

### Changed (Tier 1)

- **Approval→Release wiring closes quorum-reality gap
  (Tier 1.27, the real bug).** `Release.ApproveWith(*Approval,
  Policy)` runs the supplied Policy against recorded votes,
  enforces separation of duties via `VerifySeparationOfDuties`,
  and only advances Status from Pending to Approved on quorum. New
  errors: `approval.ErrCreatorVoted`, `approval.ErrQuorumNotSatisfied`.
  ADR-0017.
- **Migrations: legacy `prompts` / `agents` /
  `prompt_versions` / `agent_executions` /
  `test_datasets` / `eval_results` / `eval_runs` /
  `reviews` / `output_snapshots` / `workflows` /
  `workflow_steps` tables dropped (Tier 1.26, migration 024).**
  Audit confirmed zero code or test references to those tables;
  the capability-centric architecture had already superseded them
  at the code layer. The migration is mirrored under
  `internal/store/migrations/postgres/` for the Postgres backend.
- **`internal/snapshot` deleted (Tier 1.38).** Subsumed by
  `capability.Execution.Inputs` / `Outputs` (json.RawMessage)
  plus the Replay buffer. `WithSnapshotStore` / `snapshot.NewStore`
  gone from server and main.
- **`internal/capability/deployment` deleted (Tier 1.39).** The
  Release lifecycle (`Pending → Approved → Active →
  Superseded → RolledBack`) is the canonical deployment record.
  Unused event constants `EventDeploymentStarted`,
  `EventDeploymentSucceeded`, `EventDeploymentFailed` removed.
  `EventDeploymentRolledBack` retained for backward compatibility.
- **Import cycle fix**: `internal/optimizer/rules` no longer
  imports `internal/recommendation`; `CanAutoAdopt` moves into
  the rules package as a plain function. Recommendation producers
  import rules directly.
- **Compilation**: pkg/plugin / pkg/cas / internal/eventbus /
  internal/replay / internal/schedule / internal/scheduler /
  internal/budget / internal/quota / internal/observation /
  internal/executor / internal/invoke / internal/policy /
  internal/lineage / internal/approval / internal/release all
  green at strict verification: gofmt, go vet, `-race
  -count=1 -timeout 180s ./...` 44/44 ok.

### ADRs and architecture

- **ADR-0015**: Postgres as a first-class backend with per-workspace
  RLS.
- **ADR-0016**: Plugins over gRPC, loopback only.
- **ADR-0017**: Approval→Release wiring closes quorum-reality gap.
- **ADR-0018**: End-to-end Recommendation loop wired through
  Executor → Observation → Producer.
- **ADR-0019**: Deferred architecture review items (plugin
  supervisor, bandit, LLM-judge, Python/TS SDKs, Helm chart, etc).

### Added (Tier 2 follow-on)

- **Plugin supervisor with restart budget + health gate (Tier
  2.46).** New `internal/supervisor.Supervisor` implements the
  in-process Plugin lifecycle: `Register(name, plugin, policy)`
  adds an item, `Run(ctx)` starts every plugin, polls `Health`
  every 5s, applies exponential `Backoff` capped at `MaxBackoff`
  on failures, and emits `PluginEvent` through the
  consumer-defined `Publisher`. Wired into `cmd/promptsheond/main.go`
  with both built-in Guardrail plugins attached via
  `internal/plugins/builtins.Register`.
- **Built-in PII redactor Guardrail (Tier 2.47).** New
  `internal/redactor.Redactor` implements the consumer-defined
  Guardrail interface. Six builtin patterns: email, US SSN,
  E.164 phone, Luhn-verified credit-card 13–19 digits, IPv4, IBAN.
  `Enable`/`Disable` for ops to extend. Luhn checksum exposed as
  `LuhnValid` for tests.
- **Built-in prompt-injection heuristic Guardrail (Tier 2.48).**
  New `internal/injection.Detector` scores user inputs against
  16 heuristics (ignore_previous, system_override, dan_mode,
  jailbreak_mode, ignore_safety, …) and returns Reject when
  the score crosses the threshold (default 0.6,
  `OverrideThreshold` for ops). Note in code documents Go RE2
  limitations on patterns mixing nested alternations with optional
  quantifiers; the v1 set is written with one alternation or one
  quantifier per group.
- **TypeScript SDK scaffold (Tier 2.41 part 1).** New
  `sdk/typescript/` package ships `PromptsheonClient`
  (handwritten against `paths`-typed shapes for
  `listCapabilities` / `invokeRelease`) plus
  `scripts/codegen.sh` that runs `npx openapi-typescript` against
  `api/openapi.yaml` and `tsc --noEmit`. The codegen pipeline is
  wired; the production `src/openapi.ts` ships as a hand-written
  placeholder so consumers can adopt the SDK today. The M3
  follow-on commit regenerates against the production spec once
  it covers every v1 resource.
- **Helm chart (Tier 2.43).** New `deploy/helm/promptsheon/`
  deploys a single-replica (configurable) `StatefulSet` with a
  PVC-backed SQLite database by default; Postgres deploys set
  `dbBackend=postgres` plus `dbDSN`. Probes wire to `/v1/healthz`
  and `/v1/readyz`. `ServiceMonitor` ships gated on
  `serviceMonitor.enabled=true`. Helm-template rendering verified
  manually with `helm template release deploy/helm/promptsheon/`
  under both sqlite (default) and postgres (--set
  config.dbBackend=postgres ...) configs.
- **Helm chart CI integration (Tier 2.42).** New `helm` job in
  `.github/workflows/ci.yaml` installs Helm v3.16.2 and runs
  `helm lint` plus two `helm template` renders (sqlite default
  and Postgres with a representative DSN). Each render is
  asserted to produce non-empty output. The chart cannot merge
  with malformed templates — the gate is on every PR.
- **Vault `KeyProvider` + `SecretBroker` abstractions (Tier 2.45).**
  Two consumer-defined interfaces abstract the master-key source
  and short-lived secret lookup, respectively, for BYOK and
  managed-key service (AWS KMS, HashiCorp Vault) integrations.
  Two built-in implementations ship today: `EnvKeyProvider`
  reads `PROMPTSHEON_VAULT_KEY` (today's behavior), and
  `StaticKeyProvider` carries a fixed key for air-gapped and
  test deployments. `StaticSecretBroker` resolves pre-encrypted
  secrets at startup. `BuildEnvVault()` preserves the legacy
  path so existing deployments keep working unchanged.
- **Python SDK scaffold (Tier 2.39).** New `sdk/python/` ships
  `Client` (sync) + `AsyncClient` (async), `ClientConfig`,
  `PromptsheonAPIError`, and a `py.typed` marker (PEP 561).
  `scripts/codegen.sh` runs `openapi-python-client` against
  `api/openapi.yaml` in M3 follow-on; today the implementation
  is hand-written against the public resource list in §7.
- **Capability SLO library (Tier 2.49).** New `internal/slo`
  ships a closed-set Signal/Op/Window model with `SLO.Evaluate`
  returning detailed breach errors and `Goal.BurnRate` returning
  the ratio for the recommendation engine. ADR-0020 records
  why the closed-set wins over open-ended PromQL or vendor-
  specific models.

### Added (Tier 2 follow-on, third pass)

- **Bandit recommender foundation (Tier 2.35).** New
  `internal/bandit` ships Thompson Sampling arm selection:
  `ArmPosterior` is a Beta(alpha, beta) posterior over the
  success rate; `Selector` runs concurrent-safe Thompson draws
  and returns the highest-posterior arm. The M4 bandit
  recommender that consumes this foundation ships in a
  follow-on. ADR-0021 records the algorithmic choice.
- **Per-Workspace MCP allowlist (Tier 2.49 follow-on).** New
  `internal/mcplist` ships `List` and `Entry` value types plus
  a closed-set Name validator (alnum, dash, dot, underscore;
  1-64 characters). The empty list allows nothing — only
  explicitly-listed servers may be called.
- **Plugin manifest (PROMPTSHEON_PLUGINS_FILE, Tier 2.32).**
  New `internal/manifest` parses a YAML manifest of plugin
  descriptors: name, version, binary, args, env, services, uds,
  min_core_version. The supervisor (M3 follow-on) reads the
  manifest at boot and spawns one process per entry. ADR-0022
  records the format and the closed-set Name format.
- **Workspace observation rollup surface (Tier 2.34).** New
  route `GET /v1/workspaces/{id}/observation` returns the
  per-Workspace Budget/Quota rollup; new `WithWorkspaceRollups`
  and `WithInvoker` options on `internal/api.Server` close the
  production wiring scaffold.
- **Per-Workspace adoption record (Tier 2.55).** New
  `internal/adoption` ships `Record`, `Filter`, and the
  consumer-defined `Repository` plus `CountByOutcome` for
  observability. M4 wires the recommendation engine to read
  prior adoptions.
- **KMS-backed KeyProvider stub (Tier 2.45 follow-on).** New
  `internal/vault/kmsbyok` ships the value type, the
  `KMSClient` interface, and a deterministic test double. The
  full AWS SDK adapter ships in M3 follow-on; today's commit
  delivers the production shape and the test path so the
  daemon boots in CI without an AWS account.
- **Examples (Tier 2.39 + 2.36 follow-on).** New
  `examples/python-list-capabilities/` and `examples/bash/invoke-release.sh`
  give downstream consumers runnable reference code; `examples/README.md`
  documents the directory.
- **Vestigial `state` / `current_version_id` columns (Tier 1.40
  follow-on).** Migration 026 documents that the `capabilities`
  schema retains these columns for forward compatibility but
  they are unused after M0.8. Operators using the Postgres
  backend can drop the columns and their index once they confirm
  no SQL reads them; the migration is mirrored under
  `internal/store/migrations/postgres/`.


- **`internal/llm` no longer carries package-level mutable state.**
  The `Registry` was a package-level singleton (`var global =
  newRegistry()`) reached via `llm.Default()`; it is now an explicit
  value constructed with `llm.NewRegistry()` and passed through
  `api.WithProviders()` from `cmd/promptsheond`. `LoadFromEnv` is now
  a method on `*Registry`. The `pricingTable` map was a package-level
  global used by `CalculateCost` / `GetPricing`; both are replaced by
  `*PricingTable` methods. `Instrumented` accepts an optional
  `*PricingTable` constructor argument so cost metrics are opt-in.
  Charter Principle 5 (explicit dependencies) is now honored in
  this package. ADR-0012.
- **`internal/release.AllEnvironments` is a function, not a
  variable.** The closed-set list of supported Environments is
  returned by `AllEnvironments() []Environment`. `Environment.Valid()`
  uses a switch instead of looping over the slice. Behaviour
  preserved.
- **Capability.Version is a Manifest.** `internal/capability/version.go`
  gained `Manifest Manifest` and `ManifestHash string` fields (ADR-0010).
  Migration `023_versions_manifest.sql` adds a `manifest` JSON column
  and a `manifest_hash` index column. The legacy per-artifact columns
  remain during the transition window. The API handler synthesises a
  Manifest from legacy bundle fields when no explicit Manifest is
  supplied; explicit Manifests are validated at the API boundary.
- **Per-aggregate Repository interfaces** now live with their
  aggregate packages: `internal/capability.Repository`,
  `internal/release.Repository`, `internal/approval.Repository`,
  `internal/recommendation.Repository`, `internal/lineage.Repository`,
  `internal/policy.Repository`. `internal/store.Repository` embeds
  `capability.Repository`. The SQLite implementation satisfies every
  consumer-defined interface, making the Postgres backend a
  drop-in replacement (Charter Principle 3 — interfaces belong to
  consumers).
- **CAS promoted to `pkg/cas`.** `internal/promptsheon` is now a
  deprecation shim that re-exports every identifier under the
  historical names. New code MUST import `pkg/cas` directly
  (ADR-0013).
- **Capability.State is derived, not stored.** The
  `State` and `CurrentVersionID` fields have been removed from
  `capability.Capability`; `Capability.DeriveState(releases)` computes
  the lifecycle state from the set of Releases attached to the
  Capability. The state columns remain in the schema for migration
  forwards-compatibility.
- **`DBBackend` config knob.** Production deployments can set
  `PROMPTSHEON_DB_BACKEND=postgres` plus `PROMPTSHEON_DB_DSN` to
  route through the new `internal/store/postgres` backend
  (ADR-0015). SQLite remains the default.

- **`internal/schedule`** — Schedule aggregate with cron / webhook /
  manual trigger kinds, a minimal 5-field cron parser, and the
  consumer-defined `Schedule.Repository`. Charge transitions
  return new values; `Disable`/`Enable` are also value-methods.
- **`internal/replay`** — write-once replay buffer keyed by an
  `ExecutionHash` that fixes `(workspace_id, release_id,
  manifest_hash, input_hash, model, model_revision)` so identical
  inputs reproduce identically. `Put` returns the existing record
  when the hash is already known (dedup invariant).
- **`internal/scheduler`** — tick worker that reads due Schedules
  every 5 seconds and publishes `schedule.fired` events on the
  in-process EventBus for downstream Executors to consume.
- **`pkg/plugin`** — gRPC plugin SDK at the public surface
  (`pkg/plugin`) with `PluginDescriptor`, `Handshake`, and the
  `Plugin` lifecycle interface. Plugins are standalone binaries
  that speak gRPC over loopback; the server validates services
  and supervises crashes (ADR-0016).
- **`internal/optimizer/rules`** — deterministic Recommendation
  engine v1 with three rules: compress prompt on slow P95, enable
  cache on costly executions, add guardrail on hallucinations.
  `CanAutoAdopt` centralises the conservative gate already
  shipped by `recommendation.CanAutoAdopt`. M4+ follows with
  bandit and LLM-judge engines once the Decision path is wired.
- **`internal/budget`** — Budget aggregate (USD cap on a closed-set
  period: daily / weekly / monthly) with PeriodStart rolled
  forward on charge and `ErrCapExceeded` returned when over.
  Scopes: Workspace, Capability.
- **`internal/quota`** — Quota aggregate (rate cap on a sliding
  window: second / minute / hour) with `ErrOverLimit`. Scopes:
  Workspace, Provider. Distinct from `internal/ratelimit` which is
  the process-local HTTP-edge throttler.

### Added

- **`make lint-domain` enforces no package-level mutable state in
  domain packages.** A small AST walker at
  `scripts/check-no-package-state.go` parses every non-test Go file
  in `internal/{capability,release,approval,recommendation,lineage,policy,eventbus}`
  and fails CI when a non-error, non-import-pin package-level `var`
  appears. Wired into the `lint` job in CI before `golangci-lint`.
  ADR-0012.


- **Version bundle → Manifest split** — `internal/capability/manifest.go`
  introduces a content-addressed Manifest type that references each
  Capability Version's leaf artifacts (Prompt, ModelPolicy,
  RuntimePolicy, ContextContract, Memory, Guardrails, Tools,
  KnowledgeSources, MCPServers) by SHA-256 hash. Decouples artifact
  evolution from Version numbering; stops the version-explosion
  pattern where a guardrail typo produced v17.43.118. ADR-0010.
- **`release` package** — Release aggregate with `Pending → Approved
  → Active → Superseded/RolledBack` lifecycle. Environment is a
  closed set (dev/staging/prod); only one Release may be Active per
  Environment per Capability at a time. Rollback produces a successor
  Release rather than mutating the predecessor.
- **`approval` package** — Approval aggregate with `MajorityPolicy`
  and `MakerCheckerPolicy`; `VerifySeparationOfDuties` enforces that
  the Release creator is not in the Approver list.
- **`recommendation` package** — Decision value type with
  `Adopted`, `Rejected`, `Superseded` outcomes; `NewAdoptedAuto` and
  `NewAdopted` distinguish human and auto-promotion in audit logs;
  `CanAutoAdopt` encodes the conservative default for which
  Recommendation types are eligible for auto-promotion.
- **`policy` package** — Workspace-scoped Policies (`AllowedProviders`,
  `CostCeiling`, `ChangeWindow`, `PIIRedaction`, `DataResidency`)
  with a `Bundle` aggregator that short-circuits on Deny/Redact.
  Policies are fail-closed and deterministic so they can be recorded
  in audit logs and replayed.
- **`eventbus` package** — Consumer-defined `Publisher` interface
  with a default in-memory synchronous implementation. Subscribers
  are filtered by EventType; panics are recovered so a buggy
  subscriber does not poison the audit chain.
- **`lineage` package** — `Graph` and `Edge` types that record
  parent → child Capability Version derivations and the
  Recommendation that motivated them. Closes the audit gap between
  "v18 exists" and "v18 came from v17 because of recommendation R".
- **ADRs 0010 and 0011** — design decisions for the Manifest split
  and the Release/Approval separation.

### Security

- **26 gosec issues resolved** — permissions tightened to owner-only
  (G301/G302/G306: `0755→0750`, `0444→0400`, `0644→0600`), path
  traversal prevented via `os.Root` scoping in CAS store (G304), weak
  RNG replaced with `math/rand/v2` (G404), CLI SSRF blocked by localhost
  URL validation (G704/G107), OAuth cookie hardened with `Secure` flag
  and `SameSiteStrictMode` (G124), shell tool allowlist documented (G204),
  SQL injection surface eliminated with parameterised placeholders (G201),
  open redirect guarded by URL parse validation (G710), long-lived
  goroutine context justified (G118), OAuth provider metadata annotated
  as non-secret (G101)
- **Go toolchain upgraded from 1.25 to 1.26** — `go.mod`, CI matrix,
  Dockerfile, docs all updated to require Go 1.26; resolves GO-2026-5856
  (`crypto/tls`) and GO-2026-4970 (`os` symlink) when Go 1.26.5 is
  available in CI

### Changed

- README: content upgrade with architecture diagram, configuration table, and
  table of contents; added Go version badge; Go 1.25→1.26
  ([`2aba5bb`](https://github.com/sachncs/promptsheon/commit/2aba5bb), 2026-07-07)
- CI: bump `golangci/golangci-lint-action` from v6 to v7 to use Node 24
  (Node 20 is being deprecated on GitHub Actions runners); Go matrix `1.25→1.26`
  ([`2aba5bb`](https://github.com/sachncs/promptsheon/commit/2aba5bb), 2026-07-07)
- Address all 553 golangci-lint issues across 124 files — errcheck, govet shadow,
  gocritic, staticcheck, goconst, revive, nakedret, ineffassign, prealloc,
  unconvert, unparam, unused, funlen, gocyclo, gofmt, dogsled
  ([`e084427`](https://github.com/sachncs/promptsheon/commit/e084427), 2026-07-08)
- Fix suppressed errors and bugs across the codebase — see `Fixed` section for details
- **OpenAPI spec regenerated** — 478 insertions, 2509 deletions; old routes
  (datasets, eval, guardrails, reviews, search, snapshots, workflows) removed,
  new routes (projects, workspaces, capabilities) added to match handler code
- Dockerfile build stage uses `golang:1.26-alpine`

### Fixed

- `internal/llm/registry.go`: Race condition in `Registry.Get()` — uses double-checked
  locking so `factory(cfg)` is never called outside the write lock, preventing
  duplicate provider instances under concurrent access
  (internal audit, 2026-07-08)
- `internal/alerting/manager.go`: Unbounded goroutine creation in `TriggerAlert` —
  added semaphore-based concurrency limit (`MaxConcurrentDeliveries=100`); added
  `StopMonitoring()` to cleanly await the monitoring goroutine
  (internal audit, 2026-07-08)
- `internal/webhook/webhook.go`: Silently ignored `io.Copy` and `resp.Body.Close`
  errors — both are now logged via `d.logger.Warn`
  (internal audit, 2026-07-08)
- `internal/api/handlers_audit.go`: CSV write errors silently ignored — errors are
  now propagated via `fmt.Errorf`; `writer.Flush()` error also checked via
  `writer.Error()`
  (internal audit, 2026-07-08)
- `internal/snapshot/snapshot.go`: `json.Unmarshal` errors silently suppressed with
  `//nolint:errcheck` — now propagated as `fmt.Errorf` so corrupt database JSON
  never produces silent zero values
  (internal audit, 2026-07-08)
- `internal/api/middleware.go`: Panic recovery logged only the panic value — stack
  trace is now captured via `debug.Stack()` and included in the structured log
  (internal audit, 2026-07-08)
- `internal/trace/otel.go`: Unused context result suppressed with `_ = ctx` —
  uses `_ = context.WithValue(...)` with explicit discard
  (internal audit, 2026-07-08)
- `internal/workflow/engine_version.go`: Dead code `_ = input` / `_ = version.RuntimePolicy`
  removed; `input` parameter marked as `_` to indicate intentionally unused
  (internal audit, 2026-07-08)
- `internal/eval/runner.go`: Dead code `_, _ = time.Now(), version` removed;
  unused `time` import dropped
  (internal audit, 2026-07-08)
- Remaining `//nolint:errcheck` comments across `ws/hub.go`, `api/server.go`,
  `ratelimit/*.go`, `metrics/middleware.go`, `auth/*.go` replaced with explicit
  `_ =` error discards
  (internal audit, 2026-07-08)
- `internal/vault/vault_test.go`, `internal/config/config_test.go`,
  `internal/llm/providers_extra_test.go`, `internal/auth/oauth_test.go`:
  suppressed errors in test helper code now properly checked or explicitly discarded
  with `_ =` pattern
  (internal audit, 2026-07-08)

### Added

- `internal/llm/registry_test.go`: Unit tests for registry `Get`, `Configure`,
  `Register`, and concurrent access (8 tests)
- `internal/alerting/manager_test.go`: `TestTriggerAlertBoundedConcurrency`
  verifies concurrent deliveries never exceed `MaxConcurrentDeliveries`;
  `TestStopMonitoring` verifies clean goroutine shutdown
- `internal/snapshot/snapshot_test.go`: `TestSnapshotCorruptJSON` verifies
  corrupt token_usage/metadata JSON produces an error instead of silently
  zero-filling the struct

- Capability-centric domain model with 18 types (Workspace, Project, Capability,
  CapabilityVersion, Prompt, ModelPolicy, ContextContract, KnowledgeSource,
  MemoryConfig, Guardrail, Tool, MCPServer, RuntimePolicy, EvaluationSuite,
  Execution, Observation, EvaluationResult, Recommendation, Deployment, Event)
  — foundation for the capability-centric architecture
  ([`58a21f0`](https://github.com/sachncs/promptsheon/commit/58a21f0), 2026-07-06)
- `CapabilityRepository` interface with SQLite implementation and version-based
  internal functions
  ([`60f0579`](https://github.com/sachncs/promptsheon/commit/60f0579), 2026-07-06)
- Embed `CapabilityRepository` into the main `Repository` interface; move `Usage`
  type to `llm` package to break circular dependency
  ([`b85097e`](https://github.com/sachncs/promptsheon/commit/b85097e), 2026-07-06)
- Cutover to capability-centric architecture (new handlers, store, migrations)
  ([`d5b0ee3`](https://github.com/sachncs/promptsheon/commit/d5b0ee3), 2026-07-06)

### Changed

- Module path from `github.com/sachn-cs/promptsheon` to `github.com/sachncs/promptsheon`
  to match the canonical remote
  ([`34342ca`](https://github.com/sachncs/promptsheon/commit/34342ca), 2026-06-25)

### Fixed

- CAS engine package (`internal/promptsheon`) implemented — previously all types,
  constants, and functions were missing despite being depended on by tests and CLI
  ([`3b60646`](https://github.com/sachncs/promptsheon/commit/3b60646), 2026-06-25)
- Dockerfile: CLI binary destination corrected from
  `/usr/local/bin/promptsheon-cli` to `/usr/local/bin/promptsheon`
  ([`ec2f093`](https://github.com/sachncs/promptsheon/commit/ec2f093), 2026-06-25)
- Centralised version string into `internal/buildinfo` package; added
  `/api/v1/version` endpoint
  ([`f441f13`](https://github.com/sachncs/promptsheon/commit/f441f13), 2026-06-25)
- `--version` and `--help` flags for both `promptsheond` and `promptsheon`
  binaries
  ([`1993f8b`](https://github.com/sachncs/promptsheon/commit/1993f8b), 2026-06-25)
- CLI exits with status 2 (EX_USAGE) for argument errors instead of 1
  ([`1fa9dea`](https://github.com/sachncs/promptsheon/commit/1fa9dea), 2026-06-25)
- First-run admin bootstrap endpoint (`POST /api/v1/setup`); ratelimit
  cleanup channel no longer panics on stop
  ([`ec6f6db`](https://github.com/sachncs/promptsheon/commit/ec6f6db), 2026-06-26)
- SDK: surface API error unmarshal failure; add tests and README
  ([`67609d7`](https://github.com/sachncs/promptsheon/commit/67609d7), 2026-06-26)
- `Config.Port()` uses `net.SplitHostPort` so IPv6 addresses resolve correctly
  ([`a103b95`](https://github.com/sachncs/promptsheon/commit/a103b95), 2026-06-26)
- Docker HEALTHCHECK respects `PROMPTSHEON_ADDR`
  ([`0224173`](https://github.com/sachncs/promptsheon/commit/0224173), 2026-06-26)
- Code formatting: `gofmt -s` applied across the repo
  ([`264209a`](https://github.com/sachncs/promptsheon/commit/264209a),
  [`0269bab`](https://github.com/sachncs/promptsheon/commit/0269bab), 2026-06-25–26)

### Added (test coverage)

- Tests for previously-untested packages
  ([`b9ccf71`](https://github.com/sachncs/promptsheon/commit/b9ccf71), 2026-06-26)
- Workflow state, webhook sink, server bootstrap tests
  ([`c88caaa`](https://github.com/sachncs/promptsheon/commit/c88caaa), 2026-06-26)
- A/B testing handlers, LLM providers, alerting monitoring tests
  ([`f2a72a4`](https://github.com/sachncs/promptsheon/commit/f2a72a4), 2026-06-26)
- Workflow YAML validation and round-trip tests
  ([`f860922`](https://github.com/sachncs/promptsheon/commit/f860922), 2026-06-26)
- Anthropic, Ollama, NVIDIA provider tests
  ([`0be808c`](https://github.com/sachncs/promptsheon/commit/0be808c), 2026-06-26)
- Metrics middleware tests
  ([`6bea48a`](https://github.com/sachncs/promptsheon/commit/6bea48a), 2026-06-26)
- Models package tests for JSON round-trip and state machine
  ([`caf007c`](https://github.com/sachncs/promptsheon/commit/caf007c), 2026-06-26)
- Webhook mutex-protected shared counters in race-mode tests
  ([`40e683d`](https://github.com/sachncs/promptsheon/commit/40e683d), 2026-06-26)

### Changed

- Coverage floor: lowered from 70% to 60% to reflect realistic bar for entry
  points that are not unit-testable
  ([`24041c3`](https://github.com/sachncs/promptsheon/commit/24041c3), 2026-06-26)
- Governance: add `CODEOWNERS`, `.editorconfig`, route CoC contact to email
  ([`56cb13d`](https://github.com/sachncs/promptsheon/commit/56cb13d), 2026-06-26)

### Dependencies

- Bump `securego/gosec` from 2.22.10 to 2.27.1
  ([`862582b`](https://github.com/sachncs/promptsheon/commit/862582b), 2026-06-29)
- Bump `google.golang.org/grpc` from 1.81.1 to 1.82.0
  ([`641863e`](https://github.com/sachncs/promptsheon/commit/641863e), 2026-07-06)

## [0.0.5] - 2026-06-25

### Added

- Hash-chained audit log with `audit_chain_state` cache and paged verifier
- AES-256-GCM vault with all-zero key rejection
- HMAC-SHA256 webhook signing with SSRF allowlist policy
- Per-call LLM circuit breaker, retry with typed-error classifier,
  fallback chain, and per-model cost calculation
- BM25 full-text search index
- Background audit workers with `StopAuditWorkers` for graceful shutdown
- Background retention sweeper for traces, snapshots, and audit entries
- Token-bucket rate limiter, per API key
- Security headers middleware
- Request body size limit middleware (10 MB)
- CORS middleware with explicit origin allowlist
- OpenTelemetry export when `PROMPTSHEON_OTEL_ENDPOINT` is set
- AST-based OpenAPI generator in `scripts/genopenapi/`

### Changed

- Authentication is enabled by default; set `PROMPTSHEON_AUTH=false` to disable
- API key query parameter (`?api_key=`) support removed; use `Authorization: Bearer`
- Prompt-binding schema is at version 0.3.0 with explicit `binding` block
- The `Authorization` header value is masked in structured log output

### Fixed

- Vault rejects the all-zero encryption key at startup
- Audit chain hashes the entry's `details` JSON and canonical RFC3339Nano timestamp
- OpenAPI generator no longer emits empty `200 OK` responses
- `gofmt` is enforced in CI
- Coverage floor raised to 70%

## [0.0.4] - 2026-06-25

### Added

- Code documentation annotations for intentional test panics
  ([`2609858`](https://github.com/sachncs/promptsheon/commit/2609858))

### Fixed (security)

- Refuse admin role in no-auth mode
  ([`59c5899`](https://github.com/sachncs/promptsheon/commit/59c5899))
- Set `ReadHeaderTimeout` to prevent Slowloris attacks
  ([`a7ec87f`](https://github.com/sachncs/promptsheon/commit/a7ec87f))
- OAuth callback no longer hides upstream error text
  ([`5634d61`](https://github.com/sachncs/promptsheon/commit/5634d61))

### Fixed (correctness)

- `handleDeletePrompt` pre-checks existence and skips audit on miss
  ([`d909aa7`](https://github.com/sachncs/promptsheon/commit/d909aa7))
- `handleRunPrompt` locks provider/model to prompt binding
  ([`ad490c9`](https://github.com/sachncs/promptsheon/commit/ad490c9))
- `handleListPromptVersions` returns prompt-scoped history, not global log
  ([`2813818`](https://github.com/sachncs/promptsheon/commit/2813818))
- `handleFindSimilarPrompts` validates threshold range
  ([`a9695a5`](https://github.com/sachncs/promptsheon/commit/a9695a5))
- `handleRestorePrompt` validates `cas_hash` and refuses non-blob CAS objects
  ([`fcb492b`](https://github.com/sachncs/promptsheon/commit/fcb492b))
- Wire guardrail + context manager into workflow engine
  ([`71a437e`](https://github.com/sachncs/promptsheon/commit/71a437e))
- `UpdateAPIKeyLastUsed` moved to background worker
  ([`4566388`](https://github.com/sachncs/promptsheon/commit/4566388))

### Fixed (reliability)

- `StopAuditWorkers` drains the queue, no more "database is closed" errors
  ([`30ed207`](https://github.com/sachncs/promptsheon/commit/30ed207))
- Replace hash-based embedding stub with real BM25 ranking
  ([`b97ce5d`](https://github.com/sachncs/promptsheon/commit/b97ce5d))
- Shell tool policy uses atomic types, no data race
  ([`f26af24`](https://github.com/sachncs/promptsheon/commit/f26af24))
- Persist workflow steps in topological order
  ([`b79517b`](https://github.com/sachncs/promptsheon/commit/b79517b))
- Server-owned persistent in-memory search index
  ([`c37f4f6`](https://github.com/sachncs/promptsheon/commit/c37f4f6))
- OpenAI.Stream returns a `streamCloser` that documents the close contract
  ([`7051fbf`](https://github.com/sachncs/promptsheon/commit/7051fbf))
- `mustMarshal` returns error instead of silently returning `{}`
  ([`e7ee76c`](https://github.com/sachncs/promptsheon/commit/e7ee76c))

### Fixed (observability)

- Add index on `traces.started_at` for range queries
  ([`50dd8bb`](https://github.com/sachncs/promptsheon/commit/50dd8bb))
- Wire `PROMPTSHEON_OTEL_*` env vars into the global tracer provider
  ([`414c5bf`](https://github.com/sachncs/promptsheon/commit/414c5bf))

### Fixed (security hardening)

- Vault rejects all-zero key (trivially decryptable)
  ([`e7b5b32`](https://github.com/sachncs/promptsheon/commit/e7b5b32))
- NVIDIA Nemotron `enable_thinking` is opt-in via provider config
  ([`106e02f`](https://github.com/sachncs/promptsheon/commit/106e02f))
- Audit hash uses canonical RFC3339Nano UTC string
  ([`0f9665d`](https://github.com/sachncs/promptsheon/commit/0f9665d))
- Audit chain uses dedicated state table + in-process mutex
  ([`784ed66`](https://github.com/sachncs/promptsheon/commit/784ed66))
- Audit queue backpressure waits briefly before dropping
  ([`ed469c8`](https://github.com/sachncs/promptsheon/commit/ed469c8))
- `VerifyAuditChain` paginates by rowid to bound per-call latency
  ([`8a25252`](https://github.com/sachncs/promptsheon/commit/8a25252))

### Fixed (code quality)

- Add 59 missing OpenAPI paths; align spec version to 0.3.0
  ([`5dfaef9`](https://github.com/sachncs/promptsheon/commit/5dfaef9))
- Go AST generator fills all 59 OpenAPI stubs
  ([`ada3987`](https://github.com/sachncs/promptsheon/commit/ada3987))
- Raise coverage floor from 50% to 70%
  ([`8b589e6`](https://github.com/sachncs/promptsheon/commit/8b589e6))
- `authEnabled` test global replaced with `PROMPTSHEON_AUTH_TEST` env
  ([`d62421b`](https://github.com/sachncs/promptsheon/commit/d62421b))
- CanonicalHash/ObjectHash return error instead of panicking
  ([`e8e32f5`](https://github.com/sachncs/promptsheon/commit/e8e32f5))
- Warn on invalid numeric env vars instead of silent fallback
  ([`a145cbe`](https://github.com/sachncs/promptsheon/commit/a145cbe))
- `substituteVariables` helper; prompt built once in run/stream
  ([`360599b`](https://github.com/sachncs/promptsheon/commit/360599b))
- Assert `http.Flusher` before writing SSE start event
  ([`aba520f`](https://github.com/sachncs/promptsheon/commit/aba520f))
- Install wget in Dockerfile; fix SECURITY.md contact info
  ([`5037ee8`](https://github.com/sachncs/promptsheon/commit/5037ee8))
- CI fails the build on gofmt drift
  ([`8740fb4`](https://github.com/sachncs/promptsheon/commit/8740fb4))
- Retry uses `ErrTransient`/`ErrPermanent` typed errors
  ([`2ef26d7`](https://github.com/sachncs/promptsheon/commit/2ef26d7))
- Drop unused loop index in `ParseYAML`
  ([`ad7dc26`](https://github.com/sachncs/promptsheon/commit/ad7dc26))
- `oauthStates` is per-Server, not package-level
  ([`2fcddfa`](https://github.com/sachncs/promptsheon/commit/2fcddfa))
- `gofmt` whitespace cleanup across files touched by fixes
  ([`36925e9`](https://github.com/sachncs/promptsheon/commit/36925e9),
  [`c32402f`](https://github.com/sachncs/promptsheon/commit/c32402f),
  [`80f5bbc`](https://github.com/sachncs/promptsheon/commit/80f5bbc))

### Dependencies

- Bump `modernc.org/sqlite` from 1.52.0 to 1.53.0
  ([`18118fd`](https://github.com/sachncs/promptsheon/commit/18118fd), 2026-06-22)
- Bump `actions/upload-artifact` from 4 to 7
  ([`6b0e034`](https://github.com/sachncs/promptsheon/commit/6b0e034), 2026-06-22)
- Bump `actions/checkout` from 4 to 7
  ([`ecaba2d`](https://github.com/sachncs/promptsheon/commit/ecaba2d), 2026-06-22)

## [0.0.3] - 2026-06-19

Version bump with no documented changes between 0.0.2 and 0.0.3 in git history.
([`aa0884b`](https://github.com/sachncs/promptsheon/commit/aa0884b))

## [0.0.2] - 2026-06-18

Version bump with no documented changes between 0.0.1 and 0.0.2 in git history.
([`3ff607b`](https://github.com/sachncs/promptsheon/commit/3ff607b))

## [0.0.1] - 2026-06-18

Initial release — first tagged build.
([`a1e289f`](https://github.com/sachncs/promptsheon/commit/a1e289f))

[Unreleased]: https://github.com/sachncs/promptsheon/compare/v0.0.5...HEAD
[0.0.5]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.5
[0.0.4]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.4
[0.0.3]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.3
[0.0.2]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.2
[0.0.1]: https://github.com/sachncs/promptsheon/releases/tag/v0.0.1
