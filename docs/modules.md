# Modules

The internal package tree maps to the layered architecture.
Domain packages declare consumer-defined interfaces; storage
and HTTP packages implement them. The split is enforced by
the `lint-domain` and `lint-deps` Makefile targets — domain
packages must not import from `internal/api`, `internal/store`,
or `cmd/`.

| Package | Path | Layer | Purpose |
|---------|------|-------|---------|
| `api` | `internal/api/` | HTTP | Handlers, middleware, server wiring, request/response shapes. |
| `auth` | `internal/auth/` | Security | API key authentication + permission model. |
| `observation` | `internal/observation/` | Domain | Execution-windowed aggregator. |
| `optimization` (rules + bandit) | `internal/optimizer/`, `internal/bandit/` | Domain | Recommendation engine (deterministic rules + Thompson Sampling). |
| `recommendation` | `internal/recommendation/` | Domain | Producer that consumes Observations and emits Recommendations. |
| `banditstore` | `internal/banditstore/` | Domain | Persistent arm-posterior store. |
| `abtesting` | `internal/experiment/` | Domain | A/B test scaffolding around `bandit`. |
| `invocation` | `internal/invoke/` | Domain | Canonical entry point for one invocation; Budget + Quota enforcement. |
| `execution` | `internal/executor/` | Domain | Schedule + webhook → Execution record. |
| `release` | `internal/release/` | Domain | Release aggregate + application service. |
| `approval` | `internal/approval/` | Domain | MakerChecker + Majority policies. |
| `capability` | `internal/capability/` | Domain | Workspace, Project, Capability, Version, Manifest value types. |
| `harness` | `internal/harness/` | Domain | Dataset, Precondition, EvalRun types + runner. |
| `eval` | `internal/eval/` | Domain | Scorer registry. |
| `lineage` | `internal/lineage/` | Domain | Decision + lineage persistence. |
| `adoption` | `internal/adoption/` | Domain | Per-Workspace Recommendation adoption history. |
| `vault` | `internal/vault/` | Domain | AES-256-GCM + KMS-backed KeyProvider. |
| `llm` | `internal/llm/` | Domain | Anthropic + OpenAI provider implementations. |
| `webhook` | `internal/webhook/` | Domain | Event delivery with HMAC signing + SSRF protection. |
| `rollups` | `internal/rollups/` | Domain | Per-Workspace Budget/Quota rollup aggregator. |
| `rollups/clickhouse` | `internal/rollups/clickhouse/` | Storage | ClickHouse writer (build tag `clickhouse`). |
| `budget` | `internal/budget/` | Domain | USD-cap enforcement. |
| `quota` | `internal/quota/` | Domain | Rate-cap enforcement. |
| `mcplist` | `internal/mcplist/` | Domain | Per-Workspace MCP allowlist. |
| `slo` | `internal/slo/` | Domain | Capability-level SLOs + Repository contract. |
| `slo/evaluator` | `internal/slo/evaluator.go` | Domain | SLO burn-rate evaluator. |
| `redactor` | `internal/redactor/` | Plugin | PII redaction default Guardrail. |
| `injection` | `internal/injection/` | Plugin | Prompt-injection detection default Guardrail. |
| `pluginsup` | `internal/pluginsup/` | Plugin | Plugin supervisor (in-process + subprocess). |
| `subprocess` | `internal/subprocess/` | Plugin | net/rpc-over-UDS subprocess plugin transport. |
| `pluginproto` | `internal/pluginproto/` | Plugin | gRPC over UDS plugin transport (proto + stubs). |
| `pluginmanifest` | `internal/pluginmanifest/` | Plugin | Plugin manifest parser. |
| `plugins/builtins` | `internal/plugins/builtins/` | Plugin | Default in-process plugins. |
| `replay` | `internal/replay/` | Domain | Replay buffer for hash-stable round-trip reproducibility. |
| `schedule` | `internal/schedule/` | Domain | Schedule aggregate. |
| `scheduler` | `internal/scheduler/` | Domain | The tick loop. |
| `config` | `internal/config/` | Config | Env-var loader + Validate. |
| `metrics` | `internal/metrics/` | Observability | Prometheus collector + HTTP middlewares. |
| `trace` | `internal/trace/` | Observability | OTLP-only trace export. |
| `observability` | `internal/observability/` | Observability | OTel tracing, Prometheus metrics, retention sweep. |
| `ws` | `internal/ws/` | HTTP | SSE log stream hub. |
| `ratelimit` | `internal/ratelimit/` | HTTP | Per-client rate limiting. |
| `bridge` | `internal/bridge/` | Adapter | Cross-package adapters. |
| `context` | `internal/context/` | Domain | Context assembly manager. |
| `eventbus` | `internal/eventbus/` | Domain | In-process pub/sub. |
| `policy` | `internal/policy/` | Domain | Policy decision framework. |
| `alerting` | `internal/alerting/` | Domain | Alert rule + notification groups. |
| `redactor` | `internal/redactor/` | Domain | PII redaction default Guardrail. |
| `cli` | `cmd/promptsheon/` | CLI | Hand-rolled command dispatcher. |
| `daemon` | `cmd/promptsheond/` | HTTP | Server binary. |
| `auditbackfill` | `cmd/promptsheon-auditbackfill/` | Tool | One-shot audit replay. |
| `healthcheck` | `cmd/promptsheon-healthcheck/` | Tool | Container health probe. |
| `cas` (public) | `pkg/cas/` | Public | Content-addressable store. Stable public API. |
| `plugin` (public) | `pkg/plugin/` | Public | Stable plugin SDK. |
| `client` (Go SDK) | `sdk/` | SDK | Go SDK; see [docs/sdk.md](sdk.md). |
| `client` (Py SDK) | `sdk/python/src/promptsheon/` | SDK | Python SDK; generated. |
| `client` (TS SDK) | `sdk/typescript/src/` | SDK | TypeScript SDK; generated. |

## Stable public packages

The `pkg/` directory hosts packages with stable APIs intended
to be importable by external Go projects:

- `pkg/cas/` — content-addressable storage.
- `pkg/plugin/` — plugin SDK.

The `sdk/` directory hosts the language SDKs.

## Domain-package purity

`make lint-domain` enforces that every domain package
(declared via the `domain:` Makefile target) does not import
from `internal/api`, `internal/store`, or `cmd/`. This is the
core architectural rule: domain logic does not know about
HTTP, storage, or wiring.

`make lint-deps` enforces the broader rule that domain
packages may only depend on other domain packages, the
standard library, and explicitly-allowed third-party
packages (see `.golangci.yml`).

## See also

- [docs/architecture.md](architecture.md) — the system
  diagram and migration timeline.
- [docs/algorithms.md](algorithms.md) — the algorithms
  inside each domain package.