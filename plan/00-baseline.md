# Phase 0 — Production baseline

## Objective

Create a trustworthy starting point. Remove unsupported claims and unstable
surfaces before adding new platform behaviour.

## Responsible

- Engineering lead: scope and acceptance criteria.
- Backend lead: API and package inventory.
- Frontend lead: product-flow inventory.
- SRE/security: CI, deployment, threat, and dependency baseline.

## Implementation

- Inventory every route, use case, repository, migration, UI route, CLI command,
  SDK method, job, and external provider adapter.
- Classify each capability as supported, incomplete, experimental, deprecated,
  or removed.
- Remove dead routes, placeholder responses, fake integrations, and claims not
  backed by working code.
- Freeze the runtime and package-manager baseline through `.nvmrc`, engines,
  lockfile, CI, and Docker.
- Establish package boundaries:
  - `shared`: contracts, validation, migrations, pure domain logic.
  - `server`: application services, routes, repositories, agents, adapters.
  - `sdk` and `cli`: public integration surfaces.
  - `frontend`: product console.
  - `site`: public product and developer documentation.
- Add a CI matrix for typecheck, lint, unit tests, integration tests, E2E,
  site build, container build, and dependency policy checks.
- Define API versioning, migration, deprecation, and compatibility policy.
- Establish an architectural decision record for every irreversible choice.

## Reliability and throughput

- Set initial service objectives for API availability, error rate, p95 latency,
  execution start latency, and queue wait time.
- Add health and readiness checks that distinguish process health from dependency
  and storage readiness.
- Set request body, execution, database, and provider timeout limits.
- Record a baseline for test duration, build duration, API throughput, and
  SQLite write contention.
- Identify all unbounded loops, retries, payloads, queues, and in-memory state.

## Testing

- Run the complete existing test suite and record the baseline.
- Add contract tests for every public API route.
- Add smoke tests for local startup, health, readiness, frontend, and public site.
- Add dependency installation with `pnpm install --frozen-lockfile` to CI.
- Add a clean checkout verification job.
- Add static checks for unsupported product claims and stale links in docs.
- Add a failure budget for flaky tests; flaky tests must be quarantined with an
  owner and issue, never silently retried forever.

## Deliverables

- Supported capability matrix.
- Architecture and API compatibility policy.
- CI quality gates.
- Baseline reliability and throughput report.
- Removal list for unsupported functionality.

## Exit criteria

- A clean checkout installs and builds deterministically.
- CI blocks regressions across all supported surfaces.
- The README, public site, API docs, and UI agree on what exists.
- Every known production risk has an owner and a follow-up phase.

