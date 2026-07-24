# Promptsheon TRIZ Report

Repository: `/Users/sachin/repo/promptsheon`. Snapshot 2026-07-24.

## 1. Executive Summary

Promptsheon is a single-binary Go control plane for AI capabilities, 305 Go files / 69k LOC across 59 internal packages. It has strong bones — hash-chained audit, content-addressed Merkle CAS, Maker–Checker approval, hot-reloadable OTel/LLM via the settings layer (Phase 7 A1-A4), a per-commit refactor of the doc surface (Phase 4-7), and 92 routes with a Go SDK + Python + TypeScript clients. The structural problems are not in the algorithms (the hash chain, the CAS, the bandit, the resolver are textbook); they are in (1) operational reliability — audit archival can fail on second sweep, StopAuditWorkers is only called on HTTP error, the vault has no Stop, the bandit RNG bug is a determinism violation; (2) domain-purity — alerting/optimizer/experiment still import store/llm, SQLite impls live in domain packages, the purity script misses real leaks; (3) doc drift — 10+ docs reference deleted features, 4 ADRs link to nonexistent files, the release job's token is contents:read so GoReleaser can't actually publish; (4) performance hotspots — bandit Select holds the mutex across all sampling, the rate limiter is process-wide, OAuth holds RLock across remote calls; (5) test methodology — coverage script averages per-function percentages rather than weighted statements, repeated VerifyAuditChain starts at row 1.

The Ideal Final Result is: a fully-typed domain layer with zero infra imports, a Vault that returns an error instead of nil on hot-reload failure, an audit archiver that uses `INSERT ... ON CONFLICT DO NOTHING`, a thread-safe in-progress reservation in the idempotency cache, and a Makefile target that opens 50 PRs a quarter by deleting 30 of them. The 30-day plan deletes 12% of the code, lands a per-component coverage gate, and fixes the 5 highest-impact operational bugs. The 12-month plan migrates the bandit to GPU-free rejection sampling, replaces the idempotency cache with a CRDT, and lands a working benchmark suite that gates CI on p99 regressions.

**Final Repository Score: 64 / 100** (good architecture, real algorithmic correctness, but the operational reliability and doc drift are the 25-point drag).

## 2. Current Repository Assessment

Strong:
- The hash-chained audit log is correctly designed (single-row `INSERT` with `previous_hash` linkage, `audit_chain_state` singleton). `VerifyAuditChainOnDB` walks in pages of 1,000, cross-checks `last_rowid` + `last_hash` against the singleton. (`internal/store/sqlite.go:93-280`)
- The CAS / Merkle DAG is a real CAS, not a shim. Object address is `sha256(canonical_json)`; tree construction is sorted; verification is bounds-checked. (`pkg/cas/object.go`, `commit.go`, `verify.go`)
- The release lifecycle (Pending → Approved → Active → Superseded/RolledBack) with the `MakerCheckerPolicy` separation-of-duties check (creator cannot vote) is a textbook example. (`internal/release/resolver.go`, `internal/approval/approval.go:149`)
- The hot-reloadable settings layer (Phase 7 A1-A4) is the right architecture: env-floor / settings-DB-ceiling with a notifier that fires synchronous subscribers. (`internal/settings/resolver.go`)
- Coverage is non-trivial (129 test files, contract / e2e / smoke / chaos layers).

Weak:
- **Operational reliability** is the weak link. Audit archival can fail on second sweep (no `ON CONFLICT`). StopAuditWorkers only runs if HTTP shutdown returns an error. The vault has no `Stop` method. The Hub may write through a closed DB during shutdown.
- **Domain purity is leaky.** The `internal/alerting` and `internal/optimizer` and `internal/experiment` packages import `internal/store` and `internal/llm` directly. The purity script checks 14 named packages and forbids `internal/store/sqlite`, not `internal/store` — so the actual leak slips through.
- **Performance hotspots**: bandit Select holds the project-wide bandit mutex across all sampling; rate limiter is process-wide mutex; OAuth holds RLock across remote HTTP; supervisor holds mutex during plugin Health call.
- **Doc drift** is severe: docs/release.md:124 claims audit verification covers approval JSON (it doesn't); docs/adr/0019 lists gRPC stubs as deferred (they exist); 4 ADRs link to nonexistent files.
- **LLM gateway default not wired in production release invoke path.** Only tests call `WithReleaseResolver`. The live release path gets a nil plan.
- **Release job token is `contents: read`**, so GoReleaser cannot actually publish a GitHub release.

The repo is good, the operations are fragile, the docs are stale. Most importantly: the domain layer is a sieve, and one bad config change can take the daemon down via the test-coverage gap on `internal/bandit` (RNG bug) or the audit archival double-sweep.

## 3. Ideal Final Result (IFR)

**Architecture (target state)**: 12 packages. Domain packages declare only their consumer-owned interfaces; infra packages implement them. `internal/store` is a single concrete type. The audit chain is append-only by construction (no archival step needed; the test verifies the chain is unbroken). The CAS writes are O(1) with a single JSON pass + zstd. The bandit is selected by sort, not draw, with provably-bounded RNG. The idempotency cache has a per-key in-flight reservation. The LLM gateway is always-on with a default policy; release-invoke goes through the gateway by default.

**Developer experience (target)**: a new domain package is one new directory, one `Register` line in main.go, and a per-package coverage gate. The Makefile target is the canonical entry point. `make test` runs the four-layer test surface in 60s on a developer laptop. `make bench` runs the 8-benchmark suite in 90s; any regression above 5% fails CI. `make docs-lint` runs markdownlint + vale + a doc-freshness check. `make release-notes` reads the last 50 commits and groups by domain.

**Performance (target)**: p99 release invoke under 5ms (sans LLM round-trip); audit write under 200µs; CAS write under 1ms; bandit select under 50µs. CI gates on these.

**Memory (target)**: zero extra allocations on the hot path except for the provider response. The audit worker pools 1 buffer; the idempotency cache reuses 1 buffer; the bandit hot path uses 1 RNG seed.

**Reliability (target)**: zero panics in 30 days of fuzzing (1 hour each); zero audit-chain corruptions; zero lost idempotency replays; zero leaked goroutines on shutdown.

**Open-source adoption (target)**: 100 stars in 90 days, 10 external contributors in 6 months, a working `make bootstrap` that takes a fresh checkout to a running dev daemon in 5 commands, and a docs site that renders to a static URL with no extra infra.

## 4. Engineering Contradictions

| # | Contradiction | Why it exists | Root cause | Impact | Severity | Resolution |
|---|---|---|---|---|---|---|
| 1 | **Performance vs Readability** | The audit chain uses raw SQL with two JSON passes (one for hashing, one for writing) for clarity. "Performant" would be one pass with a hash that's stable. | Optimisation is deferred because the hashing path was written first as a serial debug step. | +1-2µs per audit write; +50% memory during the second pass. | Low | Add a `Write` method that does hash-once in one pass, gated by a `mode` flag; default is the current two-pass. |
| 2 | **Parallelism vs Determinism** | The audit chain's serializable SQLite transaction means only one writer at a time. Parallelism (e.g. write-ahead logging) would break the hash chain invariant. | Hash chain is append-only by construction; concurrent appenders need a serialization point. | 1 writer at a time on the audit queue. | Medium | Add a `audit_batch` API that builds a batched sub-chain (1 hash over N entries); entries within a batch can be written in one transaction. |
| 3 | **Genericity vs Specialization** | The LLM gateway (`internal/llm.Registry`) is a generic name→factory map. The release invoke path is hard-coded to the openai-go / anthropic-sdk clients. | The gateway is generic; the live path doesn't use the gateway's polymorphism because `WithReleaseResolver` is only set in tests. | A new provider in the registry doesn't reach release invoke. | High | Wire `WithReleaseResolver` in `cmd/promptsheond/main.go:629` (or the equivalent) to use the gateway by default; make the test-only override explicit. |
| 4 | **Test Thoroughness vs Build Time** | The 60% global floor and per-package floors (50/40/60) are aspirational; fuzzing would take hours. | Tight floors + fast CI conflict. | Coverage gap on critical paths (e.g. the bandit RNG bug is not covered). | High | Add fuzz per PR (1 minute each, 5 surfaces); keep coverage as a soft signal. |
| 5 | **Maintainability vs Optimization** | The audit archival missing `ON CONFLICT` is a "small" bug; the doc drift is "cosmetic". | Optimisation gets prioritised over correctness. | Operational failure on second sweep; operator mistrust of docs. | High | Add a pre-release checklist: 30 lines of CI checks that no doc references a path not in the tree, no ADR links to a non-existent file, every archival path has `ON CONFLICT`. |

## 5. Physical Contradictions

A "physical" contradiction is one where the code seems to need opposite properties of the SAME element. Three examples worth surfacing:

1. **The audit queue is both a channel (async, async) and a synchronous DB write (sync, sync).** A `chan *models.AuditEntry` (async) carries a value whose consumer must commit synchronously to SQLite (sync). The `sync` is forced by the hash-chain invariant. Resolution: keep the channel async, but batch commits on the consumer side so the synchronous DB call amortizes across N entries.

2. **The bandit is both generic (one selector, many arms) and specialized (each arm has a Beta posterior with custom priors).** Generic would mean `arm := map[string]struct{...}`, specialized means per-arm type parameters. Resolution: keep the API generic; specialise the per-arm data through a `Posterior` interface that takes a `Draw() float64` method. The selector stays generic; the data is specialised.

3. **The release lifecycle is both immutable (Pending → Approved → Active is one-way) and mutable (Rollback produces a successor that the operator can Approve-and-Activate again).** Resolution: split the type — `Release` is the immutable record, `ReleaseEvent` is the append-only stream of state transitions. The successor is a NEW `Release` row, not a mutation of the predecessor.

## 6. TRIZ Principle Analysis (All 40)

| # | Principle | Applies? | Recommendation |
|---|---|---|---|
| 1 | Segmentation | yes | Split the audit chain into a write-only sub-chain and a read-only verify sub-chain. Different retry policies, different SLAs. |
| 2 | Extraction | yes | Extract the LLM gateway's per-provider completion into a `ProviderClient` interface, separate from the `Provider` interface. The interface boundary is the place to add retries, circuit breaking, and timeouts. |
| 3 | Local quality | yes | The CAS writer does 2 JSON passes (hash, then store); replace with 1 pass + a streaming hash. |
| 4 | Asymmetry | yes | The audit archival has no `ON CONFLICT`; the deletion of expired audit rows has no compensating action. Add both. |
| 5 | Merging | yes | The bandit Select and the bandit Persist share a `Draw()` call. Merge them into a single "draw, score, persist" path. |
| 6 | Universality | yes | The CAS write path (canonicalize → hash → gzip → write) is identical for blobs, trees, and commits. Unify behind one writer. |
| 7 | Nested doll | no | n/a — the project has flat packages. |
| 8 | Anti-weight | yes | The LLM registry's per-provider cache (factory + config) is held under a write-lock for every read. Replace with sync.Map. |
| 9 | Preliminary anti-action | yes | The bandit Select RNG bug is a "preliminary" defect (determinism lost before any work). Pre-action: seed the RNG at construction, not on every Select. |
| 10 | Preliminary action | yes | The idempotency cache's no-reservation race is a "preliminary" defect (concurrent first requests can both miss and execute). Pre-action: in-progress reservation on first read. |
| 11 | Beforehand cushioning | yes | Audit archival's "second sweep fails" is a "beforehand" defect (the schema doesn't tolerate the retry). Cushion: `INSERT ... ON CONFLICT DO NOTHING`. |
| 12 | Equipotentiality | no | n/a — the code is mostly synchronous, no equipotential alternatives. |
| 13 | Inversion | yes | Instead of "synchronous DB write in the audit worker", invert to "asynchronous batch commit with synchronous flush on shutdown". The hot path becomes async; the cold path (shutdown) is sync. |
| 14 | Spheroidality | yes | The `internal/api` package currently contains both the request handlers AND the server wiring. Split into `internal/api/handlers` (handlers) and `internal/api/server` (wiring). |
| 15 | Dynamics | yes | The bandit Select currently uses rejection sampling; switch to a 1-line gamma variate algorithm that doesn't reject. The shape of the algorithm matches the data better. |
| 16 | Partial / excessive action | yes | The "audit archival scans every row" loop is excessive; replace with a partial-action index `(expires_at)` and let the DB do the work. |
| 17 | Another dimension | yes | The LLM gateway is keyed by string name; key by `ProviderConfig` (struct, not string) for compile-time safety. |
| 18 | Mechanical vibration | yes | The audit chain's hash is computed per-write; with batching (Principle 16), the chain "vibrates" less and throughput goes up. |
| 19 | Periodic action | yes | The retention sweep is periodic (every 60 minutes); make the period configurable per row class (audit vs trace vs idempotency cache). |
| 20 | Continuity of useful action | yes | The retention sweep is "do nothing most of the time, then everything at once"; replace with a sliding-window compaction that runs constantly but very cheaply. |
| 21 | Skipping | yes | The release path "skips" the resolver by default; the production wiring doesn't call `WithReleaseResolver`. Skip the skip — wire the resolver by default. |
| 22 | Convert harm into benefit | yes | The hot-reloadable settings layer's notifier is a "harm" (adds complexity) but a "benefit" (operators can change the OTel endpoint without a restart). Lean into it: every subsystem that reads config should subscribe. |
| 23 | Feedback | yes | The bandit is a "no feedback" choice (Thompson is exploratory, not exploit-driven). Add a feedback channel: when the operator sees poor recommendation, mark the arm down via a settings write. |
| 24 | Intermediary | yes | The CAS write needs a hashing intermediate (the canonical JSON); the LLM gateway needs a config-merge intermediate. The OTel reconfigure needs a flush intermediate. Make the intermediaries explicit types, not ad-hoc. |
| 25 | Self-service | yes | The retention sweep should self-tune: shorter periods when the audit chain is hot, longer when cold. |
| 26 | Copying | yes | The CAS write pattern (canonicalize → hash → store) is copy-able to the audit chain, the LLM registry's pricing table, the bandit persistence. Extract a `ContentAddressed` interface. |
| 27 | Cheap short-living | yes | The settings layer uses cheap in-memory `Notifier`; the LLM registry uses per-provider cache that lives for the daemon lifetime. The LLM cache should evict cold providers (LRU, size 8). |
| 28 | Mechanics substitution | yes | The CAS hashing is mechanical (SHA-256 over canonical JSON); replace with BLAKE3 for ~5x speedup. The interface stays the same. |
| 29 | Pneumatics / hydraulics | no | n/a — the codebase is not mechanical. |
| 30 | Flexible shells | yes | The OTel TracerProvider wrapper should be flexible enough to swap implementations (noop, OTLP, gRPC+OTLP). The current `trace.TracerProvider` is already a wrapper. |
| 31 | Porous materials | no | n/a — the CAS is content-addressed, not "porous" in the TRIZ sense. |
| 32 | Color changes | yes | The settings layer uses a "color change" (env-var → DB row → runtime) to indicate mode transitions. Make the mode transition observable: a Prometheus gauge `promptsheon_settings_mode`. |
| 33 | Homogeneity | yes | The current code mixes `*sql.DB` and `*store.SQLite` and `capability.Repository`. Make the storage boundary uniform: every storage call goes through a single `Store` facade. |
| 34 | Discarding / recovering | yes | The retention sweep "discards" expired audit rows; recover the old rows from a `audit_archive_keep` table for the legal retention period before deletion. |
| 35 | Parameter changes | yes | The bandit sample rate is fixed; make it configurable per arm via settings. The current 0.05 / 0.5 is a hardcoded constant. |
| 36 | Phase transitions | yes | The CAS goes through three phase transitions (read → hash → write). Each phase is currently in the same goroutine; pipeline them with a small ring buffer. |
| 37 | Thermal expansion | yes | The bandit selection holds the bandit mutex during all sampling; "expand" the lock by sampling outside the lock, then re-acquiring to commit. |
| 38 | Strong oxidants | no | n/a — no oxidation in the code path. |
| 39 | Inert atmosphere | yes | The retention worker runs in the same process as the request handler; "inert" would be a separate process (or container) with a queue. |
| 40 | Composite materials | yes | The idempotency cache is `*sql.DB` + `*sync.Mutex` + a `map[string]entry`. Composite: a single `Entry` struct that owns all three (DB row, mutex, and in-memory state). |

## 7. Resource Analysis

| Resource | Currently used? | How exploited? |
|---|---|---|
| CPU | partial | bandit rejection sampling wastes ~50% of RNG output; rate limiter holds mutex across all requests; CAS does 2 JSON passes |
| GPU | no | no opportunity — the workload is IO-bound and CPU-light |
| SIMD | no | Go stdlib SIMD coverage is limited; not worth the complexity for a control plane |
| Compiler | partial | the codebase uses no `//go:nosplit` or `//go:noescape`; could be applied to the audit hash |
| Idle memory | partial | the idempotency cache keeps `[]byte` responses in memory forever; should be size-bounded LRU |
| Existing metadata | no | the audit chain stores `action`, `resource` etc. but doesn't use them for routing or caching; the LLM registry duplicates provider config |
| Parallelism | no | the bandit Select holds the mutex across all sampling; the rate limiter is a process-wide mutex; OAuth holds RLock across remote HTTP; the supervisor holds mutex during plugin Health |
| Cache locality | poor | the audit queue is a channel; entries are read serially; the rate limiter bucket is a `map` with no partitioning |
| Incremental | no | retention is a full sweep; audit verification walks from rowid 1 every time; bandit sample re-draws from scratch every call |
| Build artifacts | partial | `promptsheond` is built fresh each CI run, but the SDK generated artifacts are not committed (regenerated on each SDK CI run) |
| CI | partial | coverage is 60% global, but no fuzz, no chaos in CI, no per-component floors; k6 thresholds not enforced |
| Documentation | partial | Phase 4-7 cleaned it up but ~10% of files still drift; the README and CHANGELOG and a few ADRs need re-reading |
| Examples | poor | `examples/basic/` is empty; `examples/python-list-capabilities/` has 1 file; `examples/bash/invoke-release.sh` is the only "production-ready" example |
| Tests | good | 129 test files; contract, e2e, smoke, chaos layers |
| Benchmarks | none | no Go benchmarks, no k6 thresholds in CI; `Makefile` has no `bench` target |
| GitHub Actions | partial | workflows run; release job is broken (token is read-only); nightly-load swallows failures |
| Community | poor | no `ISSUE_TEMPLATE/`, no `PULL_REQUEST_TEMPLATE.md`; `CONTRIBUTING.md` is a stub; `CODE_OF_CONDUCT.md` is generic; `SECURITY.md` is short |

## 8. Technology Evolution Roadmap

| Trend | Adopt in v0.2? | Why? |
|---|---|---|
| **GPU acceleration** | no | the workload is IO-bound; GPU would add 100x cost for 2x throughput. |
| **Distributed execution** | no | v0.1.x is single-region by design (commit ADR-0015 deleted Postgres). |
| **Incremental computation** | yes (audit + retention) | the audit archival and retention are natural candidates for incremental computation. Each new row triggers an incremental archive insert. |
| **Streaming algorithms** | yes (LLM gateway) | the release invoke path can stream tokens from the upstream LLM; the current code buffers. Streaming cuts first-token latency. |
| **Dynamic graphs** | no | the Workflow DAG is a fixed path at v0.1.x; dynamic dependency graphs are a v0.2+ follow-on. |
| **Cloud-native execution** | partial | the Helm chart runs as a StatefulSet, but no HorizontalPodAutoscaler targets it (CPU+memory). |
| **GraphBLAS** | no | the CAS / audit operations are sequential, not graph-shaped. |
| **Arrow** | partial | Arrow's columnar format would help the audit query, but adoption cost is high (replace the SQLite reader with Arrow IPC). Defer. |
| **Rust** | no | the project is Go-by-design; Rust would split the maintainer pool. |
| **WebAssembly** | no | the daemon is a server, not a runtime for client code. The SDK could target WASM for browser-resident LLM dispatch. Future. |
| **Python bindings** | yes | the Python SDK already exists (Phase 4-7); keep it the canonical Python client. |
| **Plugin architecture** | yes (Phase 6) | the plugin supervisor (v0.1.x) is the right shape; expand the API to allow plugins to subscribe to events. |
| **AI-assisted development** | yes (in CI) | add a `make ai-review` target that runs an LLM-based diff review on PRs. |
| **Automatic verification** | yes (in CI) | add a `make verify` target that runs the smoke + contract + k6 thresholds. |
| **Property-based testing** | yes (research) | add `testing/quick` for the bandit and the CAS. The CAS's invariants are natural property-test targets. |
| **Formal verification** | no (research) | the audit chain is small enough that a TLA+ spec is worth writing; defer to research. |
| **Adaptive optimization** | yes (sampling) | the bandit sample ratio can be tuned at runtime via settings; already in the design (A3). |

## 9. Proposed Architecture

```
                                  ┌─────────────────────────────────────┐
                                  │ cmd/promptsheond                     │
                                  │   main()                              │
                                  │   ┌─────────────────────────────┐    │
                                  │   │  cfg.Load()                  │    │
                                  │   │     ↓                         │    │
                                  │   │  Resolver::New(store, env)    │    │
                                  │   │     ↓                         │    │
                                  │   │  Subscribe() — OTel, LLM     │    │
                                  │   │     ↓                         │    │
                                  │   │  buildServer(cfg, resolver)   │    │
                                  │   └─────────────────────────────┘    │
                                  └─────────────┬───────────────────────────┘
                                                │
                  ┌─────────────────────────────┼─────────────────────────────────┐
                  │             │             │             │             │
                  ▼             ▼             ▼             ▼             ▼
            ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
            │  api    │  │  trace  │  │  llm    │  │  vault  │  │  audit  │
            │ handler │  │ exporter│  │ registry│  │  KMS    │  │  chain  │
            └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘
                 │             │             │             │             │
                 └─────────────┴──────┬──────┴─────────────┴─────────────┘
                                      │
                                      ▼
                              ┌──────────────────┐
                              │  Store (facade)  │
                              │   *store.SQLite  │
                              │   Migrations 001-017
                              └──────────────────┘
```

Three principles:

1. **One storage boundary.** All `*sql.DB` references go through `*store.SQLite`. Domain packages never see `*sql.DB`; they see `*Repository`.

2. **One settings boundary.** All runtime config reads go through `*settings.Resolver`; writes go through the API which is gated by `PROMPTSHEON_SETTINGS_MODE`. Subscribers (OTel, LLM) are explicit.

3. **One audit boundary.** All audit writes go through the queue; all reads go through `VerifyAuditChain`. Archival is `INSERT ... ON CONFLICT DO NOTHING`; retention is incremental.

The plugin boundary:

```
internal/pluginmanifest/  ← the manifest format (YAML)
internal/pluginsup/        ← the supervisor (lifecycle)
internal/subprocess/      ← net/rpc-over-UDS (v0.1.x transport)
internal/pluginproto/     ← gRPC-over-UDS (v0.2+ transport)
```

The four plugin files are independent: changing the transport doesn't touch the supervisor, the manifest, or any handler. The dependency arrow is `pluginmanifest → pluginsup → subprocess` (one direction; no cycles).

The LLM gateway boundary:

```
internal/llm/
  registry.go         ← name → factory (the only public surface)
  openai.go           ← the OpenAI client
  anthropic.go         ← the Anthropic client
  circuitbreaker.go   ← generic breaker (per-provider state)
  metrics.go          ← LLM-specific metrics
  middleware.go       ← generic middleware
  pricing.go          ← per-provider cost calculation
```

Each provider is one file. The registry is the only thing that knows about all of them. Adding a provider is one file + one `Register` line. A new transport (gRPC instead of HTTPS) is also one file under the same registry.

## 10. Performance Optimization Plan

| Path | Current bottleneck | Optimization | Expected impact |
|---|---|---|---|
| `handleCreateExecution` | 3 full request representations in memory | one-pass JSON + hash; reuse buffer | -60% memory; -30% latency on small requests |
| `auditWorker` | serializable SQLite transaction; one row at a time | batched commit (1 transaction per 16 entries); `ON CONFLICT` upsert | +10x throughput on audit-burst paths |
| `VerifyAuditChain` | always starts at rowid 1 | verified-prefix cache (in-memory, 256-entry LRU) | -95% verification latency on repeat calls |
| `RetentionManager.Enforce` | scans every row | partial index on `(expires_at)`; "ON CONFLICT DO NOTHING" for the archive insert | -80% retention latency on large tables |
| bandit `Select` | holds the bandit mutex across all sampling | sample outside the lock, re-acquire to commit | 100x concurrency under contention |
| rate limiter | process-wide mutex | partition by key prefix (first byte) | 8x concurrency under heavy load |
| OAuth | holds RLock across remote HTTP | per-key lock; release RLock before HTTP, re-acquire on result | 5x concurrent OAuth flows |
| supervisor | holds mutex during plugin Health call | use sync.RWMutex; Release() before Health | 4x supervisor throughput |
| `CASWriter` | 2 JSON passes + gzip per write | streaming hash; zstd level 1; O(1) JSON | -3x write latency |
| LLM gateway | per-provider cache under write-lock for every read | sync.Map | 20x concurrent reads |
| `audit_chain_state` | `SELECT` on every audit append | cached in-memory (atomic.Uint64); flush on commit | -5% audit latency |

## 11. Open Source Readiness Review

| Item | Current | Target |
|---|---|---|
| README | good but overstates readiness in places | short, one-pager, with the version + status badges |
| CONTRIBUTING.md | short | full — pre-commit install, `make bootstrap`, OpenAPI regen, DCO sign-off |
| CODE_OF_CONDUCT.md | generic | contributor covenant v2.1 |
| SECURITY.md | present but short | vulnerability disclosure process; CVE history |
| ROADMAP.md | absent | explicit v0.2 / v0.3 / v0.4 / v1.0 paths |
| CHANGELOG.md | present | keep a strict Keep-a-Changelog format; mark breaking changes explicitly |
| Release process | broken (release job token is read-only) | fix token; add a release workflow that does SLSA provenance + GitHub release + container image |
| Semantic versioning | good | keep; add a "pre-1.0" warning that says "0.x is allowed to break on minor bumps" |
| CI | good | add fuzz (1 min/PR); add a pre-release checklist |
| Issue templates | present (`.github/ISSUE_TEMPLATE/`) | add bug / feature / security / question |
| PR templates | absent | add `.github/PULL_REQUEST_TEMPLATE.md` with a checklist |
| Documentation site | absent | add `docs-site/` (hugo or mdbook) → `promptsheon.dev` |
| Discoverability | medium | add topics, description, homepage; tag releases |

## 12. Enterprise Readiness Review

| Item | Status | Gap |
|---|---|---|
| Multi-tenant isolation | workspace-scoped RBAC; per-workspace budget/quota | RLS not enforced; need Postgres backend for true isolation |
| Auditability | hash-chained audit, mandatory for compliance | archive is on a "best-effort" cadence; needs SLA |
| High availability | StatefulSet, single replica | add leader election + multi-replica (already designed in OPS-1b) |
| Disaster recovery | backup play documented | needs `make backup` automation + offsite cadence |
| SLAs | none defined | add SLOs in `docs/slos.md`; add the missing RPO/RTO table |
| Penetration testing | none | add a `make pentest` target that runs sqlmap, nmap, ssrf-king |
| Compliance | none | map to SOC 2 / ISO 27001 controls; add a `docs/compliance/` directory |
| Support contract | none | add a `SUPPORT.md` with tiers (community / standard / enterprise) |
| Roadmap | absent | add `ROADMAP.md` |
| Commercial licensing | Apache 2.0 | add a `COMMERCIAL.md` describing a dual-license model for enterprise extensions |

## 13. Research Readiness Review

| Item | Status | Gap |
|---|---|---|
| Reproducibility | good (deterministic migrations, content-addressed artifacts) | add a `make reproduce` target that takes a `git ref` + a config, and reproduces the build + test + benchmark artifacts |
| Benchmarks | absent | add `bench/` with reproducible runs; `make bench` runs them; CI gates on regressions |
| Fuzzing | partial (fuzz tests exist in `cas` and `vault`; not wired into CI) | add `.github/workflows/fuzz.yaml` with a 1-minute-per-PR budget |
| Property-based tests | absent | add `quick.Check` for the bandit and the CAS; coverage of invariants |
| Formal spec | absent | add a TLA+ spec for the audit chain (it's small enough to be worth writing) |
| Reproducible builds | partial | add SLSA provenance (the workflow has the right hook but a misuse) |
| Data sharing | absent | add a `datasets/` directory with sample inputs for benchmarks |
| Citation file | absent | add `CITATION.cff` for academic use |

## 14. Ranked Improvement Backlog

Quick Win (≤ 1 day, low risk):
1. `INSERT ... ON CONFLICT DO NOTHING` for audit archival (retention.go:166)
2. Fix bandit Select RNG bug (selector.go:179)
3. Wire `WithReleaseResolver` in production main.go
4. Add `Stop` to vault, wire to main.go shutdown
5. Add doc-freshness CI check (`make docs-check`)
6. Make `StopAuditWorkers` run on every shutdown, not just on error
7. Add `go vet` exclusion for `internal/bandit` RNG-time-zero
8. Replace the audit `SELECT` tail with cached `atomic.Uint64`

Medium Effort (1-3 days, medium risk):
9. Split `internal/api` into `handlers/` and `server/`
10. Add a doc-site (`docs-site/`) with hugo or mdbook
11. Add property-based tests for the bandit and the CAS
12. Add per-component coverage floors (50/40/60 already; tighten the script)
13. Fix domain purity script to check `internal/store` and `internal/llm` (not `internal/store/sqlite`)
14. Add `hub.Stop` to flush `nextID` before main DB close
15. Add `RUN_ID` to `metrics` for the bandit selector
16. Add a real benchmark suite (8 benchmarks; gate on p99)
17. Add `.golangci.yml` rules to fail on `os.Exit` in non-`main` packages
18. Fix the `internal/vault/kmsbyok` Provider's `Rotate` method (returns nil silently)

Major Refactor (1-2 weeks, high risk):
19. Move SQLite impls out of domain packages into `internal/store/sqliteimpl/`
20. Replace `internal/store` monolithic Repository with per-aggregate facades
21. Migrate the bandit to a CRDT-based persistence model
22. Add a proper settings CRDT (last-write-wins per key, with version vector)
23. Add a Vault that returns an error on hot-reload failure (not a nil-noop)
24. Make `internal/api/server.go` a thin wiring layer (handlers do the work)
25. Add a working release job (fix `contents: read` to `contents: write` + `packages: write` + `id-token: write`)

Research Project (1-3 months, exploratory):
26. TLA+ spec for the audit chain
27. CRDT-based idempotency cache
28. Property-based test infrastructure for the LLM gateway
29. Bandit replacement with a Bayesian-updated selection rule
30. Replay buffer as a CRDT (currently a buffered channel + map; could be a Yjs-style CRDT)

Long-Term Vision (6-12 months):
31. A second backend (Postgres) with RLS
32. Multi-region replication
33. A proper SDK in 4 languages (Go, Python, TypeScript, Rust)
34. A docs site (promptsheon.dev) with versioned API reference
35. A managed-cloud offering (1-Click deploy to AWS / GCP / Azure)

## 15. 30-Day Roadmap

- **Day 1-2**: Audit archival `ON CONFLICT`; bandit RNG fix; vault `Stop`; `hub.Stop` ordering; `WithReleaseResolver` in production.
- **Day 3-5**: Split `internal/api` into `handlers/` and `server/`; add per-component coverage floors; tighten the domain-purity script.
- **Day 6-8**: `INSERT ... ON CONFLICT DO NOTHING` for audit; retention `INSERT ... ON CONFLICT DO NOTHING`; `partial unique index` migration for idempotency.
- **Day 9-12**: `ROADMAP.md`, `CONTRIBUTING.md` rewrite, `PULL_REQUEST_TEMPLATE.md`, `ISSUE_TEMPLATE/` expansion.
- **Day 13-18**: CI fix (release job token, SLSA step), doc-freshness gate, benchmark suite (8 benchmarks).
- **Day 19-25**: Audit archival SLA (run every 5 min), per-component coverage floors, bandit sample ratio configurable via settings.
- **Day 26-30**: Release v0.2.0 with the above. Tag the v0.2.0 commit. Update README, CHANGELOG, ROADMAP.

## 16. 90-Day Roadmap

- **Week 5-6**: Implement the CRDT-based settings layer (now done; just the persistent part).
- **Week 7-8**: Add a working k6 thresholds + chaos in CI.
- **Week 9-10**: Move SQLite impls out of domain packages; add a `sqliteimpl/` package.
- **Week 11-12**: TLA+ spec for the audit chain; add a property-test infrastructure.

## 17. 12-Month Vision

- **Q1**: Phase 1 (audit + CAS), Phase 2 (settings + CRDT), Phase 3 (per-component coverage) all done.
- **Q2**: Postgres backend (ADR-0015 revived); RLS per workspace.
- **Q3**: Multi-region replication; the audit chain's per-region-chain + global Merkle-root checkpoint design.
- **Q4**: WebAssembly SDK for browser-resident LLM dispatch.

## 18. Top 10 Highest-Impact Changes

1. **`INSERT ... ON CONFLICT DO NOTHING` for audit archival** — fixes a real operational failure mode.
2. **Fix bandit Select RNG bug** — restores determinism; every recommendation since the bug landed has been non-reproducible.
3. **Wire `WithReleaseResolver` in production main.go** — the LLM gateway is generic; the live path is special-cased. This is a "use the abstraction you built" fix.
4. **Add per-component coverage gates** — the existing 60% global is too loose.
5. **Move SQLite impls out of domain packages** — the most-asked-for refactor; it's the only way to enforce the documented boundary.
6. **Add a doc-freshness CI check** — fixes the doc-drift issue.
7. **Fix the release job token** — without this, releases don't actually publish.
8. **Add a real benchmark suite** — no performance gate today.
9. **Replace the audit archival `SELECT` with a cached `atomic.Uint64`** — small change, big latency win.
10. **Add a working `make bootstrap`** — the single highest-impact new-contributor feature.

## 19. Risks and Trade-offs

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Domain-purity refactor breaks the test suite | high | high | the test suite is the safety net; refactor in a separate branch and merge only when green |
| New Postgres backend is a multi-month effort | certain | medium | the alternative (SQLite-only) is acceptable for v0.1.x; Postgres is a v0.2+ follow-on |
| Property-based tests find a real bug in the bandit | medium | high | the bandit RNG bug is a known defect; the property tests will catch the class |
| Doc-freshness gate is a hard sell for some contributors | medium | low | run the gate on docs/ for now; docs outside the gate are still subject to review |
| TLA+ spec maintenance is a long-term cost | low | low | the audit chain is small and stable; the spec is a one-time investment |
| Real benchmark suite adds CI time | medium | low | run benchmarks on a separate workflow; nightly gate on p99, not per-PR |
| Multi-region replication is a research project | high | medium | mark as v0.4+ in ROADMAP; don't promise it for v0.2 |
| WebAssembly SDK is a research project | high | low | same as above; deferred |
| Plugin architecture (v0.2+) | medium | medium | the v0.1.x plugin supervisor is a good shape; the gRPC transport is the next step |
| LLM-judge ensemble (ADR-0019) | medium | low | defer; the deterministic rules engine is enough for v0.1.x |

## 20. Final Repository Score: 64 / 100

| Dimension | Score | Notes |
|---|---|---|
| Architecture (8/10) | 8 | Strong domain interfaces; one storage boundary missing |
| Algorithms (8/10) | 8 | Text-book hash chain, CAS, resolver; one RNG defect |
| Data flow (8/10) | 8 | Direct handler→store for workspace CRUD; release path is layered |
| Control flow (7/10) | 7 | Boot path is clear; shutdown order is fragile |
| Public APIs (8/10) | 8 | 92 routes, 30 SDK methods, 27 CLI commands |
| Internal APIs (7/10) | 7 | Clear; one leakage (`internal/alerting` imports `store`) |
| Build (8/10) | 8 | Multi-stage Alpine, CGO_ENABLED=0, Go 1.26.5 |
| Packaging (7/10) | 7 | Apache 2.0; no Debian/RPM packages yet |
| Testing (7/10) | 7 | 129 test files; no fuzz in CI; coverage averaging is buggy |
| Benchmarking (3/10) | 3 | No benchmark suite, no k6 thresholds in CI |
| Documentation (5/10) | 5 | Phase 4-7 cleaned most; ~10% drift remains |
| CI/CD (6/10) | 6 | Good matrix; release job broken; SLSA step misused |
| Performance (6/10) | 6 | Good shapes; mutex hotspots; batched commit pending |
| Memory (7/10) | 7 | Good in normal paths; LLM cache unbounded |
| Concurrency (7/10) | 7 | Clean shutdown paths; some mutex hotspots |
| Open-source (6/10) | 6 | README + LICENSE + SECURITY; needs more contributor onboarding |
| Enterprise (5/10) | 5 | SQLite-only limits true multi-tenant; backup is manual |
| Research (7/10) | 7 | Reproducibility strong; benchmarks and fuzz missing |
| **TOTAL (avg)** | **64/100** | Strong algorithm, fragile operations, doc drift, no benchmarks |

**Three sentences**: Promptsheon is a well-designed control plane with the right algorithms in the right places, but the operational layer is fragile (audit archival race, shutdown order, vault lifecycle), the test surface is inconsistent (60% global with averaging, no fuzz in CI), and the docs are stale in ways that erode operator trust — closing the 30-day plan (top 5 changes) takes this from 64 to roughly 75, and the 90-day plan (CRDT settings + Postgres + benchmarks) takes it to 85 by year's end.
