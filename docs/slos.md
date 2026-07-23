# Service Level Objectives

Promptsheon publishes three first-class SLOs. Each SLO has a target,
a burn-rate alert (in [`deploy/prometheus/promptsheon-alerts.yaml`](../deploy/prometheus/promptsheon-alerts.yaml)),
and the Prometheus queries used to compute it. The numbers assume a
30-day rolling window unless otherwise stated.

## SLO 1 — Invoke p95 latency

**Target.** 95% of capability invocations complete in under 5 seconds
(server-side, end-to-end).

**Why 5s.** Generative calls include network round-trips to the LLM
provider. A 5s ceiling keeps user-facing agents responsive while
absorbing provider variance.

**Measurement.** `promptsheon_llm_latency_seconds` is a Prometheus
histogram observed inside `LLMMiddleware`. The 95th percentile over a
5-minute window is the operative signal; the same query over 30 days
is the SLO.

```promql
histogram_quantile(
  0.95,
  sum by (le) (rate(promptsheon_llm_latency_seconds_bucket[30d]))
)
```

**Burn-rate alert.** Fires when the 5-minute p95 exceeds 5s for 10
minutes (see `PromptsheonInvokeLatencyHigh`).

**What it doesn't cover.** Time spent in the client SDK before the
request hits the daemon. End-to-end client latency should be measured
upstream of the daemon.

## SLO 2 — Audit chain integrity

**Target.** Zero dropped audit entries. The hash-chained audit log
must be complete and verifiable at all times.

**Why zero.** The audit log is a regulatory artefact; any gap breaks
`VerifyAuditChain`. The chain walks from `rowid 1` forward, so a
single missing row poisons the rest of the chain.

**Measurement.** `promptsheon_audit_dropped_total` is a counter
incremented by the audit worker pool when its bounded queue
overflows. Any non-zero value is a violation.

```promql
sum(increase(promptsheon_audit_dropped_total[30d]))
```

**Burn-rate alert.** Fires on any drop in a 5-minute window (see
`PromptsheonAuditChainBroken`). The threshold is intentionally zero
tolerance; a single drop warrants investigation.

**What it doesn't cover.** Logical tampering of entries that do get
written. That is detected by the periodic `VerifyAuditChain` walk
(`/api/v1/audit/verify`), which returns a structured
`AuditVerifyResult`. Operators run it on a schedule and alert on the
`TailMismatch` field.

## SLO 3 — Harness eval success rate

**Target.** At least 90% of harness eval cases pass on a rolling 1-hour
window.

**Why 90%.** Real eval suites always include negative cases. 90% is
the floor for a healthy harness; sustained drops below this indicate
regressions in the prompts or scorer.

**Measurement.** `promptsheon_eval_cases_total` is a counter labeled
with `result="pass"` or `result="fail"`. The pass ratio over the
window is the operative signal.

```promql
sum(rate(promptsheon_eval_cases_total{result="pass"}[1h]))
/
clamp_min(sum(rate(promptsheon_eval_cases_total[1h])), 1)
```

**Burn-rate alert.** Fires when the failure ratio exceeds 10% for 30
minutes (see `PromptsheonHarnessEvalFailureRateHigh`).

**What it doesn't cover.** Pass/fail labels are scorer-derived. A
bug in the scorer can flip the metric without the harness actually
regressing; pair this SLO with a manual spot-check.

## Operating the SLOs

- The Prometheus scrape job for promptsheon is configured to hit
  `GET /metrics` (PermAuditRead-gated; bind to a loopback listener in
  production). See [`configuration.md`](configuration.md) for the
  `PROMPTSHEON_METRICS_ADDR` knob.
- The Grafana dashboard
  [`deploy/grafana/promptsheon-dashboard.json`](../deploy/grafana/promptsheon-dashboard.json)
  visualises all three SLOs alongside cost, queue depth, and workflow
  throughput.
- Alert routing should target the on-call channel for `severity=critical`
  alerts (`PromptsheonAuditChainBroken`) and the engineering channel
  for `severity=warning`.
