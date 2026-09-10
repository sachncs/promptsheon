---
layout: page
title: Contributing
subtitle: How to set up a dev environment, file issues, and submit changes.
---

# Contributing

Promptsheon is a TypeScript codebase (Fastify 5 + better-sqlite3 backend, Next.js 16 + React 19 frontend) organised as a pnpm workspace. This page covers the practical workflow. The engineering standards — type safety, validation, naming, testing bar, lifecycle, repository conventions — live in [`AGENTS.md`](https://github.com/sachncs/promptsheon/blob/master/AGENTS.md). Read AGENTS.md before opening your first PR.

## Prerequisites

- **Node.js 26 or newer.** Check with `node --version`.
- **pnpm 11.** `corepack enable && corepack prepare pnpm@11 --activate`.
- **Git.**

## Local setup

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
pnpm install
cp .env.example .env
$EDITOR .env
pnpm dev
```

`pnpm dev` launches the backend (`:8080`) and the frontend (`:3000`) together with hot reload. The frontend's Next.js config rewrites `/api/*` to the backend automatically.

## Pre-PR checklist

Run every step before pushing:

```bash
pnpm typecheck                                # tsc --noEmit everywhere
pnpm --dir packages/shared test                # vitest, shared package
pnpm --dir packages/server test                # vitest, server package
pnpm --dir frontend test:e2e                   # Playwright tier suite
pnpm --dir frontend build                      # next build
```

All four steps must pass locally before you push. The CI workflow re-runs the same checks on every PR.

## Pull request process

1. Branch from `master` with a Conventional Commits prefix: `feat/<slug>`, `fix/<slug>`, `docs/<slug>`, `refactor/<slug>`, `test/<slug>`, `chore/<slug>`.
2. Make your changes in atomic commits. Each commit should represent one logical change.
3. Write or update tests:
   - Backend changes require vitest cases in the relevant `packages/*/test/` file.
   - Frontend behaviour changes require a Playwright spec in `frontend/tests/`.
4. Push your branch and open a PR against `master`. Fill out the [PR template](https://github.com/sachncs/promptsheon/blob/master/.github/PULL_REQUEST_TEMPLATE.md).
5. Wait for CI to pass. Every PR that touches a path listed in [`CODEOWNERS`](https://github.com/sachncs/promptsheon/blob/master/CODEOWNERS) needs an owner approval before merge.

## Style guidelines

Promptsheon follows the standards codified in [`AGENTS.md`](https://github.com/sachncs/promptsheon/blob/master/AGENTS.md). In summary:

- **Type safety** — `strict: true`, `noImplicitAny: true`, `strictNullChecks: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`. No `any` in production code.
- **Validation** — Zod for every external input boundary. Schemas live in `packages/shared/src/validation.ts`.
- **Database** — `better-sqlite3` prepared statements through the repo layer (`packages/server/src/repos/`). No raw SQL in routes.
- **AI / LLM** — every LLM call goes through `@strands-agents/sdk`; agents live in `packages/server/src/agents/`.
- **HTTP** — all routes use Fastify; errors return the `{ error: { code, message } }` shape.
- **Documentation** — every exported identifier carries a TSDoc comment explaining purpose, behaviour, and constraints.

## Reporting issues

When you file a bug report, include:

- Node.js version (`node --version`) and pnpm version (`pnpm --version`).
- Operating system and architecture.
- Steps to reproduce.
- Expected behaviour.
- Actual behaviour (with the relevant log line or stack trace).
- The package the issue lives in.

For security vulnerabilities, **do not** open a public GitHub issue. Follow the disclosure process in [`SECURITY.md`](https://github.com/sachncs/promptsheon/blob/master/SECURITY.md).

## Code of conduct

This project follows the [Contributor Covenant v2.1](https://github.com/sachncs/promptsheon/blob/master/CODE_OF_CONDUCT.md). By participating, you are expected to uphold that standard.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Read the engineering standards | [`AGENTS.md`](https://github.com/sachncs/promptsheon/blob/master/AGENTS.md) |
| Solve a specific problem | [FAQ]({{ '/faq/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
