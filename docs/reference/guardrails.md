# Guardrails

Guardrails are a `Provider` plugin that wraps a Capability's
LLM call. Two Guardrails ship as built-in plugins through
the supervisor; production tenants can add custom
Guardrails via the plugin manifest.

The two built-in Guardrails:

- **`redactor`** (`backend/redactor`) — strips PII patterns
  (emails, US SSNs, phone numbers, etc.) at the pre-LLM and
  post-LLM boundaries.
- **`injection`** (`backend/injection`) — flags
  role-confusion attacks and common prompt-injection patterns.
  Heuristic today; production deployments layer an LLM-judge
  behind the same Guardrail interface.

## Plugin contract

A Guardrail plugin implements the `Provider` interface
(`Complete(ctx, *Request) (*Response, error)`); the daemon
chains Guardrails around the base LLM `Provider` so the
response flows through every registered Guardrail before
returning to the caller.

The plugin supervisor wires built-in Guardrails the same
way it wires remote plugins: through the manifest. Built-ins
declare `binary: <builtin>` and the supervisor resolves that
to the in-process handler.

## Endpoints

The daemon does not expose an HTTP surface for plugin
registration today; plugins are configured via
`PROMPTSHEON_PLUGINS_FILE` (manifest path) and the daemon
boots with the supervisor's in-process built-ins registered
under their fixed names (`pii-redactor`, `prompt-injection`).
The two metrics below are the only operator-visible surface
for built-in Guardrail activity; the supervisor surfaces
restart counts and health via the standard
`/api/v1/metrics` Prometheus endpoint.

## Metrics

The two built-in Guardrails emit metrics through the
`promptsheon_guardrail_*` series:

| Metric | Type | Notes |
|--------|------|-------|
| `promptsheon_guardrail_violations_total` | counter | Every Guardrail rejection. |
| `promptsheon_guardrail_blocks_total` | counter | Every block. |
| `promptsheon_guardrail_passes_total` | counter | Every pass. |

`promptsheon_guardrail_violations_total` is what you alert on
in production. A non-zero rate over 5 minutes indicates the
Guardrail is catching things — a 0 rate over 24h is a
stronger signal that the Guardrail isn't being invoked at
all (probably misconfigured).

## Operator guide

- The `redactor` Guardrail is heuristic; production
  deployments should layer an LLM-judge Guardrail behind the
  same interface for sensitive use cases.
- The `injection` Guardrail runs every user message through a
  small set of canonical patterns. To add a new pattern,
  edit `backend/detector.go` and add the regex
  plus a test.
- Custom Guardrails register a `Provider` factory on the
  manifest; see [docs/development.md](../development/development.md) for the
  plugin manifest format.

The daemon does **not** expose a
`/api/v1/guardrails/check` endpoint — Guardrail evaluation
happens inline on the LLM call, not via a separate HTTP
route. The metrics above are the only surface for Guardrail
activity.
