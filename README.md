# Promptsheon (TypeScript Port)

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
- `packages/server/` — Fastify backend with 29 repos, 4 Strands agents, 17 route groups
- `packages/web/` — React 19 frontend with 25 views, 15 UI components, 22 API modules

## Quick Start

```bash
# Install
cd packages && pnpm install

# Start server (port 8080)
cd server && npm run dev

# Start web (port 5173)
cd web && npm run dev
```

## Tests

```bash
cd packages/server && npm test
# 5 files, 23 tests, all passing
```

## Documentation

- [`packages/shared/README.md`](packages/shared/README.md)
- [`packages/server/README.md`](packages/server/README.md)
- [`packages/server/API.md`](packages/server/API.md)
- [`packages/web/README.md`](packages/web/README.md)
- [`plan/tier/`](plan/tier/) — 18-work-unit implementation plan
