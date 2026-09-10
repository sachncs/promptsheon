# Index — docs/compliance/

This directory holds the controls, plans, and procedures a SOC 2
Type II auditor will inspect (when the audit window opens).

| File | What it is | For whom |
|---|---|---|
| [`SOC2-controls.md`](./SOC2-controls.md) | The control map. Maps every AICPA Trust Services Criterion to a specific, reproducible artifact in this repo. | Auditor, SRE, security |
| [`pen-test-plan.md`](./pen-test-plan.md) | Black-box + grey-box + white-box scope. Defines what gets tested, what does not, and the engagement SLA. | External pen-test firm, internal SRE |
| [`vendor-risk-questionnaire.md`](./vendor-risk-questionnaire.md) | The questionnaire promptsheon sends to OpenAI / Anthropic / AWS / custom-gateway vendors before integrating with them. | Vendor security contacts |
| [`threat-model.md`](./threat-model.md) | STRIDE-class threat inventory + mitigations. Updated every major release. | SRE, security, pen-test firm |
| [`incident-response.md`](./incident-response.md) | What we do when something goes wrong. Severity classes, runbooks, public-statement SLAs. | SRE, customer success, on-call rotations |
| `sbom.json` | Auto-generated Software Bill of Materials. Re-run `scripts/build-sbom.sh` after every release. | Auditor, dependency-review tooling |

## What is NOT in this directory (yet)

- The actual SOC 2 audit report — that lives with the auditor
  after the engagement closes.
- Penetration-test findings — those live in the pen-test firm's
  secure portal.
- Customer-specific access logs — those live in the customer's
  SIEM.

See `SOC2-controls.md` for the controls map that should
eventually produce an audit report, and `pen-test-plan.md` for the
engagement timeline that produces the pen-test report.
