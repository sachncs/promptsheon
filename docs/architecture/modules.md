# Modules

The `backend/` package tree maps to the layered architecture.
Domain packages declare consumer-defined interfaces; storage
and HTTP packages implement them. The split is enforced by
the `lint-domain` and `lint-deps` Makefile targets — domain
packages must not import from `backend/store` (or anything
that pulls in `package main` at the repo root).

| Package / File | Path | Layer | Purpose |
|---|---|---|---|
| `server` | `backend/server.go` | HTTP | Server composition, options, mux setup. |
| `routes` | `backend/routes.go` | HTTP | Single `RegisterRoutes` entry point. |
| `handlers_*` | `backend/handlers_*.go` | HTTP | One HTTP handler file per domain. |
| `helpers` | `backend/handlers_helpers.go` | HTTP | `validateNonEmpty`, `parsePagination`, `writeJSON`, `HTTPError`. |
| `auth` | `backend/auth/` | Security | API key authentication + permission model. |
| `observation` | `backend/observation/` | Domain | Windowed `ExecutionRecord` aggregator. |
| `optimizer` | `backend/optimizer/` | Domain | Optimizer rules + `CanAutoAdopt`. |
| `rules` | `backend/rules/` | Domain | Recommendation rule primitives. |
| `recommendation` | `backend/recommendation/` | Domain | Producer that consumes Observations and emits Recommendations. |
| `bandit` | `backend/bandit/` | Domain | Thompson Sampling arm selector. |
| `banditstore` | `backend/banditstore.go` | Domain | Persistent arm-posterior store. |
| `experiment` | `backend/experiment.go` | Domain | A/B test scaffolding around `bandit`. |
| `invoke` | `backend/invoke/` | Domain | Canonical entry point for one invocation; Budget + Quota enforcement. |
| `executor` | `backend/executor/` | Domain | Schedule + webhook → Execution record. |
| `../reference/release.md` | `backend/release/` | Domain | Release aggregate + application service. |
| `approval` | `backend/approval/` | Domain | MakerChecker + Majority policies. |
| `capability` | `backend/capability/` | Domain | Workspace, Project, Capability, Version, Manifest value types. |
| `../reference/harness.md` | `backend/harness/` | Domain | Dataset, Precondition, EvalRun types + runner. |
| `../reference/eval.md` | `backend/eval/` | Domain | Scorer registry. |
| `lineage` | `backend/lineage/` | Domain | Decision + lineage persistence. |
| `adoption` | `backend/adoption.go` | Domain | Per-Workspace Recommendation adoption history. |
| `vault` | `backend/vault/` | Domain | AES-256-GCM + KMS-backed KeyProvider. |
| `llm` | `backend/llm/` | Domain | Anthropic + OpenAI provider implementations + `Registry`. |
| `webhook` | `backend/webhook/` | Domain | Event delivery with HMAC signing + SSRF protection. |
| `budget` | `backend/budget/` | Domain | USD-cap enforcement. |
| `quota` | `backend/quota/` | Domain | Rate-cap enforcement. |
| `mcplist` | `backend/mcplist.go` | Domain | Per-Workspace MCP allowlist. |
| `redactor` | `backend/redactor.go` | Plugin | PII redaction default Guardrail. |
| `detector` | `backend/detector.go` | Plugin | Prompt-injection heuristic Guardrail. |
| `guardrail` | `backend/guardrail/` | Plugin | Guardrail interface + runner. |
| `plugins` | `backend/plugins/` | Plugin | In-process + subprocess plugin transport. |
| `supervisor` | `backend/supervisor/` | Plugin | Plugin lifecycle (restart budget, health gate). |
| `replay` | `backend/replay/` | Domain | Replay buffer for hash-stable round-trip reproducibility. |
| `schedule` | `backend/schedule/` | Domain | Schedule aggregate. |
| `scheduler` | `backend/scheduler/` | Domain | The tick loop. |
| `config` | `backend/config.go` | Config | Env-var loader + `Validate`. |
| `metrics` | `backend/metrics/` | Observability | Prometheus collector + HTTP middlewares. |
| `trace` | `backend/trace/` | Observability | OTLP-only trace export. |
| `retention` | `backend/retention.go` | Observability | Audit archive retention sweep. |
| `ratelimit` | `backend/ratelimit/` | HTTP | Per-client partitioned rate limiter. |
| `eventbus` | `backend/eventbus/` | Domain | In-process pub/sub. |
| `policy` | `backend/policy.go` | Domain | Policy decision framework. |
| `alerting` | `backend/alerting/` | Domain | Alert rule + notification groups. |
| `rollups` | `backend/rollups/` | Domain | Per-Workspace Budget/Quota rollup aggregator (in-memory). |
| `settings` | `backend/settings/` | Domain | System config CRDT + resolver. |
| `selfevolve` | `backend/selfevolve/` | Domain | Closed-loop self-evolution orchestrator. |
| `search` | `backend/search/` | Domain | Catalog search. |
| `election` | `backend/election.go` | Domain | Leader election for HA. |
| `reasoning` | `backend/reasoning.go` | Domain | Reasoning compiler primitives. |
| `llmcontext` | `backend/llmcontext.go` | Domain | LLM context assembly. |
| `usage` | `backend/usage.go` | Observability | `UsageTracker` + Prometheus exposition. |
| `models` | `backend/models/` | Domain | Shared wire types (User, APIKey, ProviderKey, ...). |
| `errs` | `backend/errs/` | Domain | Sentinel errors. |
| `store` | `backend/store/` | Infrastructure | SQLite-backed `Repository` implementation + migrations. |
| `testutil` | `backend/testutil/` | Test | Shared test helpers (logger, sqlite, harness fixture). |
| `testutil/harnessrepo` | `backend/testutil/harnessrepo/` | Test | Shared in-memory `harness.Repository` fixture. |
| `cas` | `backend/cas/` | Domain | Content-addressable store (Merkle DAG). |
| `../development/cli.md` | root `cli.go`, `cli_cas.go`, `cli_harness.go`, `cli_http.go`, `cli_selfevolve.go` | CLI | Command dispatcher + handlers (in `package main`). |
| `daemon` | root `daemon.go`, `daemon_evolver.go`, `daemon_release_invoker.go`, `embed_frontend.go`, `healthcheck.go` | HTTP | Server entry point + dispatch (in `package main`). |
| `client` (Go SDK) | `sdk/` | SDK | Go SDK; see [docs/sdk.md](../reference/sdk.md). |
| `client` (Py SDK) | `sdk/python/src/promptsheon/` | SDK | Python SDK; generated from `backend/spec/spec.yaml`. |
| `client` (TS SDK) | `sdk/typescript/src/` | SDK | TypeScript SDK; generated from `backend/spec/spec.yaml`. |

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