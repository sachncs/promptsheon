# ADR-0001: Production package boundaries

- Status: accepted
- Date: 2026-09-29

## Context

Promptsheon contains a Fastify API, application services, SQLite
repositories, agent adapters, a Next.js console, a CLI, an SDK, and a public
documentation site. Production reliability depends on keeping transport,
workflow decisions, domain contracts, and infrastructure concerns separate.

## Decision

The repository uses these boundaries:

```text
HTTP / UI / CLI / SDK adapters
              ↓
       application use cases
              ↓
       shared domain contracts
              ↓
       infrastructure ports and adapters
```

- `packages/shared` owns contracts, validation, migrations, and pure logic.
- `packages/server/src/application` owns workflow decisions and orchestration.
- `packages/server/src/routes` owns HTTP parsing, authentication wiring, and
  response translation; routes do not contain repository queries for workflows.
- `packages/server/src/repos` owns prepared SQLite access and data scoping.
- `packages/server/src/infrastructure` owns concrete system and storage probes.
- `packages/sdk` and `packages/cli` expose supported integration surfaces.
- `frontend` owns presentation and client-side server-state orchestration.
- `site` owns public product positioning and developer documentation.

Dependencies point inward. Concrete adapters are constructed in the server
composition root and injected into route and application boundaries.

## Consequences

This makes use cases testable without HTTP, allows infrastructure adapters to
change without changing domain contracts, and gives the platform a controlled
path toward queues, external databases, and multiple provider adapters.

It also requires explicit dependency injection and occasional wiring changes
when a new application service is introduced. That cost is intentional: hidden
construction and route-level business logic make failure isolation and
throughput work harder.

## Rejected alternatives

- A route-first architecture was rejected because it couples validation,
  persistence, and workflow decisions to Fastify.
- A single shared service container was rejected because mutable global state
  hides lifecycle and makes tests order-dependent.
- Direct repository access from frontend code was rejected because it bypasses
  API authorization and contract validation.

