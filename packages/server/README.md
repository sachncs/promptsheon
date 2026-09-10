# @promptsheon/server

Fastify 5 backend with Strands AI agents, SQLite, and SSE streaming.

## Setup

```bash
pnpm install                # installs all workspace deps from the repo root
pnpm --filter @promptsheon/shared build
pnpm --filter @promptsheon/server build
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_PORT` | 8080 | HTTP port |
| `PROMPTSHEON_HOST` | 127.0.0.1 | HTTP host |
| `PROMPTSHEON_DB_PATH` | promptsheon.db | SQLite database path |
| `PROMPTSHEON_CAS_PATH` | .promptsheon | Content-Addressable Store path |
| `PROMPTSHEON_FRONTEND_PATH` | ./frontend/dist | Built frontend directory served on the same host |
| `PROMPTSHEON_CORS_ORIGIN` | (empty → `http://localhost:3000`) | CORS allowed origin |
| `PROMPTSHEON_AUTH` | true | Enable API key + SVID auth |
| `PROMPTSHEON_JWT_SECRET` | (empty) | Required when `PROMPTSHEON_AUTH=true` |
| `PROMPTSHEON_LLM_PROVIDER` | openai | LLM provider: openai, anthropic, bedrock, custom |
| `PROMPTSHEON_LLM_MODEL` | gpt-4 | Model ID |
| `PROMPTSHEON_LLM_API_KEY_ENV` | OPENAI_API_KEY | Env var name holding the API key |
| `PROMPTSHEON_LLM_MAX_RETRIES` | 5 | Max LLM retry attempts |
| `PROMPTSHEON_LLM_TIMEOUT_MS` | 120000 | LLM request timeout |
| `PROMPTSHEON_SELF_EVOLVE_ENABLED` | false | Enable self-evolution |
| `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC` | 900 | Default cooldown between cycles |
| `PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT` | 3 | Max concurrent cycles |
| `PROMPTSHEON_FIPS_MODE` | false | Enforce FIPS-validated crypto for the audit chain |
| `PROMPTSHEON_WEBHOOK_SECRET` | (empty → dev fallback) | HMAC secret for incoming webhooks — required in production |
| `PROMPTSHEON_ALLOW_SYSTEM_ACTOR` | (dev-only default true) | Enable `X-User-Id: api` bypass in dev/test only |

The single source of truth for these defaults is
[`src/config/env.ts`](src/config/env.ts); the README is generated
from it. Edit the env file when adding a new variable.

## Run

```bash
pnpm dev          # Fastify + tsx watch, :8080
pnpm build        # tsc → dist/
pnpm start        # node dist/index.js
```

## API

See [API.md](./API.md) for the full REST endpoint reference.
OpenAPI 3.1 is auto-emitted at `/api/openapi.json` once the
server is running.

Key endpoints:

- `GET /api/health` — Health check
- `POST /api/invoke` — Invoke a capability via Strands agent
- `POST /api/executions` — Execute a capability (streams over SSE when `Accept: text/event-stream`)
- `POST /api/eval/run` — Run an evaluation
- `POST /api/self-evolve/run` — Trigger self-evolution cycle
- `GET /api/events/:channel` — SSE event stream

## Architecture

The numbers below are generated from `pnpm stats` (see
`scripts/stats.sh`); regenerate before tagging a release.

- `src/index.ts` — Fastify app bootstrap, wires DB, repos, agents, middleware, routes, scheduler
- `src/routes/` — 56 route modules (REST + SSE endpoints, see `src/routes/index.ts`)
- `src/middleware/` — 5 middleware modules (auth, cors, org-context, admin, error mapping)
- `src/agents/` — 7 Strands AI agent subsystems (invocation, evaluation, evolution/goal, compiler, planner, executor, replay)
- `src/repos/` — 43 repository modules (better-sqlite3)
- `src/policy/` — Cedar policy loader + authorizer gate
- `src/scheduler/` — Polling scheduler, alert checker, retention sweeper, webhook delivery
- `src/audit/` — Hash-linked append-only audit chain
- `src/sse/` — Server-Sent Events pub/sub hub
- `src/firewall/` — Prompt firewall sidecar (Fastify plugin + standalone CLI)

## Tests

```bash
pnpm test
```

Counts (regenerate with `scripts/stats.sh`):

- `packages/server`: 86 test files, 619 vitest cases.
- `packages/shared`: 4 test files, 37 vitest cases.

The server tests live in [`test/`](test/) and cover DB,
CAS, audit, settings, routes, policy, gateway, scheduler,
retention, firewall, CLI, and the agent subsystems.
