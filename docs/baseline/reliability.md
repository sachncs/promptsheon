# Reliability and throughput baseline

Baseline captured on 2026-09-29 on Node.js 26.8.1 and pnpm 11.23.0. These are
engineering measurements, not production SLO commitments.

## Current quality gates

| Gate | Result |
|---|---|
| Frozen dependency install | Passing locally and required in CI |
| Workspace typecheck | Passing |
| Shared/server tests | 105 server test files and 700 shared/server tests passing in the latest verified run |
| Frontend lint | Passing |
| Frontend production build | Passing |
| Public site check | Passing with zero diagnostics |
| Public site production build | Passing |
| Docker build and readiness smoke | Required by push CI |
| Browser smoke suite | Required by push CI |
| Documentation local-link check | Added in Phase 0 |

## Runtime baseline

- Node.js: `26.8.1` from `.nvmrc`.
- pnpm: `11.23.0` from the package-manager field.
- Server: Fastify with SQLite through prepared repository statements.
- Frontend: Next.js App Router with TanStack Query and Playwright.
- Public site: Astro static build.
- Persistence: SQLite database plus content-addressed storage.

## Initial reliability objectives

These targets are the starting point for load-test design. They must be
validated against the intended deployment profile before being advertised as
service-level objectives.

| Signal | Initial target | Measurement |
|---|---:|---|
| API availability | ≥ 99.5% monthly | Successful readiness and API probes |
| Non-LLM API error rate | < 1% | Structured HTTP error metrics |
| Read-only API p95 latency | < 250 ms | Route-level histogram |
| Mutation API p95 latency | < 750 ms excluding provider calls | Route-level histogram |
| Execution admission p95 | < 500 ms | Request to queued event |
| Queue wait p95 | < 2 s under designed load | Queue age histogram |
| Evidence write loss | 0 acknowledged records | Durable evidence reconciliation |
| Rollback completion | < 60 s after decision | Release transition telemetry |

## Known risks and owners

| Risk | Owner | Follow-up |
|---|---|---|
| SQLite write contention under concurrent executions | Runtime/backend + SRE | Phase 2 and Phase 9 load tests |
| Provider latency, rate limits, and outages | Provider/runtime owner | Phase 2 circuit breakers and budgets |
| Evaluation workload competing with production traffic | Platform owner | Phase 4 isolated workers and quotas |
| Mutation search fan-out or recursive loops | Agent systems owner | Phase 5 bounded search and ancestry checks |
| Evidence containing sensitive prompts or outputs | Security owner | Phase 3 redaction and retention tests |
| Public/API contract drift | DX owner | Phase 0 docs gate and Phase 8 executable examples |
| Deployment or migration rollback failure | SRE owner | Phase 9 restore and rollback drills |

