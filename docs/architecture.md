# Architecture

## Overview

Promptsheon applies Git's immutable object model to AI agent configurations. Every prompt, agent workflow, and evaluation run is stored as a content-addressed object in a Merkle DAG, giving you cryptographic integrity, full history, and diff capabilities for AI assets.

## Storage Model

Four object types form the core:

| Object | Purpose | Mutability |
|---|---|---|
| **Blob** | Raw content (prompt text, tool configs, YAML) | Immutable |
| **Tree** | Named mapping of blobs forming a snapshot | Immutable |
| **Commit** | Immutable node: references a tree, parent commits, and metadata | Immutable |
| **Ref** | Mutable pointer (branch) to a named commit | Mutable |

Every object is addressed by its SHA-256 hash. Content-addressable storage ensures deduplication and tamper evidence.

## Component Map

```
┌─────────────────────────────────────────────────────────────────┐
│                        REST API (api)                           │
│          http.Handler · SSE streaming · JSON responses          │
├──────────┬──────────┬──────────┬──────────┬──────────┬─────────┤
│  Auth    │ Prompts  │ Agents   │ Eval     │ Guard    │ Alerts  │
│ (authn/  │          │          │ Runner   │ rails    │         │
│  authz)  │          │          │          │          │         │
├──────────┴──────────┴──────────┴──────────┴──────────┴─────────┤
│                         Core Engine                             │
│   LLM Providers (llm) · Workflow Engine (workflow)              │
│   Context Manager (context) · Snapshot Store (snapshot)         │
├─────────────────────────────────────────────────────────────────┤
│                         Persistence                             │
│   SQLite (store) · Vault (vault) · Webhooks (webhook)          │
├─────────────────────────────────────────────────────────────────┤
│                      Observability                              │
│   Tracing (trace) · Metrics (metrics) · Alerting (alerting)    │
│   WebSocket Hub (ws) · Rate Limiting (ratelimit)               │
└─────────────────────────────────────────────────────────────────┘
```

## Package Layout

```
promptsheon/
├── cmd/
│   ├── promptsheond/      Server daemon (main)
│   └── promptsheon/       CLI client (main)
├── internal/
│   ├── api/               HTTP REST API, middleware, SSE streaming
│   ├── auth/              Authentication, authorization, OAuth, API keys
│   ├── llm/               Provider abstraction (OpenAI, Anthropic, Azure, Ollama, NVIDIA)
│   ├── models/            Domain structs (Prompt, Agent, Workflow, etc.)
│   ├── store/             SQLite persistence (Repository interface)
│   ├── eval/              Evaluation engine, comparators, scorers
│   ├── workflow/          DAG-based workflow execution engine
│   ├── guardrail/         Content policy rules and enforcement
│   ├── context/           Context assembly and message management
│   ├── snapshot/          Output snapshot storage
│   ├── trace/             Distributed tracing (spans, trace trees)
│   ├── metrics/           Prometheus-compatible metrics collector
│   ├── alerting/          Threshold-based alerting manager
│   ├── vault/             Encrypted provider key storage
│   ├── webhook/           Webhook dispatcher (HMAC-signed)
│   ├── ws/                WebSocket hub for real-time streaming
│   ├── ratelimit/         Token-bucket rate limiting
│   └── observability/     Structured logging, correlation IDs
├── sdk/                   Go client SDK for the REST API
├── api/                   OpenAPI 3.0 specification
├── test/                  Integration tests
└── docs/                  This documentation
```

## Design Principles

1. **Immutable by default** — Every version is a permanent, addressable object.
2. **Content-addressable** — SHA-256 hashes serve as unique IDs and integrity checks.
3. **Audit-chain integrity** — Audit entries form a hash-linked chain for tamper detection.
4. **Accept interfaces, return structs** — Go idioms for composability.
5. **Zero external dependencies for storage** — SQLite via `modernc.org/sqlite` (pure Go, CGO-free).
6. **Observable by default** — Tracing, metrics, and structured logging are first-class.

## Data Flow

```
User Request
    │
    ▼
┌──────────┐     ┌──────────┐     ┌──────────┐
│   API    │────▶│  AuthZ   │────▶│ Guard    │
│  Server  │     │  Check   │     │  Rails   │
└──────────┘     └──────────┘     └──────────┘
    │                                   │
    ▼                                   ▼
┌──────────┐                     ┌──────────┐
│  Store   │                     │  LLM     │
│ (SQLite) │                     │ Provider │
└──────────┘                     └──────────┘
    │                                   │
    ▼                                   ▼
┌──────────┐                     ┌──────────┐
│  Trace   │                     │  Eval    │
│  Spans   │                     │  Runner  │
└──────────┘                     └──────────┘
```

## Further Reading

- [Getting Started](getting-started.md)
- [Configuration](configuration.md)
- [API Reference](api-reference.md)
