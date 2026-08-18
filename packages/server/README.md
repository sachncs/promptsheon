# @promptsheon/server

Fastify backend with Strands AI agents, SQLite, and SSE streaming.

## Setup

```bash
cd packages
pnpm install
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_PORT` | 8080 | HTTP port |
| `PROMPTSHEON_HOST` | 127.0.0.1 | HTTP host |
| `PROMPTSHEON_DB_PATH` | promptsheon.db | SQLite database path |
| `PROMPTSHEON_CAS_PATH` | .promptsheon | Content-Addressable Store path |
| `PROMPTSHEON_CORS_ORIGIN` | (empty) | CORS allowed origin |
| `PROMPTSHEON_AUTH` | true | Enable API key auth |
| `PROMPTSHEON_JWT_SECRET` | (empty) | JWT secret |
| `PROMPTSHEON_LLM_PROVIDER` | openai | LLM provider: openai, anthropic, bedrock |
| `PROMPTSHEON_LLM_MODEL` | gpt-4 | Model ID |
| `PROMPTSHEON_LLM_API_KEY_ENV` | OPENAI_API_KEY | Env var name holding the API key |
| `PROMPTSHEON_LLM_MAX_RETRIES` | 5 | Max LLM retry attempts |
| `PROMPTSHEON_LLM_TIMEOUT_MS` | 120000 | LLM request timeout |
| `PROMPTSHEON_SELF_EVOLVE_ENABLED` | false | Enable self-evolution |
| `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC` | 900 | Default cooldown between cycles |
| `PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT` | 3 | Max concurrent cycles |

## Run

```bash
# Dev (with tsx)
npm run dev

# Production
npm run build && npm start
```

## API

See [API.md](./API.md) for the full REST endpoint reference.

Key endpoints:
- `GET /api/health` — Health check
- `POST /api/invoke` — Invoke a capability via Strands agent
- `POST /api/eval/run` — Run evaluation
- `POST /api/self-evolve/run` — Trigger self-evolution cycle
- `GET /api/events/:channel` — SSE event stream

## Architecture

- `src/index.ts` — Fastify app bootstrap, wires DB, repos, agents, middleware, routes, scheduler
- `src/routes/` — 17 route groups (REST endpoints)
- `src/middleware/` — 7 middleware (auth, CORS, rate-limit, SSRF, idempotency, audit, errors)
- `src/agents/` — 4 Strands AI agents (invocation, evaluation, evolution, compiler)
- `src/repos/` — 29 repository modules (better-sqlite3)
- `src/scheduler/` — Polling scheduler, alert checker, webhook delivery
- `src/audit/` — Hash-linked append-only audit chain
- `src/sse/` — Server-Sent Events pub/sub hub

## Tests

```bash
npm test
```

5 test files, 23 tests covering DB, CAS, audit, settings, routes.
