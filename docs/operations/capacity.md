# Capacity and SLO baseline

This document is the deployment review baseline for a single-node Promptsheon
installation using SQLite. It records measured engineering signals and bounded
limits; it is not a universal production capacity claim.

## Service objectives to validate

Before accepting a deployment profile, record the workload, host/runtime,
tenant distribution, and observation window alongside these starting targets:

| Signal | Starting target | Evidence source |
| --- | ---: | --- |
| API availability | ≥ 99.5% monthly | `/api/ready` probes |
| Read-only API p95 | < 250 ms | route/request metrics |
| Mutation API p95 | < 750 ms excluding provider calls | route/request metrics |
| Execution admission p95 | < 500 ms | enqueue timestamp to queued state |
| Queue wait p95 | < 2 s under designed load | queue age and worker metrics |
| Acknowledged evidence loss | 0 records | evidence reconciliation |
| Rollback completion | < 60 s after decision | release transition audit events |

These values become accepted SLOs only after a representative load/soak run is
attached to the release review. Provider latency and network distance are
outside the local deterministic benchmark.

## Bounded controls

The default controls below prevent unbounded growth. Change them only with a
capacity test and an operational owner:

| Resource | Default bound | Observable signal |
| --- | ---: | --- |
| Pending execution jobs per organization | 10,000 | `QUEUE_CAPACITY_EXCEEDED`, queue metrics |
| Durable worker concurrency | 4 per process | running queue count |
| Per-organization execution concurrency | 2 per process | queue age and running count |
| Execution attempts | 3 by default, 1–10 per request | job attempts/state |
| Provider operations | 8 per provider | provider gate wait/error metrics |
| Tool operations | 16 per provider/tool pair | tool gate wait/error metrics |
| Evidence buffer | bounded; critical events bypass drops | sink metrics and logs |
| Evidence query page | 1–500 records | cursor `before` |
| Request rate limit | 100 requests/minute by default | `429` responses and headers |

## Repeatable credential-free benchmarks

Run both profiles from a clean checkout. Neither calls an LLM provider:

```bash
pnpm benchmark:baseline
PROMPTSHEON_BENCHMARK_JOBS=10000 \
PROMPTSHEON_BENCHMARK_ORGANIZATIONS=16 \
pnpm benchmark:execution
```

The execution benchmark must report `exactlyOnce: true` and
`remainingQueue: 0`. Preserve its JSON output with the host/runtime and queue
configuration in the deployment review. The benchmark exercises the real
SQLite queue repository, not a mock.

## Scaling and degradation decisions

- Increase worker concurrency only when queue age is high while provider and
  SQLite contention remain within their tested limits.
- Reduce admission or return `429` when an organization reaches its pending
  queue limit; do not silently discard accepted work.
- Preserve execution failures, cancellations, guardrail decisions, permission
  decisions, and observed errors when evidence backpressure occurs.
- Treat provider outages as bounded retries/circuit-breaker events; do not
  increase queue capacity to hide a failing dependency.
- Move to an external queue/database before claiming multi-node SQLite
  capacity. The current adapter boundary is single-node by design.

## Review record

Attach benchmark output, CI run, migration version, backup verification, CAS
verification, and the accepted risks to the release or incident record. A
green unit or browser test alone is not evidence of production capacity.
