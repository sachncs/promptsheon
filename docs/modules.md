# Modules

One-line descriptions of every Go package in the repository, grouped by layer. The package name is the directory under `internal/`, `sdk/`, or `cmd/`. Use this as a lookup when reading source.

## Server packages (`internal/`)

### HTTP and entry points

| Package | Path | Purpose |
|---|---|---|
| `api` | `internal/api/` | HTTP REST API, middleware (CORS, security headers, recovery, logging), SSE streaming, OpenAPI generator glue. |
| `auth` | `internal/auth/` | API key authentication, RBAC, OAuth (Google, GitHub). |
| `config` | `internal/config/` | Loads and validates all environment variables. Single source of truth for runtime configuration. See [Configuration](configuration.md). |
| `observability` | `internal/observability/` | Retention sweep (trace / snapshot / audit TTLs) and operational policies. |

### Core domain and execution

| Package | Path | Purpose |
|---|---|---|
| `models` | `internal/models/` | Domain structs (`Prompt`, `Agent`, `Workflow`, `EvalRun`, `AuditEntry`, etc.). |
| `store` | `internal/store/` | `Repository` interface and the SQLite implementation. Owns the migration runner and the audit chain. See ADR [0003](adr/0003-hash-chained-audit-log.md). |
| `promptsheon` | `internal/promptsheon/` | The CAS engine. Blobs, trees, commits, refs. Used by the CLI; not currently exposed to the HTTP server. See ADR [0001](adr/0001-use-cas-for-prompt-history.md). |
| `search` | `internal/search/` | BM25 index for prompt search and similarity. See [Algorithms — BM25](algorithms.md#bm25). |
| `workflow` | `internal/workflow/` | DAG-based execution engine for multi-step agents. See [Workflows](workflows.md). |
| `context` | `internal/context/` | Context assembly, token estimation, and truncation strategies. |
| `snapshot` | `internal/snapshot/` | Output snapshot persistence for LLM calls. |
| `eval` | `internal/eval/` | Scorer interface (`exact_match`, `contains`, `regex`, `json_schema` placeholder) + per-case eval runner. See [Eval](eval.md). |
| `harness` | `internal/harness/` | Dataset, Precondition, EvalRun domain types; PreconditionRunner (sh -c with timeouts); harness.Repository interface. See [harness.md](harness.md). |
| `guardrail` | `internal/guardrail/` | Static (pre-LLM) and runtime (post-LLM) guardrail enforcement. See [Guardrails](guardrails.md). |

### Resilience and observability

| Package | Path | Purpose |
|---|---|---|
| `llm` | `internal/llm/` | Provider abstraction (`Provider` interface), per-provider implementations, retry, circuit breaker, fallback, cost, instrumentation. See [LLM Providers](llm-providers.md) and [Algorithms](algorithms.md#retry). |
| `ratelimit` | `internal/ratelimit/` | Token-bucket rate limiter, per API key. |
| `trace` | `internal/trace/` | In-memory distributed tracing. Pluggable backend (OTel). |
| `metrics` | `internal/metrics/` | Prometheus-compatible metrics collector and `/metrics` endpoint. |
| `alerting` | `internal/alerting/` | Alert rules, threshold monitoring, and notification groups. |
| `ws` | `internal/ws/` | WebSocket / SSE hub for real-time log streaming. |

### Security and integration

| Package | Path | Purpose |
|---|---|---|
| `vault` | `internal/vault/` | AES-256-GCM encryption for provider API keys. See [Algorithms — Vault](algorithms.md#vault-aes-256-gcm). |
| `webhook` | `internal/webhook/` | Event delivery to external HTTP endpoints. HMAC signing, SSRF policy, retry. See ADR [0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md). |
| `abtesting` | `internal/abtesting/` | A/B testing engine for prompt versions. |
| `optimizer` | `internal/optimizer/` | AI-powered prompt analysis and improvement suggestions. |
| `playground` | `internal/playground/` | Interactive prompt testing environment (server-side state). |
| `collab` | `internal/collab/` | Real-time collaborative editing over WebSockets. |

## Client packages

| Package | Path | Purpose |
|---|---|---|
| `sdk` | `sdk/` | Go client library for the REST API. See [SDK](sdk.md). |
| `promptsheon` (cmd) | `cmd/promptsheon/` | CLI client. Operates on a `.promptsheon` repository. See [CLI](cli.md). |
| `promptsheond` (cmd) | `cmd/promptsheond/` | Server daemon. Entry point: `main.go` in `cmd/promptsheond/`. |

## Tooling

| Package | Path | Purpose |
|---|---|---|
| `genopenapi` | `scripts/genopenapi/` | Walks `internal/api/server.go` for routes and `internal/api/handlers_*.go` for request schemas, then emits `api/openapi.yaml`. Idempotent. See [API Reference — Generator](api-reference.md#generator). |

## Where to start reading

If you are new to the codebase:

1. `cmd/promptsheond/main.go` — server startup, wiring, graceful shutdown.
2. `internal/api/server.go` — route table, middleware chain, the `Func` signature (exported as `api.Func`).
3. `internal/config/config.go` — every env var and its default.
4. `internal/store/sqlite.go` — the `Repository` interface and its SQLite implementation.
5. `internal/llm/provider.go` — the `Provider` interface, the `Request`/`Response` shapes.

If you are adding a new feature:

1. Pick the package closest to the concern (above).
2. Add the public type and method.
3. Wire it into `internal/api/server.go` and write the handler in a `handlers_*.go` file.
4. Run `make openapi` to regenerate the spec.
5. Add tests under the same directory.

## Naming conventions

- Package names are short, lowercase, singular (`store`, not `stores`).
- One package per directory.
- Tests live in the same package as the code they test (`store_test` is in `internal/store/`).
- Handler files are named `handlers_<topic>.go`.
- Internal-only types are unexported; everything that crosses a package boundary is exported.
