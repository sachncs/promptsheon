<p align="center">
  <h1 align="center">promptsheon</h1>
  <p align="center">Prompt management platform — DAG editor, releases with canary rollout, eval suites, maker-checker approvals, and self-evolution, all powered by Strands Agents.</p>
  <p align="center">
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache_2.0-blue" alt="License"></a>
    <a href="https://github.com/sachncs/promptsheon/actions"><img src="https://img.shields.io/github/actions/workflow/status/sachncs/promptsheon/ci.yaml?branch=master" alt="CI"></a>
    <a href="https://github.com/sachncs/promptsheon/stargazers"><img src="https://img.shields.io/github/stars/sachncs/promptsheon" alt="Stars"></a>
  </p>
</p>

A single Fastify backend on `:8080` plus a Next.js web UI on `:3000`,
talking to a local SQLite file and a content-addressed store. No cloud
account, no signup — `pnpm install && pnpm dev` and you have a fully
working prompt-management platform with multi-provider LLM, audit chain,
webhooks, eval scorers, and a live DAG editor.

Built using [**Strands Agents SDK**](https://github.com/strands-agents/sdk)
for every AI call: planning is a 5-agent `Swarm`, execution is a `Graph`
of per-node `Agent`s, and standalone `Agent`s handle compilation,
scoring, and self-evolution.

Full documentation: **[`AGENTS.md`](AGENTS.md)** — the engineering
constitution. Per-package references at
[`packages/shared/README.md`](packages/shared/README.md),
[`packages/server/README.md`](packages/server/README.md), and the API
reference at [`packages/server/API.md`](packages/server/API.md).

---

## Features

- **DAG-based capability editor** — visual node-graph editor with
  per-node config, runtime guards, and live execution preview. See
  `frontend/src/components/dag/`.
- **Releases with canary rollout** — versioned releases, per-environment
  activation, weighted traffic split (`canary_percent`), and one-click
  rollback / supersede. See
  `packages/server/src/routes/release.ts`.
- **Maker-checker approvals** — release creator cannot approve their own
  release; approvals are persisted with reason + voter. See
  `packages/server/src/routes/manifest-approval.ts`.
- **Strands-powered planning** — `Swarm` of 5 specialised agents
  decomposes a goal into a capability DAG (DAG-Builder, Idea-Splitter,
  Goal-Refiner, plus orchestrator + retry). See
  `packages/server/src/agents/planner/`.
- **Strands-powered execution** — each capability node becomes a `Graph`
  node with its own `Agent`, shared scratchpad, and observability hooks.
  See `packages/server/src/agents/executor/`.
- **Multi-provider LLM** — OpenAI, Anthropic, or AWS Bedrock, selected
  via `PROMPTSHEON_LLM_PROVIDER`. Strands handles retries, timeouts,
  and structured-output validation.
- **Eval suites + scorers** — declarative dataset cases, pluggable
  scorers (LLM-judge, regex, exact-match), and parallel run results.
  See `packages/server/src/agents/evaluation/`.
- **Self-evolution loop** — monitors live eval scores of an active
  release; on regression it triggers a re-plan and re-release with
  cooldown. See `packages/server/src/agents/evolution/`.
- **Audit chain** — append-only, hash-linked audit log with
  cryptographic verification endpoint. See
  `packages/server/src/routes/audit.ts` and migration `002_audit_chain`.
- **Content-addressed store (CAS)** — every compiled manifest is hashed
  and stored by content, never by name. See
  `packages/shared/src/cas/`.
- **Maker-checker webhooks** — incoming webhooks with HMAC verification
  and replay protection. See
  `packages/server/src/routes/webhooks-incoming.ts`.
- **Chaos engineering hooks** — admin endpoints to inject, list, and
  clear failures on any route for resilience drills. See
  `packages/server/src/routes/chaos.ts`.
- **Observability** — OpenTelemetry traces + metrics, structured
  Pino logs, SSE event stream. See
  `packages/server/src/observability/`.
- **Rate limiting + CORS** — `@fastify/rate-limit` (per-IP, configurable
  trust list) and `@fastify/cors` (configurable origin).

---

## Installation

### From source

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
pnpm install
cp .env.example .env
# fill in your LLM API key (OPENAI_API_KEY / ANTHROPIC_API_KEY / AWS_BEDROCK_*)
pnpm dev   # from inside packages/
```

By default the backend listens on `http://localhost:8080` and the
frontend on `http://localhost:3000`. No external services beyond the
LLM provider you choose.

### With hot reload

```bash
cd packages
pnpm dev:server    # Fastify + tsx watch
pnpm dev:frontend  # Next.js + Turbopack
```

### From Docker

Not shipped — Docker support is on the roadmap. See
[Roadmap](#roadmap).

**Requirements:** Node.js ≥ 26, pnpm 11.

---

## Quick Start

### 1. Confirm the backend is up

```bash
curl http://localhost:8080/api/health
# {"status":"ok","service":"promptsheon-server", ...}
```

### 2. Open the UI

```text
http://localhost:3000
```

The frontend `next.config.ts` rewrites `/api/*` →
`http://localhost:8080/api/*`, so you only need both servers running.

### 3. Make your first capability

1. Go to **Workspaces** → pick or create a workspace
2. **Goals** → describe the capability you want
3. The planner `Swarm` decomposes it into a DAG
4. Edit nodes in the DAG editor
5. **Compile** → manifest is hashed into the CAS
6. **Release** → creates a v1 release; activate it
7. **Executions** → invoke the capability with a payload
8. **Eval** → run a dataset, see per-case scores

### TypeScript SDK

The repo ships as a self-hosted backend — there is no published npm
package yet. If you want to drive the API from your own code, see the
typed client in `frontend/src/lib/api.ts` for the full request shapes,
or read [`packages/server/API.md`](packages/server/API.md).

---

## Configuration

All configuration is via environment variables prefixed with
`PROMPTSHEON_`. Copy `.env.example` to `.env` and edit as needed.
**Never commit `.env`** — it is in `.gitignore`.

### Server

| Variable                       | Purpose                              |
|--------------------------------|--------------------------------------|
| `PROMPTSHEON_PORT`             | HTTP listen port (default `8080`)    |
| `PROMPTSHEON_HOST`             | Bind address (default `127.0.0.1`)   |
| `PROMPTSHEON_DB_PATH`          | SQLite file path                     |
| `PROMPTSHEON_CAS_PATH`         | Content-addressed store directory    |
| `PROMPTSHEON_FRONTEND_PATH`    | Path to the built frontend (`./frontend/.next`) |
| `PROMPTSHEON_CORS_ORIGIN`      | Allowed CORS origin (default `http://localhost:3000`) |
| `PROMPTSHEON_LOG_LEVEL`        | Pino log level                       |

### Authentication

| Variable                | Purpose                                  |
|-------------------------|------------------------------------------|
| `PROMPTSHEON_AUTH`      | Enable JWT auth (`true` / `false`)       |
| `PROMPTSHEON_JWT_SECRET`| HMAC secret for token verification       |

### LLM (Strands Agents SDK)

| Variable                          | Purpose                                       |
|-----------------------------------|-----------------------------------------------|
| `PROMPTSHEON_LLM_PROVIDER`        | `openai` / `anthropic` / `bedrock`            |
| `PROMPTSHEON_LLM_MODEL`           | Model id (e.g. `gpt-4`, `claude-3-5-sonnet`, `anthropic.claude-3-5-sonnet-20241022-v2:0`) |
| `PROMPTSHEON_LLM_API_KEY_ENV`     | Name of the env var holding the API key       |
| `PROMPTSHEON_LLM_MAX_RETRIES`     | Strands retry budget                          |
| `PROMPTSHEON_LLM_TIMEOUT_MS`      | Per-request LLM timeout                       |

The matching provider credential must also be set:

| Variable                 | Purpose                                  |
|--------------------------|------------------------------------------|
| `OPENAI_API_KEY`         | OpenAI provider key                      |
| `ANTHROPIC_API_KEY`      | Anthropic provider key                   |
| `AWS_BEDROCK_REGION`     | Bedrock region (e.g. `us-east-1`)        |

### Self-evolution

| Variable                                  | Purpose                              |
|-------------------------------------------|--------------------------------------|
| `PROMPTSHEON_SELF_EVOLVE_ENABLED`         | Enable the self-evolution loop        |
| `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC`    | Min seconds between re-evolves        |
| `PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT`  | Cap concurrent evolutions per worker  |

### Observability

| Variable                    | Purpose                          |
|-----------------------------|----------------------------------|
| `PROMPTSHEON_OTEL_ENDPOINT` | OpenTelemetry OTLP collector URL |

### Webhooks

| Variable                    | Purpose                          |
|-----------------------------|----------------------------------|
| `PROMPTSHEON_WEBHOOK_SECRET` | HMAC secret for incoming webhooks |

---

## API

31 route modules exposing 76 REST endpoints. Full reference with
request/response shapes:
[`packages/server/API.md`](packages/server/API.md).

| Group        | Sample endpoints                                                         |
|--------------|--------------------------------------------------------------------------|
| Health       | `GET /api/health`                                                        |
| Capabilities | `GET/POST /api/capabilities`, `GET/PUT /api/capabilities/:id`           |
| Versions     | `GET/POST /api/capability-versions`, `GET /api/capability-versions/:id`  |
| Releases     | `GET/POST /api/releases`, `PUT /api/releases/:id/activate`, `PUT /api/releases/:id/canary`, `PUT /api/releases/:id/supersede`, `PUT /api/releases/:id/rollback` |
| Manifests    | `GET /api/manifests`, `GET /api/manifests/:hash`, `POST /api/manifests/:hash/approvals`, `POST /api/manifests/:hash/approve`, `POST /api/manifests/:hash/reject` |
| Compiler     | `POST /api/compiler/compile`, `POST /api/compiler/decompile`             |
| Executions   | `GET /api/executions`, `GET /api/executions/:id`                         |
| Eval         | `POST /api/eval/run`, `POST /api/eval/score`, `GET /api/eval/evaluators`, `GET /api/eval-runs`, `GET /api/eval-runs/:id`, `GET /api/eval-runs/:id/results` |
| Datasets     | `GET/POST /api/datasets`, `GET/POST /api/datasets/:id/cases`             |
| Approvals    | `GET /api/approvals`, `GET /api/approvals/:releaseId`                    |
| Self-evolve  | `POST /api/self-evolve/run`, `GET /api/self-evolve/:capabilityId/state`  |
| Goals        | `GET/POST /api/goals`, `GET /api/goals/:hash`, `POST /api/manifests/:hash/evolve`, `POST /api/ideas/plan` |
| Audit        | `GET /api/audit`, `GET /api/audit/state`, `POST /api/audit/verify`       |
| Webhooks     | `POST /api/webhooks/incoming/:id`                                        |
| Snapshots    | `GET /api/snapshots`, `POST /api/snapshots/:id/restore`                  |
| Sessions     | `GET/POST /api/sessions`, `GET /api/sessions/:id`, `GET /api/sessions/:id/messages` |
| SSE          | `GET /api/events/:channel`                                               |
| Workspaces   | `GET/POST /api/workspaces`, `GET/PUT /api/workspaces/:id`                |
| Projects     | `GET/POST /api/projects`, `GET/PUT /api/projects/:id`                    |
| Users        | `GET /api/users`, `GET /api/users/me`, `PUT /api/users/:id/role`          |
| API keys     | `GET/POST /api/api-keys`, `DELETE /api/api-keys/:id`                     |
| Alerts       | `GET /api/alerts`, `POST /api/alerts/:id/acknowledge`, `GET/POST /api/alert-rules`, `GET/PUT /api/alert-rules/:id` |
| Feature flags| (see `packages/server/src/routes/`)                                     |
| Settings     | `GET /api/settings`, `GET/PUT /api/settings/:key`                        |
| Chaos        | `POST /api/admin/chaos/inject`, `GET /api/admin/chaos/list`, `POST /api/admin/chaos/clear` |

---

## Examples

### CLI: Health check + first capability

```bash
curl -s http://localhost:8080/api/health | jq
curl -s -X POST http://localhost:8080/api/goals \
  -H 'content-type: application/json' \
  -d '{"description":"summarise a URL into 3 bullet points"}' | jq
```

### TypeScript client (Next.js side)

```typescript
import { goalApi, releaseApi } from '@/lib/api';

const goal = await goalApi.create({
  description: 'summarise a URL into 3 bullet points',
});
const canary = await releaseApi.canary(goal.firstReleaseId, { percent: 10 });
```

### Driving the planner directly

```bash
curl -s -X POST http://localhost:8080/api/ideas/plan \
  -H 'content-type: application/json' \
  -d '{"idea":"classify support tickets by urgency"}' | jq
```

---

## Project Structure

```
promptsheon/
├── packages/
│   ├── shared/                  # Domain types, Zod schemas, CAS, migrations, SSE
│   │   ├── src/
│   │   │   ├── types/           # 25 type modules (release, capability, eval, …)
│   │   │   ├── validation.ts    # Zod schemas for every external input
│   │   │   ├── cas/             # Content-addressed store
│   │   │   └── db-migrate.ts    # 25 SQL migrations
│   │   └── db/migrations/       # 001_core_schema … 026_eval_scorer_results
│   └── server/                  # Fastify + Strands agents + observability + hardening
│       ├── src/
│       │   ├── routes/          # 31 route modules, 76 REST endpoints
│       │   ├── repos/           # 22 SQL repos (release, capability, eval, …)
│       │   ├── agents/          # 6 Strands subsystems (planner, executor, compiler, evaluation, evolution, invocation)
│       │   ├── observability/   # OpenTelemetry, Pino, hooks
│       │   └── config/          # env parsing + Zod validation
│       └── test/                # 44 Vitest files (290 tests)
├── frontend/                    # Next.js 16 App Router + shadcn/ui + Tailwind v4
│   └── src/
│       ├── app/                 # 27 routes (App Router)
│       ├── components/
│       │   ├── ui/              # 17 shadcn primitives
│       │   └── dag/             # CapabilityNode, DagCanvas, NodeConfigPanel
│       └── lib/                 # api client, types, hooks
├── .github/workflows/ci.yaml    # typecheck + test + build (Node 26)
├── AGENTS.md                    # Engineering constitution (read first)
├── LICENSE                      # Apache-2.0
└── NOTICE                       # Attribution
```

---

## Development

```bash
pnpm install                # install all workspace deps
pnpm --dir packages dev     # both server + frontend (concurrently)
pnpm --dir packages build   # tsc all packages + next build the frontend
pnpm --dir packages typecheck
```

### Server development

```bash
cd packages/server
pnpm dev      # Fastify + tsx watch, port 8080
pnpm test     # Vitest 44 files / 290 tests
pnpm build    # tsc → dist/
pnpm start    # node dist/index.js
```

### Frontend development

The `frontend/` workspace is a [Next.js 16](https://nextjs.org/docs)
App Router app using [shadcn/ui](https://ui.shadcn.com),
[Tailwind v4](https://tailwindcss.com), and
[TanStack React Query](https://tanstack.com/query). It expects the
Fastify backend at `http://localhost:8080` (override via
`NEXT_PUBLIC_API_BASE_URL`).

```bash
cd frontend
pnpm install
pnpm dev        # http://localhost:3000 (Turbopack)
pnpm build      # production build
pnpm typecheck
```

Adding a page follows the `page.tsx` (server) + `*-client.tsx`
(client) split, with the client using `useSearchParams()` and React
Query against the API client in `src/lib/api.ts`. UI primitives are
installed via the shadcn CLI (`pnpm dlx shadcn@latest add <name>`)
and live in `src/components/ui/`.

### Shared package

```bash
cd packages/shared
pnpm build          # tsc → dist/ + copies db/migrations to dist/db
pnpm typecheck
```

### Code Style

- Line length: 100 (TypeScript default)
- Quotes: double (`"`)
- Type hints: required on all public signatures (`strict` TypeScript)
- All HTTP bodies validated by Zod before reaching handlers
- All SQL goes through a repo — no raw SQL in route handlers
- No dead code, no commented-out code
- The constitution lives in [`AGENTS.md`](AGENTS.md) — read it first

### Commit Conventions

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add canary-percent setter on release route
fix: clamp audit-chain hash to 32 bytes
docs: document PROMPTSHEON_WEBHOOK_SECRET
refactor: extract capability-version repo
test: add adversarial eval fixtures
chore: bump @strands-agents/sdk to 1.14
```

---

## Testing

```bash
cd packages/server && pnpm test    # 44 files / 290 tests
cd packages && pnpm typecheck      # shared + server + frontend
```

Shared package ships migration tests (`migration-022.test.ts`,
`migration-025.test.ts`) that exercise each SQL migration against a
fresh in-memory SQLite.

---

## Build

```bash
cd packages && pnpm build
# 1. pnpm --dir shared build      → packages/shared/dist + dist/db
# 2. pnpm --dir server build      → packages/server/dist (executable)
# 3. (cd ../frontend && pnpm build)→ frontend/.next (static + dynamic routes)
```

Then run production:

```bash
node packages/server/dist/index.js
# serves API on :8080 and static frontend from PROMPTSHEON_FRONTEND_PATH
```

---

## Release

This repo is not yet published to npm. There is no Docker image
shipped yet (see [Roadmap](#roadmap)). To run a "release":

1. Bump `version` in `packages/server/package.json`,
   `packages/shared/package.json`, `frontend/package.json`
2. Update `CHANGELOG.md` (if you start one)
3. Commit with `chore: release vX.Y.Z`
4. Tag and push — CI runs typecheck + test + build

---

## Deployment

There is no first-class deployment target yet. The intended production
shape is:

- Run `pnpm build` on a build host
- Copy `packages/server/dist`, `packages/shared/dist`, `frontend/.next`,
  plus the SQLite file and CAS directory to the target
- Run `node packages/server/dist/index.js` behind a reverse proxy
- Provide an LLM API key via env

Docker packaging is on the roadmap (see below).

---

## Observability

- **Logs** — structured Pino, configurable level, error stream
- **Traces + metrics** — OpenTelemetry, OTLP exporter, configurable
  endpoint (`PROMPTSHEON_OTEL_ENDPOINT`)
- **Live events** — Server-Sent Events at `/api/events/:channel` for
  execution progress, release activations, audit events
- **Audit chain** — append-only, hash-linked, with a `/api/audit/verify`
  endpoint that proves no row has been mutated

---

## Tech Stack

| Category       | Technology                                       |
|----------------|--------------------------------------------------|
| Runtime        | Node.js ≥ 26                                     |
| Language       | TypeScript (strict mode)                         |
| Package mgr    | pnpm 11 (workspaces)                             |
| HTTP           | [Fastify 5](https://fastify.dev)                 |
| Validation     | [Zod 4](https://zod.dev)                         |
| Database       | SQLite via [better-sqlite3](https://github.com/WiseLibs/better-sqlite3) (25 migrations) |
| AI             | [`@strands-agents/sdk`](https://github.com/strands-agents/sdk) — `Agent`, `Swarm`, `Graph`, hooks, retries |
| LLM providers  | OpenAI, Anthropic, AWS Bedrock                   |
| Observability  | [OpenTelemetry](https://opentelemetry.io), [Pino](https://getpino.io) |
| Rate limit     | [@fastify/rate-limit](https://github.com/fastify/fastify-rate-limit) |
| CORS           | [@fastify/cors](https://github.com/fastify/fastify-cors) |
| Tests          | [Vitest 4](https://vitest.dev) (44 files / 290 tests) |
| Frontend       | [Next.js 16](https://nextjs.org) (App Router, Turbopack) |
| UI             | [React 19](https://react.dev), [shadcn/ui](https://ui.shadcn.com) (17 primitives), [Tailwind v4](https://tailwindcss.com) |
| Data fetching  | [TanStack React Query v5](https://tanstack.com/query) |
| DAG editor     | [@xyflow/react](https://reactflow.dev) v12        |
| Icons          | [lucide-react](https://lucide.dev)               |
| Hashing        | Node `crypto` (audit chain, manifest CAS, HMAC)  |
| Container      | Docker — planned, not yet shipped                |
| Package        | npm — not planned (self-hosted product)          |

---

## Roadmap

- **v0.1.0** (shipped) — Fastify + Strands backend, Next.js 16
  frontend, 76 REST endpoints, 22 repos, 6 Strands subsystems (Swarm +
  Graph), DAG editor, releases + canary, maker-checker approvals, audit
  chain, eval suites, self-evolution loop, webhooks + replay
  protection, chaos hooks, OpenTelemetry, 25 SQLite migrations, 290
  tests.
- **v0.2.0** (next) — Docker packaging (`Dockerfile` + multi-stage
  build), production deployment guide, RBAC refinement on the maker-
  checker flow, dataset import/export.
- **Backlog** — gRPC interface alongside HTTP, multi-tenant SSO,
  Postgres adapter behind the better-sqlite3 repo layer, Helm chart for
  Kubernetes, OpenAPI → typed client codegen.

Have an idea? [Open a feature request](https://github.com/sachncs/promptsheon/issues/new).

---

## Contributing

Contributions are welcome. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the process, coding standards,
and Conventional Commits workflow. Bug reports and feature requests use
the [issue templates](.github/ISSUE_TEMPLATE/).

The full engineering standards — type safety, validation, repo layout,
testing bar, naming, lifecycle — are codified in
[`AGENTS.md`](AGENTS.md). Read it before opening a PR.

## Code of Conduct

This project follows the
[Contributor Covenant v2.1](CODE_OF_CONDUCT.md). By participating, you
are expected to uphold that standard. Report unacceptable behaviour to
**sachncs@gmail.com**.

## Security

Please do **not** file security vulnerabilities as public GitHub
issues. See [`SECURITY.md`](SECURITY.md) for the disclosure policy.
Contact: **sachncs@gmail.com**.

## License

[Apache-2.0](LICENSE) © 2026 Sachin — **sachncs@gmail.com**.