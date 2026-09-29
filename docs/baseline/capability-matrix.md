# Supported capability matrix

Baseline captured on 2026-09-29 from the current repository and passing local
verification commands.

## Supported product surfaces

| Surface | Location | Baseline status | Verification |
|---|---|---|---|
| Shared contracts and validation | `packages/shared` | Supported | `pnpm --dir packages/shared build`, shared tests |
| Fastify HTTP API | `packages/server` | Supported with ongoing hardening | Server typecheck and 105-file test suite |
| SQLite repositories and migrations | `packages/server/src/repos`, `packages/shared/db/migrations` | Supported | Repository and migration tests |
| Agent planning/execution adapters | `packages/server/src/agents` | Supported with provider/runtime limits | Server tests and provider fixtures |
| TypeScript SDK | `packages/sdk` | Supported integration surface | SDK typecheck/build |
| Operator CLI | `packages/cli` | Supported integration surface | CLI typecheck/build/tests |
| Next.js console | `frontend` | Supported product console | Next build, lint, Playwright smoke tier |
| Public product site and docs | `site` | Supported public surface | Astro check and production build |
| Docker image | `Dockerfile` | Supported deployment surface | CI Docker build and health/readiness smoke |

## Explicitly bounded or incomplete

| Area | Current classification | Required follow-up |
|---|---|---|
| Autonomous improvement | Controlled capability, not autonomous production mutation | Phase 5 proposal/evaluation/approval workflow |
| SQLite scale-out | Single-node persistence baseline | Phase 9 capacity and adapter work |
| Provider behaviour | Adapter-dependent and externally variable | Phase 2 budgets, circuit breakers, and contract fixtures |
| Evaluation quality | Available but dependent on suite/scorer configuration | Phase 4 reproducibility and regression gates |
| Public API versioning | Unversioned `/api` contract with policy | Maintain `docs/API-COMPATIBILITY.md`; introduce versioning when required |
| Throughput targets | Baselines recorded, production SLOs not yet accepted | Phase 9 load and capacity validation |

## Removed from supported scope

- The experimental VS Code extension workspace.
- Direct browser access to server secrets.
- Route-level repository escape hatches for application workflows.

## Verification commands

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm test
pnpm --dir frontend lint
pnpm --dir frontend build
pnpm --dir site exec astro check
pnpm --dir site build
pnpm check:docs
```

