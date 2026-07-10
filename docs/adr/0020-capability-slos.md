# ADR 0020: Capability Service Level Objectives

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

Production tenants need to declare per-Capability availability
and performance contracts that the platform can monitor and
alert on. Without a first-class SLO model, the platform can
surface raw observability but cannot say "this Capability's
p95 latency breached its 200ms target twice this hour".

The architecture review board (§21 Tier 2.49) called out Capability
SLOs as the credibility lever between "we ship a tool" and
"we run their production."

## Decision

The SLO library ships as `internal/slo` with:

- `Signal` closed-set: `p95_latency_ms`, `p99_latency_ms`,
  `success_rate`, `hallucination_rate`, `avg_cost_usd_micro`,
  `availability`. New signals land as new constants and a
  follow-on signal-specific evaluator path; the closed set
  prevents ad-hoc signal strings.
- `Op` closed-set: `<`, `<=`, `>`, `>=`.
- `Window` closed-set: `5min`, `1hour`, `1day`. Burn-rate
  computation is the ratio `actual / target` (for `<=`)
  with the inverse for `>=` goals. Reasonable defaults
  match typical SLO practice.
- `SLO.Validate()` enforces the closed sets at construction.
  An invalid SLO refuses to compile against the evaluator.
- `SLO.Evaluate(actual)` returns nil on satisfaction, an
  error describing the breach otherwise. Errors are wrapped
  with the SLO ID, target, and actual value so audit logs
  are machine-readable.
- `SLO.Repository` is the consumer-defined persistence
  interface; the SQLite / Postgres implementations land
  in M1 follow-on work.

Severity is one of `page` (PagerDuty), `ticket` (Linear / Jira),
or `log` (Sentry / Datadog) and is wired to the Webhook / Alerting
package in a follow-on commit.

## Options considered

1. **Open-ended SLO model (raw PromQL expressions).** Rejected:
   the value type is value-typed and the alarm evaluation is
   local; raw PromQL would require a Prometheus sidecar and a
   network round-trip for every observation.
2. **Vendor-specific SLO (Prometheus, Datadog).** Rejected:
   vendors change; the model must survive rotation.
3. **Closed-set Goal + Burn-rate (chosen).** Matches the
   signals we already aggregate; aligns with the Charter
   "immutable aggregates" principle; lets the recommendation
   engine consume breaches as Recommendations.

## Consequences

Positive:

- Each Capability can declare one or more SLOs; breaches
  become Recommendation candidates and Policy alerts.
- The Repository interface lives in the consumer-defined
  directory; storage backends drop in.
- Burn-rate is computed locally; the recommendation loop has
  no latency penalty.

Negative:

- Closed-set Signals / Ops / Windows are a usability tax
  when an operator wants something exotic (e.g. "p75 of
  cache hits"). The follow-on migration path is a new
  constant.
- Severity wiring to PagerDuty / Linear / Sentry is out of
  scope for this commit (Tier 2 follow-on).

## References

- `internal/slo/slo.go` — value type, evaluator, Repository
- `internal/slo/slo_test.go` — full boundary table-driven suite
- Architecture Review §21 Tier 2.49.
