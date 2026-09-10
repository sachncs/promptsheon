# Security

This directory is the operator-facing index for Promptsheon's
security material. It links out to the canonical documents rather
than duplicating them.

## Threat model

The full threat model — assets, adversaries, trust boundaries,
attack vectors, mitigations — lives at
[../compliance/threat-model.md](../compliance/threat-model.md).
SOC2 controls ([../compliance/SOC2-controls.md](../compliance/SOC2-controls.md))
map the threat model to the CC family of controls.

## Penetration testing

The pen-test plan — scope, methodology, exit criteria, reporting
format — lives at
[../compliance/pen-test-plan.md](../compliance/pen-test-plan.md).
Internal pen-test cadence is documented in the same file.

## Prompt-security benchmark

The 53-case OWASP-LLM benchmark dataset and the per-rule
RESULTS.md live at:

- [./benchmark/dataset.json](./benchmark/dataset.json)
- [./benchmark/RESULTS.md](./benchmark/RESULTS.md)

The benchmark exercises the scanner in
`packages/server/src/security/prompt-scanner.ts`. Run it with
`pnpm --filter @promptsheon/server bench:security`; the runner
writes back to `./benchmark/RESULTS.md` and exits non-zero on
any regression.

## Incident response

The on-call runbook — severity ladder, escalation contacts,
post-mortem template — lives at
[../compliance/incident-response.md](../compliance/incident-response.md).

## Reporting a vulnerability

Follow [../../SECURITY.md](../../SECURITY.md) — do not file
public GitHub issues for security bugs.
