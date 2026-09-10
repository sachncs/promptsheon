# Contributing to Promptsheon

Thank you for your interest in contributing!

Promptsheon is a TypeScript codebase (Fastify 5 + better-sqlite3 backend,
Next.js 16 + React 19 frontend) organised as a pnpm workspace. This guide
covers the practical workflow; the engineering standards — type safety,
validation, naming, testing bar, lifecycle, repository conventions — live
in [AGENTS.md](AGENTS.md). Read AGENTS.md before opening your first PR.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting started](#getting-started)
- [Pull request process](#pull-request-process)
- [Style guidelines](#style-guidelines)
- [Reporting issues](#reporting-issues)
- [Questions](#questions)

## Code of Conduct

This project and everyone participating in it is governed by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## Getting started

1. Install the prerequisites:

   - **Node.js 26 or newer** — check with `node --version`.
   - **pnpm 11** — `corepack enable && corepack prepare pnpm@11 --activate`.
   - **Git** for the version-control workflow.

2. Fork and clone the repository:

   ```bash
   git clone https://github.com/sachncs/promptsheon.git
   cd promptsheon
   ```

3. Install dependencies for the entire workspace (one command resolves
   shared, server, cli, sdk, and frontend):

   ```bash
   pnpm install
   ```

4. Copy the environment template and fill in the values you need:

   ```bash
   cp .env.example .env
   $EDITOR .env   # at minimum: PROMPTSHEON_AUTH=false for local dev
   ```

5. Run the full local check before opening a PR:

   ```bash
   pnpm typecheck                                    # tsc --noEmit everywhere
   pnpm --dir packages/shared test                  # vitest, shared package
   pnpm --dir packages/server test                  # vitest, server package
   pnpm --dir frontend test:e2e                     # Playwright tier suite
   ```

   Or, from the repo root, the matching workspace selectors:

   ```bash
   pnpm -r typecheck
   pnpm -r --filter './packages/*' test
   ```

The full architecture, repo layout, package boundaries, and design
decisions are documented in [AGENTS.md](AGENTS.md) (sections
"Package Architecture" and "Repository-Specific Rules"). The
on-disk documentation under [docs/](docs/) covers operator-facing
material — SOC2, threat model, on-prem deployment, security
benchmark — not contributor onboarding.

## Pull request process

1. Branch from `master` using a Conventional Commits style prefix:
   `feat/<slug>`, `fix/<slug>`, `docs/<slug>`, `refactor/<slug>`,
   `test/<slug>`, `chore/<slug>`.

2. Make your changes in atomic commits. Each commit should represent
   one logical change that the reviewer can review in isolation.
   See [AGENTS.md](AGENTS.md) "Repository-Specific Rules" for the
   conventions this repo enforces.

3. Write or update tests. Backend changes require vitest cases in the
   relevant `packages/*/test/` file; frontend behaviour changes
   require a Playwright spec in `frontend/tests/`.

4. Run the full pre-PR checklist before pushing:

   ```bash
   pnpm -r typecheck
   pnpm --dir packages/shared test
   pnpm --dir packages/server test
   pnpm --dir frontend test:e2e
   ```

5. Push your branch and open a pull request against `master`. Fill
   out the [PR template](.github/PULL_REQUEST_TEMPLATE.md) completely.

6. Wait for CI to pass, then request a review. Every PR that touches
   a path listed in [CODEOWNERS](CODEOWNERS) needs an owner approval
   before merge.

## Style guidelines

Promptsheon follows the standards codified in [AGENTS.md](AGENTS.md).
In summary:

- **Type safety** — `strict: true`, `noImplicitAny: true`,
  `strictNullChecks: true`, `noUncheckedIndexedAccess: true`,
  `exactOptionalPropertyTypes: true`. No `any` in production code.
- **Validation** — `zod` for every external input boundary. Schemas
  live in `packages/shared/src/validation.ts`. Never use `as Type`
  casts to bypass a schema.
- **Database** — `better-sqlite3` prepared statements through the
  repo layer (`packages/server/src/repos/`). No raw SQL in routes.
- **AI / LLM** — every LLM call goes through `@strands-agents/sdk`;
  agents live in `packages/server/src/agents/`.
- **HTTP** — all routes use Fastify; errors return the
  `{ error: { code, message } }` shape.
- **Documentation** — every exported identifier carries a TSDoc
  comment explaining purpose, behaviour, and constraints.
- **Tests** — Vitest on the server and shared packages; Playwright
  on the frontend. Tests must pass before merge.

Before opening a PR, also run Prettier (`npx prettier --write .`) and
ESLint (`pnpm --dir frontend lint`) on the touched files. The local
pre-commit hooks (see [.pre-commit-config.yaml](.pre-commit-config.yaml))
enforce the same checks on staged changes.

## Reporting issues

Include:

- Node.js version (`node --version`) and pnpm version (`pnpm --version`).
- Operating system and architecture.
- Steps to reproduce the problem.
- Expected behaviour.
- Actual behaviour (with the relevant log line or stack trace).
- The package the issue lives in (`packages/server`, `packages/shared`,
  `packages/cli`, `packages/sdk`, or `frontend`).

For security vulnerabilities, **do not** open a public GitHub issue.
Follow the disclosure process in [SECURITY.md](SECURITY.md).

## Questions

- Open a [GitHub Discussion](https://github.com/sachncs/promptsheon/discussions)
- Check existing [issues](https://github.com/sachncs/promptsheon/issues)
- Read [AGENTS.md](AGENTS.md) for engineering conventions
- Read the operator docs under [docs/](docs/) for deployment, security,
  and compliance material
