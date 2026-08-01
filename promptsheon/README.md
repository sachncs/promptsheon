# Backend

The Promptsheon API server backend.

## Layout

The `backend/` tree has two tiers:

### Tier 1: Subsystem packages (real cohesive domains)

These are kept as separate Go packages because they own a coherent technical
domain with non-trivial internal structure:

| Package | Purpose | LoC |
|---|---|---:|
| `cas` | Git-like content-addressable storage | ~6.0k |
| `store` | SQLite-backed persistence | ~5.0k |
| `llm` | LLM provider abstraction (OpenAI, Anthropic, ...) | ~4.0k |
| `selfevolve` | Closed-loop prompt self-evolution | ~2.5k |
| `capability` | Core domain model (workspace, project, capability, version) | ~1.1k |
| `vault` | Encryption-at-rest + KMS providers | ~1.0k |
| `release` | Release lifecycle (vote, activate, rollback) | ~0.8k |
| `harness` | Eval harness runner | ~1.0k |
| `auth` | Authn/Authz (sessions, OAuth, permissions) | ~1.3k |
| `webhook` | Outbound webhook delivery | ~1.1k |
| `workflow` | Workflow engine | ~1.5k |
| `eval` | Eval scorers (LLM-as-judge) | ~0.8k |
| `bandit` | Multi-armed bandit selector | ~1.5k |
| `recommendation` | Recommendation aggregate | ~0.4k |
| `settings` | LWW CRDT settings | ~0.6k |
| `observation` | Execution observation windows | ~0.4k |
| `search` | BM25 full-text search | ~0.8k |
| `pluginproto` | Wire contract for plugins | (proto stubs) |
| `models` | Shared persistence records | ~0.2k |
| `spec` | OpenAPI/Swagger YAML | (YAML) |
| `plugins/builtins` | Built-in plugin registration | ~0.1k |

`metrics` is also kept as a package: it has 10 importers across the backend and
its own internal structure (collector, middleware, transport).

### Tier 2: Behavior-named files at `backend/` root

Smaller cohesive units are flattened into top-level files in `package backend`.
Each file owns one behavior. Notable files:

- HTTP handlers, one per resource (`handlers_auth.go`, `handlers_capabilities.go`, `handlers_workspaces.go`, ...)
- Shared types and helpers (`http.go`, `middleware.go`, `idempotency.go`, ...)
- SSE log streaming (`audit_log.go`)
- Configuration loader (`config.go`)
- Server lifecycle (`server.go`, `options.go`, `routes.go`)
- Adapter glue (`audit.go`, `audit_workers.go`, `bandwidth.go`)
- Detector/Redactor built-ins (`detector.go`, `redactor.go`)
- LLM context manager (`llmcontext.go`)
- Election (`election.go`)
- Limits (Budget + Quota aggregates)
- Adoption tracking (`adoption.go`)
- A/B testing (`experiment.go`)
- Retained metrics (`retention.go`)
- MCP server allowlist (`mcplist.go`)
- Bandit store (`banditstore.go`)
- Reasoning compiler (`reasoning.go`)
- Webhook URL validation (`webhooks.go` - used by `handlers_webhooks.go`)
- Bridge (`recommendation_bridge.go`)

### Test files

Tests are co-located with their implementation. The big `handlers_test.go` was
split into per-handler `handlers_*_test.go` files. Shared test helpers live in
`handlers_test_support_test.go`.

## Why we kept `errs/`

The `errs` package has 27 internal importers — it serves as a single source of
sentinel error values that sub-packages reference without creating fan-out
cycles. Each error carries a domain prefix for diagnostics. Flatttening `errs`
into `package backend` would require every internal package to import the root,
which would create import cycles with several handlers and subsystem packages.

## Why some small packages remain subdirs

A handful of single-file packages (`approval`, `budget`, `context` only had
`Manager`, `quota`, `eventbus`, `executor`, `guardrail`, `invoke`, `metrics`,
`optimizer`, `replay`, `schedule`, `scheduler`, `supervisor`)
could not be flattened without introducing import cycles:

- `metrics` is depended on by 10 backend subdirs; flattening creates cycles.
- `invoke`/`eventbus`/`executor` are tightly coupled to other subdirs.
- `quota`/`budget` are used by `rollups` and `invoke` subdirs.

To flatten these, the dependent subdirs would need to define local interfaces
capturing only the methods they use, and the flattened types would have to
satisfy those interfaces implicitly. That work is documented as a follow-up.

## Build and test

```bash
go build ./...
go test -count=1 -timeout 120s ./backend/...
```
