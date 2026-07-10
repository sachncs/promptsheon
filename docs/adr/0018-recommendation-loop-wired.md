# ADR 0018: End-to-end Recommendation loop wired through Executor -> Observation -> Producer

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The Capability engineering board review flagged that the
Recommendation loop was only half-built: capability.Decision
shipped, but capability.Recommendation had no producer. The
Executor published `execution.finished` to the EventBus but
nothing consumed them; the Optimizer rules engine sat unused.

This ADR records the closing of that loop in three atomic
commits:

1. `internal/observation` — `Aggregator` accumulates
   `ExecutionRecord` values by `(capability_id, version_id,
   environment)` and produces `rules.Observation` summaries.
2. `internal/recommendation` — `Producer` is an EventBus
   subscriber on `schedule.fired` and `execution.finished`;
   `Tick(now)` runs an aggregation pass, feeds the rules engine,
   persists via the supplied SinkFunc, and emits one
   `EventRecommendationGenerated` per Recommendation.
3. `internal/invoke` — the canonical `Invoker` enforces Quota and
   Budget, runs the Caller through the Executor, and adds every
   `ExecutionRecord` to the Aggregator. The next Tick produces any
   Recommendations that the rules justify.

## Decision

1. **DefaultEnforcer** (in `internal/invoke`) is the in-memory Budget
   + Quota enforcer used by tests and single-process installs.
   It persists `q.Charge` and `b.Charge` results by reassigning the
   map entry — a real bug I introduced and immediately fixed in
   this commit (Charge returns a new value per the immutability
   principle).
2. **import cycle fix**: the `rules` package now provides its own
   `CanAutoAdopt` helper; the previous boundary pulled
   `rules -> recommendation -> rules` which Go forbids. The
   shape of the helper is the conservative default; the
   capability package retains the data types.
3. **New domain package: `internal/invoke`**. It owns the canonical
   invocation path and enforces `Budget` + `Quota`. Quota is
   consulted before the call (cheap, in-memory) so an over-quota
   request never reaches an LLM provider. Budget is consulted
   after the call against the Caller-reported cost.

## Options considered

1. **Closure-on-execution vs streaming.** Rejected streaming:
   providers stream deltas, but the Invocation record is what
   triggers the Observation window; we want the full record
   before producing a Recommendation.
2. **Single-tier Tick frequency (every N seconds) vs scheduled
   per-window-aggregation.** Rejected per-window; the v1 rule
   engine fires on aggregate thresholds above a 32-execution
   floor, and a single Tick per minute is sufficient.

## Consequences

Positive:

- The Recommendation loop is closed end-to-end against deterministic
  rules. Production wiring adds bandit and LLM-judge engines
  without changing the Producer API.
- Quota rejection (429) and Budget rejection (402) errors map
  cleanly to HTTP status codes in the handlers that adopt the
  `invoke.Invoker`.
- The Aggregator window size is configurable (5 minutes / hour)
  but the default 32-exec minimum matches the rules' heuristic.

Negative:

- The Aggregator is in-memory; production scale moves it to
  ClickHouse. The contract doesn't change.
- `DefaultEnforcer` is a stand-in; production wiring supplies a
  backend-backed `Enforcer` implementation (SQLite / Postgres).

## References

- `internal/observation/observation.go` — Aggregator
- `internal/recommendation/producer.go` — Producer
- `internal/invoke/invoke.go` — Invoker, Enforcer, DefaultEnforcer
- `internal/optimizer/rules/rules.go` — relocated CanAutoAdopt helper
- Architecture Review §21 Tier 1.04.
