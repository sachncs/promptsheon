# Durable execution operations

Durable execution jobs are stored in SQLite and addressed by an immutable
agent-specification hash. Submit work under a workspace:

```http
POST /api/workspaces/{workspaceId}/execution-jobs
Idempotency-Key: request-123
```

The JSON body contains `agentHash`, `inputs`, `idempotencyKey`, and an optional
`maxAttempts` between 1 and 10. The API returns `202` with a queued job. Repeat
the same request key with the same hash and input to receive the original job;
reuse it with different content returns `409`.

Observe or cancel a job with the workspace-scoped GET and cancel endpoints. The
job lifecycle is `queued → running → completed|failed|cancelled|timed-out`.
Workers claim leases atomically. An expired lease is requeued while attempts
remain and is failed after the configured maximum. Process shutdown aborts
active handlers and requeues them, so a restart does not silently lose work.

Each worker enforces global and per-organization concurrency limits, bounded
retry backoff with jitter, input/token/cost/wall-clock budgets, and an
`AbortSignal` propagated to the execution adapter. Queue depth, state counts,
and oldest queued time are available from `GET /api/execution-jobs/metrics`.

Multi-step DAG executions persist one checkpoint per node. A recovered job
loads completed checkpoints by execution ID and skips those nodes, avoiding
duplicate completed work after a crash. Checkpoint output is retained for
recovery and should be treated as execution data by storage and retention
operations.

Operational invariants:

- Never manually edit a job state in SQLite; use the repository transition API.
- A worker lease must be shorter than the process supervisor’s shutdown grace
  period or recovery will be delayed.
- Increasing concurrency requires observing queue age, SQLite contention, model
  provider saturation, and budget-exhaustion rates together.
- Provider/tool adapters should wrap calls in the shared `CircuitBreaker` and
  classify failures before handing them to the worker retry policy.
