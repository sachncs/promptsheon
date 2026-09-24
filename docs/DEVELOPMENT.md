# Engineering Guide

This document is the working guide for developing, reviewing, testing, and
operating Promptsheon. It complements `AGENTS.md`, which is the repository's
normative TypeScript engineering constitution.

## Engineering position

Promptsheon is a self-hosted TypeScript monorepo. Production quality means
more than a successful request: boundaries must be explicit, tenant access
must be enforced at the persistence boundary, failures must be observable,
and critical workflows must be covered by automated tests.

This project does not promise backward compatibility for internal APIs,
database shapes, or transitional abstractions. A breaking change is
acceptable when it removes legacy baggage and produces a simpler, safer
architecture. Every such change must update callers, migrations, tests, and
documentation in the same change set.

## Local development

### Prerequisites

- Node.js 22 or newer (the CI image and package engine constraint use this
  production baseline).
- pnpm 11, enabled through Corepack.
- An LLM provider credential for workflows that invoke an agent. Route and
  repository tests do not require a live provider.

```bash
corepack enable
corepack prepare pnpm@11 --activate
pnpm install
cp .env.example .env
```

Review `.env.example` before starting the server. Keep secrets in the local
environment or a secret manager; never commit them, put them in browser code,
or print them in logs.

### Run the environment

```bash
pnpm dev              # Fastify on :8080 and Next.js on :3000
pnpm dev:server       # backend only
pnpm dev:frontend     # frontend only
```

The frontend proxies `/api/*` to the backend. The backend applies migrations
at startup and exposes `/api/health` for liveness and `/api/ready` for
readiness. Readiness should be used by deployment orchestration because it
includes database and startup dependency checks.

### Useful checks

```bash
pnpm typecheck
pnpm test
pnpm --dir packages/server test
pnpm --dir frontend typecheck
pnpm --dir frontend test:e2e
pnpm --dir frontend build
```

Use focused commands while iterating, then run the complete matrix before
opening a pull request.

## Repository structure and boundaries

```text
packages/shared/       domain contracts, validation, migrations, pure logic
packages/server/
  src/application/     use-case orchestration and ports
  src/repos/           SQLite persistence adapters
  src/infrastructure/  concrete runtime adapters (SQLite probes, providers)
  src/routes/          HTTP adapters: parse, authorize, call application code
  src/agents/           LLM/agent adapters
  src/middleware/      request identity and organization context
  src/observability/   logs, metrics, traces, lifecycle instrumentation
frontend/src/app/      Next.js route composition and page UX
frontend/src/components/ reusable UI and domain presentation components
frontend/src/lib/      typed API client, session, and browser infrastructure
frontend/tests/        Playwright end-to-end tests
packages/cli/          command-line adapter
packages/sdk/          public programmatic client
```

The intended dependency direction is:

```text
HTTP/UI adapters -> application use cases -> domain contracts
                         |                 ^
                         v                 |
                    ports/interfaces <- infrastructure adapters
```

Routes do not contain business rules or SQL. They validate external input
with Zod, obtain the authenticated organization context, call a use case or
repository method, and translate the result into the HTTP error contract.
Repositories own prepared SQL, row mapping, pagination, and organization
scoping. Agents are infrastructure adapters and are injected into application
services; domain code must not construct an SDK client or read environment
variables directly.

Application services own workflow decisions and depend on small ports. Concrete
adapters are assembled once in the composition root (`routes/index.ts`); for
example, `HealthService` depends on `HealthProbe`, while
`SqliteHealthProbe` provides the SQLite implementation. This keeps transport
tests and use-case tests independent of database drivers and makes replacement
adapters explicit rather than implicit.

The frontend follows the same separation: pages compose queries and states,
the API module owns HTTP behavior, shared components own presentation, and
server responses are treated as `unknown` until normalized at the boundary.
Raw `fetch` is reserved for the SSE stream path; ordinary data access uses
TanStack Query through the API client.

## How to add a module

1. Define the domain vocabulary and invariants in `packages/shared`.
2. Add the external-input schema in the shared validation module or at the
   adapter boundary when the schema is route-specific.
3. Define a small application port for behavior the use case needs.
4. Implement the SQLite or SDK adapter in the infrastructure package.
5. Add the use case in `packages/server/src/application` and inject its ports
   from the composition root.
6. Add the Fastify route only after the use case exists. The route must require
   organization context and must never interpolate user input into SQL.
7. Add unit tests for pure/domain behavior, repository tests for query and
   tenant boundaries, route tests for HTTP contracts, and an end-to-end test
   for user-visible critical paths.
8. Add the frontend query/mutation and explicit pending, empty, error, and
   success states. Reuse existing UI primitives instead of creating a one-off
   visual pattern.
9. Update API, architecture, configuration, and migration documentation in
   the same pull request.

Do not add a compatibility wrapper to avoid changing a caller. Update the
caller and remove the obsolete abstraction in the same atomic change.

## Coding standards

Follow Google TypeScript Style Guide conventions and the repository rules in
`AGENTS.md`:

- strict TypeScript; no `any` or assertion used to bypass validation;
- explicit, small interfaces and composition over inheritance;
- Zod validation for every untrusted HTTP, file, or provider boundary;
- prepared statements through repositories only;
- structured errors using `{ error: { code, message } }`;
- explicit lifecycle ownership and cleanup for timers, sockets, database
  handles, and SDK clients;
- TSDoc for exported APIs and comments that explain why, not what;
- no hidden global mutable state, module-level initialization, or secret logs.

## Testing strategy

| Layer | Purpose | Location / command |
| --- | --- | --- |
| Pure unit | deterministic domain, parser, scorer, and policy behavior | `packages/*/test`, `pnpm --dir packages/shared test` |
| Repository integration | SQL mapping, migrations, constraints, tenant isolation | `packages/server/test/*repo*.test.ts` |
| Route integration | validation, authorization, status/error contracts | `packages/server/test/routes`, `pnpm --dir packages/server test` |
| Frontend type/build | component and API contract correctness | `pnpm --dir frontend typecheck`, `pnpm --dir frontend build` |
| End-to-end | browser navigation and critical authenticated workflows | `frontend/tests`, `pnpm --dir frontend test:e2e` |

Every security or organization-boundary fix must include a regression test
that proves a foreign tenant cannot read or mutate the resource. Failure paths
are first-class behavior: test malformed input, missing context, authorization
denials, dependency failure, retries, idempotency, and partial results where
the feature supports them.

## Configuration and production operation

Configuration is loaded and validated at startup. Keep environment-specific
values outside source control and separate development, test, staging, and
production databases and CAS paths. Production deployments should:

- set authentication and an explicit organization context requirement;
- provide a strong secret through a secret manager, not `.env` in the image;
- use a durable database and CAS volume with backup and restore procedures;
- expose liveness and readiness separately to the orchestrator;
- export structured logs and OpenTelemetry telemetry, with request IDs carried
  through logs and traces;
- configure rate limits, CORS, outbound URL policy, retention, and allowed
  origins explicitly;
- run migrations as a controlled deployment step and verify readiness before
  routing traffic;
- capture audit-chain verification and database/CAS backup checks in the
  operational runbook.

See [`../configuration.md`](../configuration.md), [`../architecture.md`](../architecture.md), [`../SECURITY.md`](../SECURITY.md), and
`docs/operations/` for deployment-specific details.

## Debugging and troubleshooting

1. Check `/api/health` and `/api/ready` before debugging the UI.
2. Use the request ID from the structured server log to correlate a browser
   error with backend logs and traces.
3. Confirm the request has a valid authenticated identity and organization
   context. A missing context is an authorization failure, not an invitation
   to fall back to a global query.
4. Check migration output and the database path used by the running process.
5. Reproduce the smallest failing route with a focused Vitest test or a
   Playwright test before changing production code.
6. For LLM failures, verify provider configuration, vault references,
   timeout/retry settings, and sanitized provider error logs.

Do not disable authentication, tenant checks, SSRF protection, or audit
logging to make a local symptom disappear. Use a test fixture or an explicit
development configuration instead.

## OSS contribution workflow

Create a focused branch, make atomic Conventional Commits, and open a pull
request with the problem, design decision, tests, migration impact, and
operational impact. Keep unrelated formatting or generated files out of the
change. Security issues must follow `SECURITY.md` rather than a public issue.

Reviewers should check dependency direction, organization isolation, failure
semantics, observability, accessibility, and documentation—not only whether
the happy path works.

Breaking changes are expected to be explicit. Describe the new contract and
the migration or rollout procedure; do not preserve a deprecated API merely
to avoid updating internal callers.
