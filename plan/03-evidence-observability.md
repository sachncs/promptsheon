# Phase 3 — Evidence and observability

## Objective

Make every execution explainable, measurable, and useful for evaluation and
future improvement.

## Responsible

- Platform/operations: telemetry model, dashboards, alerts, retention.
- Runtime/backend: instrumentation at execution boundaries.
- Security: redaction, access control, sensitive-data handling.
- Data/domain: evidence schema and query model.
- Frontend: execution inspection and evidence presentation.

## Implementation

- Define immutable evidence records for inputs, outputs, model calls, routing,
  tool calls, guardrail decisions, permission decisions, costs, latency, errors,
  and resource usage.
- Attach correlation ID, trace ID, organization ID, agent hash, execution ID,
  and step ID to every record.
- Add structured Pino logs with stable event names.
- Add OpenTelemetry traces, metrics, and provider spans.
- Add redaction before logs or telemetry leave the process.
- Separate operational telemetry from user-visible evidence.
- Add evidence query, timeline, export, and retention APIs.
- Add dashboards for throughput, latency, errors, cost, token use, and provider
  health.
- Add retention and deletion workflows by organization and evidence class.

## Reliability and throughput

- Make telemetry non-blocking for the critical execution path.
- Bound buffers and define behaviour when telemetry backends are unavailable.
- Sample high-volume traces while retaining all policy, error, and release events.
- Use asynchronous batching for metrics and non-critical evidence.
- Avoid logging complete prompts or outputs unless policy explicitly permits it.
- Measure instrumentation overhead and set an acceptable budget.
- Add alert thresholds for queue age, error rate, cost spikes, and evidence lag.

## Testing

- Unit tests for redaction and sensitive-field classification.
- Contract tests for evidence schemas and event versioning.
- Trace-context propagation tests across HTTP, workers, providers, and tools.
- Tests proving telemetry failure does not fail an otherwise valid execution.
- Retention and deletion tests for tenant isolation.
- Snapshot tests for execution timelines and evidence exports.
- Load tests measuring telemetry overhead and buffer saturation.
- Security tests ensuring secrets never appear in logs, traces, or exports.

## Deliverables

- Evidence schema and event catalogue.
- Structured logging and tracing conventions.
- Execution timeline UI/API.
- Operational dashboards and alerts.
- Retention, export, and redaction runbooks.

## Exit criteria

- Any execution can be traced from request to final result.
- A user can identify cost, latency, decisions, and failure cause.
- Telemetry outages do not stop core execution.
- Sensitive information is removed according to policy.

