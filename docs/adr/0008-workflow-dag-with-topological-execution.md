# ADR 0008: Execute workflows as a topological DAG with level-by-level concurrency

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

An agent is a collection of steps. Some steps depend on others; some are independent. We need an execution model that:

- Rejects cyclic graphs (a step that depends on its own descendant).
- Runs independent steps in parallel.
- Stops downstream work when a dependency fails.
- Is cancellable from outside (HTTP request cancelled, server shutdown).

A simple "run steps in order" model is correct but wastes parallelism. A "spawn one goroutine per step" model can be correct but can also starve on dependencies.

## Decision

We use a topological-sort execution model:

1. **Validate.** Reject cycles. The validator walks the graph; if a back-edge is found, return an error.
2. **Topological sort.** Compute a level for every step. Level 0 is the set of steps with no dependencies. Level N is the set of steps whose dependencies are all in level ≤ N-1.
3. **Execute level by level.** For each level, start a goroutine for each step. Wait for the whole level to finish. Proceed to the next level.
4. **Failure propagation.** A step failure marks all transitive descendants as `skipped`.
5. **Cancellation.** A cancelled context is honoured by every step. When the context is cancelled, in-flight steps are given a chance to clean up, then marked `cancelled`.

## Options considered

1. **Topological levels (chosen).** Easy to reason about, easy to test, predictable resource usage.
2. **One goroutine per step with channel-based signalling.** More flexible but harder to bound concurrency and harder to test.
3. **External orchestrator (Temporal, Airflow).** Powerful, but a second service to operate.

## Consequences

Positive:

- Worst-case concurrency is bounded by the maximum width of the DAG, not by the number of steps.
- The execution status (`pending`, `running`, `completed`, `failed`, `skipped`, `cancelled`) is a closed enum and is persisted after every transition.
- Cycle detection is one DFS away.

Negative:

- A wide level with a slow step blocks the next level. We rely on the per-step timeout in the LLM middleware to bound the worst case.
- The execution is "level" synchronous. There is no speculative execution of steps whose dependencies have not yet finished.

## References

- `internal/workflow/` — execution engine
- `docs/workflows.md` — user-facing documentation
- `docs/algorithms.md` — execution algorithm pseudocode
