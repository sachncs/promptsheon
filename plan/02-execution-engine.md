# Phase 2 — Durable execution engine

## Objective

Execute an immutable agent specification predictably, safely, and with bounded
resource usage.

## Responsible

- Runtime/backend: execution state machine and orchestration.
- Provider integrations: model adapters, retries, rate limits, error mapping.
- Tooling/backend: tool registry, permissions, sandbox boundaries.
- Platform/SRE: worker lifecycle, queues, cancellation, capacity.
- Security: authorization and data-access enforcement.

## Implementation

- Define execution states: queued, running, completed, failed, cancelled,
  timed-out, and partially-completed where applicable.
- Create an execution record referencing the agent hash and input hash.
- Implement a durable work queue or explicit worker abstraction.
- Add idempotency keys and deduplicate repeated requests.
- Enforce execution, step, provider, tool, token, cost, and wall-clock budgets.
- Implement `AbortController` cancellation through every long-running adapter.
- Implement bounded retries with backoff and retry classification.
- Add provider-neutral model and tool interfaces.
- Add DAG scheduling with bounded parallelism and dependency handling.
- Persist checkpoints for recoverable multi-step executions.
- Add safe process shutdown and requeue/recovery for interrupted work.
- Keep route handlers thin; application services own execution decisions.

## Reliability and throughput

- Use a bounded queue to provide backpressure instead of unlimited in-memory
  work.
- Limit per-tenant concurrency and provider concurrency.
- Separate interactive requests from long-running background executions.
- Avoid holding SQLite transactions across network calls.
- Batch independent metadata writes where safe.
- Add circuit breakers for failing providers and tools.
- Apply jitter to retries to prevent synchronized load spikes.
- Track queue depth, queue age, active workers, provider saturation, and budget
  exhaustion.
- Make all state transitions atomic and idempotent.

## Testing

- State-machine tests for every legal and illegal transition.
- Unit tests for retry classification, budgets, timeouts, cancellation, and
  backoff.
- Integration tests with deterministic fake model and tool adapters.
- Crash-recovery tests at every checkpoint boundary.
- Duplicate-request and idempotency tests.
- Authorization tests for every tool and organization boundary.
- DAG tests for ordering, parallelism, failure propagation, and cancellation.
- Load tests for queue throughput, worker saturation, and SQLite contention.
- Provider contract tests with timeout, rate-limit, malformed-response, and
  partial-failure fixtures.

## Deliverables

- Durable execution service.
- Worker and queue abstraction.
- Provider and tool ports.
- Execution status and cancellation APIs.
- Recovery and operational runbook.

## Exit criteria

- A run can be started, observed, cancelled, retried, recovered, and completed.
- Every run references one immutable agent hash.
- Resource limits are enforced under load.
- Restarting the service does not silently lose or duplicate work.

## Implementation status

Implemented in the current Phase 2 slice:

- Durable SQLite execution jobs with explicit lifecycle transitions.
- Idempotent enqueue, atomic leases, bounded retries, jitter, cancellation,
  recovery, graceful shutdown, and organization concurrency limits.
- Workspace-scoped execution API and queue metrics.
- Immutable agent-hash loading through the Phase 1 specification repository.
- Execution budget checks, provider/tool ports, DAG dependency scheduling with
  bounded parallelism, and durable per-step checkpoints/resume.
- Provider-neutral router adapter injection for durable tool-free nodes, with
  validated response normalization and abort propagation.
- Tool-bearing nodes honor their immutable provider/model policy when building
  the Strands model, rather than silently using global defaults.
- Registered Strands tools are filtered through the specification allowlist and
  denied at the pre-tool hook boundary. Durable execution now injects a
  process-scoped tool registry plus an execution-scoped authorizer that also
  enforces organization, execution, and immutable specification allowlists.
- Queue throughput coverage exercises 250 durable claims without duplicate
  ownership, and a two-worker SQLite contention regression covers shared queue
  ownership under concurrent polling.
- Provider and registered-tool calls are bounded by cancellation-aware
  concurrency gates and independently protected by circuit breakers.
- The production composition root now registers deterministic, network-free
  `json.parse` and `text.length` adapters. They remain inaccessible unless a
  manifest declares them and includes them in `metadata.allowedTools`, so the
  default tool surface is useful for simulator workflows without weakening
  authorization boundaries.
- The DAG editor exposes these built-ins through node configuration and keeps
  the node tool list and `metadata.allowedTools` synchronized.

Remaining exit-gate work is limited to adding product-specific ToolAdapters
when product capabilities define them, plus final full-repository verification.
The platform boundary is now injectable and covered by an adapter-backed tool
integration test; no domain-specific tool implementation was defined by this
phase. Circuit breakers now protect the gateway provider path and durable
provider/tool invocations.
