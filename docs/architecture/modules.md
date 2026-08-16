# Modules

The `promptsheon/` package tree maps to the layered architecture.
Domain packages declare consumer-defined interfaces; storage
and HTTP packages implement them. The split is enforced by
the `lint-domain` and `lint-deps` Makefile targets — domain
packages must not import from `promptsheon/store` (or anything
that pulls in `package main` at the repo root).

| Package / File | Path | Layer | Purpose |
|---|---|---|---|
| `server` | `promptsheon/server.go` | HTTP | Server composition, options, mux setup. |
| `routes` | `promptsheon/routes.go` | HTTP | Single `RegisterRoutes` entry point. |
| `handlers_*` | `promptsheon/handlers_*.go` | HTTP | One HTTP handler file per domain. |
| `helpers` | `promptsheon/handlers_helpers.go` | HTTP | `validateNonEmpty`, `parsePagination`, `writeJSON`, `HTTPError`. |
| `auth` | `promptsheon/auth/` | Security | API key authentication + permission model. |
| `observation` | `promptsheon/observation/` | Domain | Windowed `ExecutionRecord` aggregator. |
| `optimizer` | `promptsheon/optimizer/` | Domain | Optimizer rules + `CanAutoAdopt`. |
| `rules` | `promptsheon/rules/` | Domain | Recommendation rule primitives. |
| `recommendation` | `promptsheon/recommendation/` | Domain | Producer that consumes Observations and emits Recommendations. |
| `bandit` | `promptsheon/bandit/` | Domain | Thompson Sampling arm selector. |
| `banditstore` | `promptsheon/banditstore.go` | Domain | Persistent arm-posterior store. |
| `experiment` | `promptsheon/experiment.go` | Domain | A/B test scaffolding around `bandit`. |
| `invoke` | `promptsheon/invoke/` | Domain | Canonical entry point for one invocation; Budget + Quota enforcement. |
| `executor` | `promptsheon/executor/` | Domain | Schedule + webhook → Execution record. |
| `../reference/release.md` | `promptsheon/release/` | Domain | Release aggregate + application service. |
| `approval` | `promptsheon/approval/` | Domain | MakerChecker + Majority policies. |
| `capability` | `promptsheon/capability/` | Domain | Workspace, Project, Capability, Version, Manifest value types. |
| `../reference/harness.md` | `promptsheon/harness/` | Domain | Dataset, Precondition, EvalRun types + runner. |
| `../reference/eval.md` | `promptsheon/eval/` | Domain | Scorer registry. |
| `lineage` | `promptsheon/lineage/` | Domain | Decision + lineage persistence. |
| `adoption` | `promptsheon/adoption.go` | Domain | Per-Workspace Recommendation adoption history. |
| `vault` | `promptsheon/vault/` | Domain | AES-256-GCM + KMS-backed KeyProvider. |
| `llm` | `promptsheon/llm/` | Domain | Anthropic + OpenAI provider implementations + `Registry`. |
| `webhook` | `promptsheon/webhook/` | Domain | Event delivery with HMAC signing + SSRF protection. |
| `budget` | `promptsheon/budget/` | Domain | USD-cap enforcement. |
| `quota` | `promptsheon/quota/` | Domain | Rate-cap enforcement. |
| `mcplist` | `promptsheon/mcplist.go` | Domain | Per-Workspace MCP allowlist. |
| `redactor` | `promptsheon/redactor.go` | Plugin | PII redaction default Guardrail. |
| `detector` | `promptsheon/detector.go` | Plugin | Prompt-injection heuristic Guardrail. |
| `guardrail` | `promptsheon/guardrail/` | Plugin | Guardrail interface + runner. |
| `plugins` | `promptsheon/plugins/` | Plugin | In-process + subprocess plugin transport. |
| `supervisor` | `promptsheon/supervisor/` | Plugin | Plugin lifecycle (restart budget, health gate). |
| `replay` | `promptsheon/replay/` | Domain | Replay buffer for hash-stable round-trip reproducibility. |
| `schedule` | `promptsheon/schedule/` | Domain | Schedule aggregate. |
| `scheduler` | `promptsheon/scheduler/` | Domain | The tick loop. |
| `config` | `promptsheon/config.go` | Config | Env-var loader + `Validate`. |
| `metrics` | `promptsheon/metrics/` | Observability | Prometheus collector + HTTP middlewares. |
| `trace` | `promptsheon/trace/` | Observability | OTLP-only trace export. |
| `retention` | `promptsheon/retention.go` | Observability | Audit archive retention sweep. |
| `ratelimit` | `promptsheon/ratelimit/` | HTTP | Per-client partitioned rate limiter. |
| `eventbus` | `promptsheon/eventbus/` | Domain | In-process pub/sub. |
| `policy` | `promptsheon/policy.go` | Domain | Policy decision framework. |
| `alerting` | `promptsheon/alerting/` | Domain | Alert rule + notification groups. |
| `rollups` | `promptsheon/rollups/` | Domain | Per-Workspace Budget/Quota rollup aggregator (in-memory). |
| `settings` | `promptsheon/settings/` | Domain | System config CRDT + resolver. |
| `selfevolve` | `promptsheon/evolve/` | Domain | Closed-loop self-evolution orchestrator. |
| `search` | `promptsheon/search/` | Domain | Catalog search. |
| `election` | `promptsheon/election.go` | Domain | Leader election for HA. |
| `reasoning` | `promptsheon/reasoning.go` | Domain | Reasoning compiler primitives. |
| `llmcontext` | `promptsheon/llmcontext.go` | Domain | LLM context assembly. |
| `usage` | `promptsheon/usage.go` | Observability | `UsageTracker` + Prometheus exposition. |
| `models` | `promptsheon/models/` | Domain | Shared wire types (User, APIKey, ProviderKey, ...). |
| `errs` | `promptsheon/errs/` | Domain | Sentinel errors. |
| `store` | `promptsheon/store/` | Infrastructure | SQLite-backed `Repository` implementation + migrations. |
| `testutil` | `promptsheon/testutil/` | Test | Shared test helpers (logger, sqlite, harness fixture). |
| `testutil/harnessrepo` | `promptsheon/testutil/harnessrepo/` | Test | Shared in-memory `harness.Repository` fixture. |
| `cas` | `promptsheon/cas/` | Domain | Content-addressable store (Merkle DAG). |
| `../development/cli.md` | root `cli.go`, `cli_cas.go`, `cli_harness.go`, `cli_http.go`, `cli_evolve.go` | CLI | Command dispatcher + handlers (in `package main`). |
| `daemon` | root `daemon.go`, `daemon_evolver.go`, `daemon_release_invoker.go`, `embed_frontend.go`, `healthcheck.go` | HTTP | Server entry point + dispatch (in `package main`). |
| `client` (Go SDK) | `pkg/promptsheon` | SDK | Go SDK; see [docs/reference/sdk.md](../reference/sdk.md). |

## Domain-package purity

`make lint-domain` enforces that every domain package
(declared in `scripts/check-no-package-state.go`'s
`domainPackages` list) does not declare package-level
mutable state. Sentinel errors (`Err…`) and import-pin
discards (`var _ = ...`) are explicitly allowed.

`make lint-deps` enforces the broader rule that domain
packages may only depend on other domain packages, the
standard library, and explicitly-allowed third-party
packages (see `.golangci.yml`).

## See also

- [docs/architecture.md](architecture.md) — the system
  diagram and migration timeline.
- [docs/algorithms.md](algorithms.md) — the algorithms
  inside each domain package.