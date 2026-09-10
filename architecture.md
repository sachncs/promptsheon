---
layout: page
title: Architecture
subtitle: How the backend, the agents, and the audit chain fit together.
---

# Architecture

Promptsheon is a TypeScript monorepo. This page describes the on-disk architecture, the runtime architecture, and the cross-cutting concerns (auth, audit, observability).

## Repository layout

```
promptsheon/
├── packages/
│   ├── shared/      Domain types, validation schemas, SQLite migrations, CAS
│   ├── server/      Fastify backend, Strands agents, Cedar authorizer, audit chain
│   ├── cli/         Command-line client
│   └── sdk/         Typed fetch client for Node and TypeScript
├── frontend/        Next.js 16 + React 19 control plane
├── extensions/
│   └── promptsheon/ VS Code extension
├── docs/            Operator-facing material (SOC2, threat model, on-prem)
└── scripts/         Build helpers (offline installer, SBOM, social-preview)
```

Every package is a pnpm workspace member. A single `pnpm install` resolves the whole tree.

## Runtime architecture

```
┌────────────────────────────────────────────────────────────┐
│                        Browser                             │
│                  Next.js 16 + React 19                     │
└──────────────────┬─────────────────────────────────────────┘
                   │ /api/* (rewritten to :8080)
┌──────────────────▼─────────────────────────────────────────┐
│                       Fastify 5                            │
│                                                             │
│  onRequest:                                                 │
│    1. requestId + startTime stamped                         │
│    2. cors + rateLimit + helmet (if enabled)                │
│                                                             │
│  preHandler:                                                │
│    3. authMiddleware  — Bearer | SVID | dev X-User-Id      │
│    4. orgContextMiddleware — resolves org from session     │
│    5. Cedar gate (when PROMPTSHEON_AUTH=true)              │
│                                                             │
│  handler:                                                   │
│    6. Zod-validated route handler                           │
│    7. Repo / agent / scheduler call                        │
│    8. AuditChain.append(...) on every state transition     │
│                                                             │
│  onResponse:                                                │
│    9. Structured log line with status + duration            │
└──────────────────┬─────────────────────────────────────────┘
                   │
        ┌──────────┴──────────┐
        ▼                     ▼
┌────────────────┐   ┌─────────────────────┐
│  better-sqlite3 │   │  Content-Addressed  │
│  (workspaces,   │   │  Store on disk      │
│   releases, …)  │   │  (.promptsheon/)    │
└────────────────┘   └─────────────────────┘
```

## Boot sequence

`packages/server/src/index.ts` (`main()`):

1. `loadConfig()` reads environment variables and returns an `AppConfig`.
2. `validateConfig(config)` fails the boot on missing JWT secret or bad port.
3. `createConnection(config)` opens the SQLite database.
4. `runMigrations(db)` applies every migration under `packages/shared/db/migrations/`.
5. `AuditChain` and `CedarAuthorizer` are constructed and installed as singletons.
6. Repos, agents, the LLM router, the SSE hub, the retention sweeper, the webhook receiver, the scheduler are wired.
7. Fastify registers CORS, rate limiting, request/response hooks, and the auth + org-context middleware.
8. `registerRoutes(app, deps)` registers every route under `/api/*` with its Zod-validated schema.
9. `app.listen({port, host})` starts the HTTP server.
10. SIGINT and SIGTERM handlers stop the scheduler, close the Fastify server, and close the SQLite handle.

## Cross-cutting concerns

### Authentication

`authMiddleware` (in `packages/server/src/middleware/auth.ts`) accepts three input shapes:

| Header | Effect |
|--------|--------|
| `Authorization: Bearer <token>` | sha256 lookup in `api_keys`; on success stamps `userId` and `userRole`. |
| `Authorization: SVID <token>` | ed25519 verification against `PROMPTSHEON_SVID_PUBLIC_KEY_PEM`; on success stamps an `Agent` principal with id, org, classification. |
| `X-User-Id` (only when `PROMPTSHEON_AUTH=false`) | Dev/test fallback. |

When `PROMPTSHEON_AUTH=true` and no Bearer or SVID header is present, the middleware returns `401 UNAUTHORIZED`. The legacy `X-User-Id` fallback is *not* honoured in this mode.

### Authorization

Cedar policies under `packages/server/policies/promptsheon.cedar` are the single source of truth for every authorization decision. The `CedarAuthorizer` is loaded at boot and invoked by the gate middleware. The smoke-test route is `/api/orgs/:orgId/teams` (see `routes/org-team.ts`). Callers that haven't migrated to the gate yet can still use the legacy `requireRole()` helper in `middleware/org-context.ts`.

### Audit chain

Every state transition appends a frame to the hash-linked chain. A frame has:

```typescript
interface AuditFrame {
  index: number;
  timestamp: string;
  prevHash: string;
  actor: { kind: 'user' | 'agent' | 'system'; id: string };
  action: string;
  resourceKind: string;
  resourceId: string;
  details: Record<string, unknown>;
  hash: string;
}
```

The chain is publicly verifiable at `GET /api/audit/verify`. Tampering with any frame returns `{"valid": false, "brokenAt": N}`. The chain is never swept by the retention sweeper.

### Content-addressed store

The CAS lives under `PROMPTSHEON_CAS_PATH` (default `.promptsheon/`) with a sharded layout (`aa/bb/<full-hash>`) that keeps any single directory bounded. Garbage collection runs alongside the retention sweeper.

### Observability

Pino for structured logging, OpenTelemetry for distributed tracing. Set `PROMPTSHEON_OTEL_ENDPOINT` to enable the OTLP exporter.

## Agent subsystems

`packages/server/src/agents/` ships seven Strands-powered subsystems:

- **Invocation** — runs a single capability release; the canonical `/api/executions` path.
- **Evaluation** — runs an eval suite against a capability; reports per-case scores.
- **Evolution** — watches live eval scores and re-plans the manifest when a release drifts.
- **Goal evolver** — a higher-level evolution loop keyed to a goal hash.
- **Compiler** — turns a free-form prompt into a manifest DAG.
- **Planner** — the `Swarm` that decomposes a goal into a capability DAG (5 specialised agents).
- **Executor** — the `Graph` that runs a manifest node-by-node with a shared scratchpad.

Each subsystem uses `AgentResult.lastMessage.content` (not `.message`) for its return text. The shared `extractText()` helper handles both shapes.

## Scheduler

`Scheduler` polls `scheduleRepo` every `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC` for pending cycles, fires the alert checker, and dispatches webhook deliveries. The retention sweeper prunes `eval_results` and `human_review_queue` past the configured horizon. The audit chain is never swept.

## What runs where

| Process | Address | Restart on crash | Logs |
|---------|---------|------------------|------|
| Fastify backend | `:8080` | systemd or `pnpm start` | stdout (Pino JSON) |
| Next.js UI | `:3000` | Next.js dev server | stdout |
| Firewall sidecar | `:9090` (configurable) | optional | stdout |
| Replicator | n/a (one-shot or daemon) | systemd timer | stdout |
| Failover CLI | one-shot | manual | stdout |

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Walk through a guided build | [Tutorials]({{ '/tutorials/' | relative_url }}) |
| Read the threat model | [Security]({{ '/security/' | relative_url }}) |
