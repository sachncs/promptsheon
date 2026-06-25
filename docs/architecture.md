# Architecture

## Overview

Promptsheon applies Git's immutable object model to AI agent configurations. Every prompt, agent workflow, tool spec, and evaluation result is stored as a content-addressed object in a Merkle DAG, giving you cryptographic integrity, full history, and diff capabilities for AI assets. The HTTP server and the Go SDK are thin layers on top of this core.

## Storage model

Four object types form the core:

| Object | Purpose | Mutability |
|---|---|---|
| **Blob** | Raw content (prompt text, tool configs, YAML) | Immutable |
| **Tree** | Named mapping of blobs forming a snapshot | Immutable |
| **Commit** | Immutable node: references a tree, parent commits, and metadata | Immutable |
| **Ref** | Mutable pointer (branch) to a named commit | Mutable |

Every object is addressed by its SHA-256 hash. Content-addressable storage ensures deduplication and tamper evidence. The CAS lives in `internal/promptsheon/` and is used by the CLI; the HTTP server stores prompts, agents, and workflows in SQLite keyed by the same hashes.

See [Modules](modules.md#core-domain-and-execution) and ADR [0001](adr/0001-use-cas-for-prompt-history.md).

## System diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                           Clients                                │
│   CLI (promptsheon) · Go SDK · curl · web playground             │
└──────────────────────────────┬───────────────────────────────────┘
                               │ HTTP/JSON
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                      HTTP API (internal/api)                     │
│   Mux · middleware (CORS, security headers, recovery, MaxBytes)  │
│   Handlers (handlers_*.go) · SSE streaming                       │
├──────────────┬──────────────┬──────────────┬──────────────┬──────┤
│  Auth        │  Workflow    │  Eval        │  Guardrail   │ ...  │
│  (authn,     │  Engine      │  Engine      │  (static +   │      │
│   RBAC,      │  (DAG)       │  (scorers,   │   runtime)   │      │
│   OAuth)     │              │   halluc.)   │              │      │
├──────────────┴──────┬───────┴──────┬───────┴──────┬───────┴──────┤
│                     │              │              │              │
│   LLM Providers (internal/llm)      │   Search    │  Snapshot    │
│   OpenAI · Anthropic · Azure ·      │   (BM25)    │              │
│   Ollama · NVIDIA                   │             │              │
│   + retry · circuit breaker ·       │             │              │
│     fallback · cost                 │             │              │
├─────────────────────┴──────────────┴──────────────┴──────────────┤
│                       Persistence (internal/store)               │
│   SQLite (modernc.org/sqlite) · Repository interface             │
│   Hash-chained audit log · Migrations                            │
├──────────────────────────────────────────────────────────────────┤
│                  Security & integration                           │
│   Vault (AES-256-GCM) · Webhooks (HMAC, SSRF allowlist)          │
├──────────────────────────────────────────────────────────────────┤
│                    Observability                                  │
│   slog (logging) · trace (in-memory + OTel)                      │
│   metrics (Prometheus) · alerting · retention sweeper             │
│   WebSocket / SSE hub · rate limiter                              │
└──────────────────────────────────────────────────────────────────┘
```

## Package layout

```
promptsheon/
├── cmd/
│   ├── promptsheond/      Server daemon (main.go, e2e_test.go)
│   └── promptsheon/       CLI client (main.go)
├── internal/
│   ├── api/               HTTP REST API, middleware, SSE, OpenAPI glue
│   ├── auth/              API keys, RBAC, OAuth (Google, GitHub)
│   ├── config/            Environment-variable loader
│   ├── llm/               Provider abstraction, retry, circuit breaker, fallback, cost
│   ├── models/            Domain structs (Prompt, Agent, Workflow, etc.)
│   ├── store/             Repository interface and SQLite implementation
│   ├── promptsheon/       CAS engine (blobs, trees, commits, refs)
│   ├── search/            BM25 index
│   ├── workflow/          DAG-based execution engine
│   ├── eval/              Evaluation engine, scorers, hallucination detector
│   ├── guardrail/         Static and runtime guardrail enforcement
│   ├── context/           Context assembly and token budget management
│   ├── snapshot/          Output snapshot persistence
│   ├── trace/             Distributed tracing (in-memory + OTel)
│   ├── metrics/           Prometheus-compatible metrics collector
│   ├── alerting/          Threshold-based alerting
│   ├── vault/             Encrypted provider key storage (AES-256-GCM)
│   ├── webhook/           Webhook dispatcher (HMAC, SSRF)
│   ├── ws/                WebSocket / SSE hub
│   ├── ratelimit/         Token-bucket rate limiter
│   ├── observability/     Retention policies
│   ├── abtesting/         A/B testing engine
│   ├── optimizer/         Prompt analysis and improvement
│   ├── playground/        Interactive prompt testing
│   └── collab/            Real-time collaborative editing
├── sdk/                   Go client SDK
├── scripts/
│   └── genopenapi/        OpenAPI generator (AST-based, idempotent)
├── api/
│   └── openapi.yaml       Generated spec
├── test/                  Integration tests
├── tests/load/            k6 load scenarios
└── docs/                  This documentation
```

For one-line package purposes, see [Modules](modules.md).

## Database migrations

The schema is applied by the migration runner in `internal/store/sqlite.go` at server startup. Migrations are SQL files in `internal/store/migrations/`, named `NNN_description.sql`.

| # | File | Purpose |
|---|---|---|
| 001 | `001_initial.sql` | Initial schema: prompts, prompt versions, basic metadata. |
| 002 | `002_users.sql` | User accounts. |
| 003 | `003_eval_runs.sql` | Evaluation runs, test cases, scores. |
| 004 | `004_workflows.sql` | Workflow definitions. |
| 005 | `005_snapshots.sql` | Output snapshots. |
| 006 | `006_audit_chain.sql` | Hash chaining on `audit_entries`. |
| 007 | `007_provider_keys.sql` | Provider key metadata (encrypted blobs in the vault). |
| 008 | `008_prompt_binding.sql` | Prompt-to-provider binding. |
| 009 | `009_environment.sql` | Per-environment overrides. |
| 010 | `010_review_quorum.sql` | Review quorum (multi-approver). |
| 011 | `011_execution_logs.sql` | Execution-time logs. |
| 012 | `012_prompt_system_prompt.sql` | System prompt split-out. |
| 013 | `013_guardrail_rules.sql` | Guardrail rules table. |
| 014 | `014_prompt_generation_config.sql` | Generation parameters (temperature, max tokens). |
| 015 | `015_contexts.sql` | Conversation contexts. |
| 016 | `016_agent_configs.sql` | Agent configurations. |
| 017 | `017_agent_executions.sql` | Agent execution history. |
| 018 | `018_alerts.sql` | Alert rules and notifications. |
| 019 | `019_webhook_endpoints.sql` | Webhook endpoints and deliveries. |
| 020 | `020_audit_canonical_ts.sql` | Canonical RFC3339Nano timestamp column. |
| 021 | `021_audit_chain_state.sql` | Single-row state cache for the chain head. |

To add a migration, see [Development — Migrations](development.md#migrations).

## Design principles

1. **Immutable by default.** Every version is a permanent, addressable object.
2. **Content-addressable.** SHA-256 hashes serve as unique IDs and integrity checks.
3. **Audit-chain integrity.** Audit entries form a hash-linked chain for tamper detection. See ADR [0003](adr/0003-hash-chained-audit-log.md).
4. **Accept interfaces, return structs.** Go idioms for composability.
5. **Zero external dependencies for storage.** SQLite via `modernc.org/sqlite` (pure Go, no CGO). See ADR [0006](adr/0006-modernc-sqlite-no-cgo.md).
6. **Observable by default.** Tracing, metrics, and structured logging are first-class. See ADR [0007](adr/0007-slog-as-observability-foundation.md).
7. **Fail closed.** The shell tool, the SSRF policy, the vault key validation — every security control defaults to the safe state.

## Request lifecycle

A typical `POST /api/v1/prompts/{id}/run` flow:

```
client → API server
   ↓
middleware chain: logging → CORS → security headers → recovery → auth
   ↓
handler: parse body, validate, look up prompt in store
   ↓
binding resolution: provider, model, api_key_ref → vault lookup
   ↓
guardrail check (static, pre-LLM)
   ↓
LLM middleware chain: timeout → circuit breaker → retry → fallback
   ↓
provider call
   ↓
guardrail check (runtime, post-LLM)
   ↓
persist result (snapshot, trace span, audit entry)
   ↓
emit webhook event (if subscribed)
   ↓
response to client
```

Every step emits a log line, a span, and (for state-changing steps) an audit entry. The chain is preserved end-to-end through the request ID.

## Further reading

- [Getting Started](getting-started.md)
- [Configuration](configuration.md)
- [API Reference](api-reference.md)
- [Modules](modules.md)
- [Algorithms](algorithms.md)
- [Security](security.md)
- [Observability](observability.md)
- [Design Decisions](design-decisions.md) and the [ADRs](adr/README.md)
