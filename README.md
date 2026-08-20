# Promptsheon (TypeScript)

TypeScript rewrite of the Promptsheon prompt management platform. Strands Agents SDK for all AI operations, Fastify for HTTP, better-sqlite3 for storage, React for the UI.

## Architecture

```
                    ┌─────────────────┐
                    │  React (web)    │
                    │  + shadcn/ui    │
                    └────────┬────────┘
                             │ HTTPS
                    ┌────────▼────────┐
                    │  Fastify (api)  │
                    │  + Zod          │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐
        │  SQLite   │  │   CAS     │  │  Strands  │
        │  + repos  │  │  store    │  │  Agents   │
        └───────────┘  └───────────┘  └───────────┘
```

## Packages

- `packages/shared/` — Domain types, Zod schemas, CAS store, error handling
- `packages/server/` — Fastify backend with 20 repos, 6 Strands agents (incl. Swarm + Graph), 30+ route groups
- `frontend/` — Next.js 15 App Router with 26 routes, 16 shadcn/ui primitives, full DAG editor

## Quick Start

```bash
# 1. Install all workspace dependencies
pnpm install

# 2. Launch both servers in one command
cd packages && pnpm dev
# server  → http://localhost:8080 (Fastify)
# frontend → http://localhost:3000 (Next.js 16 + Turbopack)

# Or run them separately:
cd packages && pnpm dev:server    # API only
cd packages && pnpm dev:frontend  # Web only

# 3. Open http://localhost:3000
```

The frontend `next.config.ts` rewrites `/api/*` → `http://localhost:8080/api/*`, so you only need both servers running.

### Production mode

```bash
cd packages/server && pnpm build && pnpm start
cd frontend && pnpm build && pnpm start
```

## Tests

```bash
# Server tests (44 files, 290 tests)
cd packages/server && pnpm test

# Typecheck all packages
cd packages && pnpm typecheck
```

## Documentation

- [`packages/shared/README.md`](packages/shared/README.md)
- [`packages/server/README.md`](packages/server/README.md)
- [`packages/server/API.md`](packages/server/API.md) — 80+ REST endpoints

## Stack

- **Backend**: Node 22, Fastify 5, better-sqlite3, Zod 4, Strands Agents SDK 1.13
- **Frontend**: Next.js 15 (App Router), React 19, Tailwind CSS v4, shadcn/ui, TanStack Query v5, @xyflow/react
- **AI**: `@strands-agents/sdk` (Agent, BedrockModel, Swarm, Graph, hooks, retry strategies)
- **Database**: SQLite 26 migrations
- **Tests**: vitest 4 (44 test files, 290 tests)

## Project Structure

```
.
├── packages/
│   ├── shared/        # domain types, Zod schemas, CAS, errors, audit, SSE
│   └── server/        # Fastify + Strands agents + observability + hardening
├── frontend/          # Next.js 15 App Router + shadcn/ui
├── AGENTS.md          # engineering constitution
├── Dockerfile         # multi-stage Node 22 build
├── .github/workflows/ # CI (typecheck + test + build + docker)
└── README.md          # this file
```

## License

See [`LICENSE`](LICENSE).
