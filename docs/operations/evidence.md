# Evidence and observability operations

Promptsheon records immutable, tenant-scoped evidence for execution lifecycle
events, model calls, tool calls, guardrail decisions, and permission decisions.
Evidence is operationally useful without making raw prompts or outputs a
requirement: payloads are redacted and canonically hashed before persistence.

## Data contract

Every record includes an event type, schema version, occurrence time,
organization, correlation ID, and payload hash. Execution records also carry
the execution ID and immutable agent hash; step-level records carry the step
ID. The event catalogue lives in `packages/server/src/repos/evidence.ts`.

Evidence is append-only. Updates are rejected by the database trigger, so
corrections are represented as a new event rather than mutation of history.
The user-facing trace page reads evidence through
`GET /api/traces/:id/evidence`; organization-wide administrators can query or
export it through `GET /api/evidence` and `GET /api/evidence/export`.
The trace dashboard reads `GET /api/traces/summary` for seven-day throughput,
error rate, latency, token/cost totals, and model-level health breakdowns.

## Redaction policy

Redaction runs at the persistence boundary and also on trace span fields and
structured log paths. Known credential keys, key/value secrets, email
addresses, SSNs, and payment-card-like values are replaced before storage or
telemetry emission. Do not add raw credentials, complete authorization
headers, or unrestricted prompt/output logging to new instrumentation.

If a new sensitive field is introduced:

1. Add a focused redaction test containing the field name and representative
   value.
2. Confirm the value is absent from evidence, trace spans, exports, and logs.
3. Record the retention class and update the event contract if the field is
   user-visible.

## Retention and deletion

The default evidence retention is 90 days. An administrator can set an
organization-specific value from 1 to 3650 days with
`PUT /api/orgs/:id/retention`, then run an immediate scoped sweep with
`POST /api/orgs/:id/retention/sweep`. The background sweeper performs the same
operation every six hours. Deletions are organization-scoped and are recorded
in the audit chain; audit entries themselves are not swept.

Use the organization retention API for deletion requests rather than issuing
SQL directly. Verify the returned `evidence_records` deletion count and run a
scoped evidence query afterward to confirm that the cutoff was applied.

## Throughput and failure behaviour

Evidence writes are asynchronous and bounded. Low-priority events may be
dropped when the buffer is saturated; execution failures, cancellations,
guardrail decisions, permission decisions, and observed errors are retained
preferentially. Writer failures are swallowed by the sink so core execution
state remains authoritative. The sink is flushed during graceful server
shutdown.

When investigating evidence lag, inspect queue depth and writer errors in the
service logs, then compare the execution timeline with the evidence query.
Increasing the buffer should be preceded by measuring memory and write
throughput; it is not a substitute for fixing a slow database or retention
backlog.

## Alerting baseline

Configure alert rules for the operational summary and execution queue with
thresholds appropriate to the organization. The minimum production baseline
is:

| Signal | Starting threshold | Response |
| --- | ---: | --- |
| Queue age | 60 s | inspect worker capacity and provider latency |
| Error rate | 5% over 5 min | inspect the evidence failure events and provider/model split |
| Average latency | 2 s over 5 min | inspect model, tool, and guardrail spans |
| Cost spike | 2× seven-day baseline | check routing changes and token budgets |
| Evidence lag/drops | any sustained drop or writer failure | inspect sink metrics and database throughput |

These are starting thresholds, not universal SLOs. Alert rule changes should
be reviewed with the owning team and recorded with the release or incident
that motivated them.

## Verification checklist

- Run the server typecheck and evidence/trace/retention tests.
- Exercise an execution and confirm one trace root plus correlated evidence.
- Confirm an export contains no test secret, token, cookie, or authorization
  value.
- Run a scoped retention sweep in a test organization and verify tenant
  isolation.
- Confirm the trace detail page shows spans, evidence, cost, latency, and
  evaluation results for the same trace ID.
