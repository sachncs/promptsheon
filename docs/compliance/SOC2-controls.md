# SOC 2 Type II — Control mapping

This document maps promptsheon's engineering controls to the
**AICPA Trust Services Criteria (TSC 2017, revised 2022)** so that
the auditor can verify each criterion against a specific,
reproducible artifact in the repo. The certification is *not yet
issued* — this map is the evidence ledger the auditor will work
from once the Type II observation window begins.

## Scope

In-scope system: the `packages/server` Fastify backend + the
`packages/shared` schema + migration bundle + the audit chain.
Out-of-scope: the SaaS hosting layer (where the customer
operates the binary), their AWS/GCP account, their CI runners,
their browser fleet, and the third-party LLM providers they
configure (OpenAI, Anthropic, Bedrock). The vendor due-diligence
questionnaire (`vendor-risk-questionnaire.md`) is what we send
the customer for the out-of-scope layer.

## Common Criteria (CC)

| CC | Description | Control evidence in this repo |
|---|---|---|
| CC1.1 | COSO principle 1 — integrity / ethical values | `AGENTS.md` engineering constitution; `CODE_OF_CONDUCT.md`; `CONTRIBUTING.md` requires Conventional Commits. |
| CC1.2 | Board oversight | _N/A — open-source project, no board._ Customer-side: their security steering committee owns oversight of promptsheon in their tenancy. |
| CC1.3 | Management establishes structures, reporting lines, authorities | Org chart under `CODEOWNERS`. `github/CODEOWNERS` defines file-level reviewers. |
| CC1.4 | Demonstrates commitment to competence | `CONTRIBUTING.md` describes the dev env, AGENTS.md codifies standards. |
| CC1.5 | Holds individuals accountable for their actions | Audit chain (see CC7.x); every mutation is logged; PRs require CODEOWNERS review. |

## Logical and Physical Access (CC6.x)

| CC | Description | Control evidence |
|---|---|---|
| CC6.1 | Logical access security software, infrastructure, architectures | OIDC SSO with per-team RBAC (`/api/auth/oidc/*`, T2-2). SCIM 2.0 for user provisioning. API keys issued via `/api/api-keys` with admin-only issuance + per-key role caps. |
| CC6.2 | Prior authorization for asset access | Per-org residency (`local | us | eu | ap | sa | me | af`); per-team scoping in audit chain partitions; admin-only routes gated by `requireAdmin()`. |
| CC6.6 | Logical access security measures | Bearer-token auth (sha256-hashed in DB). X-User-Id / X-Org-Id headers only for the dev/test path; production rejects these unless explicitly enabled. |
| CC6.7 | Restriction of credential transmission | HTTPS-only via the deployment reverse-proxy. CORS allowlist is explicit. |
| CC6.8 | Detection of unauthorized changes | Audit chain (`/api/audit/verify`) is hash-linked; tampering flips the verification flag. |

## System Operations (CC7.x)

| CC | Description | Control evidence |
|---|---|---|
| CC7.1 | Detect / monitor configuration vulnerabilities | Weekly dependabot (`/.github/dependabot.yml`) opens PRs on CVE bumps. CI runs `pnpm typecheck` + `pnpm --dir packages/server test` on every push (`/.github/workflows/ci.yaml`). |
| CC7.2 | System monitoring | `/api/events/:channel` SSE stream; OTel hooks (`packages/server/src/observability/setup.ts`); pino structured logs at every route. |
| CC7.3 | Evaluates security events | `AutoEval` runs against every trace_run; `/api/scores/summary` rolls up per evaluator. `error-rate` evaluator flags traces where > 0% of spans errored. |
| CC7.4 | Incident response | `docs/compliance/incident-response.md`. |
| CC7.5 | Recovery | `packages/server/test/backup.test.ts` (auto-scheduled) verifies the audit chain round-trip after a restore. Snapshots API (`/api/snapshots`) supports point-in-time recovery of capability state. |

## Change Management (CC8.x)

| CC | Description | Control evidence |
|---|---|---|
| CC8.1 | Change-management baseline | Every change lands via a PR; CI must pass before merge. AGENTS.md codifies the standards; lint + typecheck + tests are mandatory. |
| CC8.2 | Risk assessment of changes | Renovate/dependabot PRs include CVE severity; PR template requires "Risk + rollback" section. |
| CC8.3 | Change authorisation | CODEOWNERS enforces reviewer-by-file; production deployments go through a separate approval gate (`PROMPTSHEON_DEPLOY_PROTECTION=true`). |
| CC8.4 | Tracking and approval | GitHub release tags, signed commits (`signing-keys` migration + ed25519). |

## Risk Mitigation (CC9.x)

| CC | Description | Control evidence |
|---|---|---|
| CC9.1 | Identifies, selects, and develops risk mitigation activities | Threat model in `docs/compliance/threat-model.md`. Maker-checker + audit chain + KMS-vault abstraction for the highest-impact risks. |
| CC9.2 | Vendor and business partner risk | `docs/compliance/vendor-risk-questionnaire.md` is sent to OpenAI/Anthropic/AWS for tier-1 vendors. |

## Availability (A1.x)

| CC | Description | Control evidence |
|---|---|---|
| A1.1 | Maintains, monitors, and evaluates current processing capacity | Rate limiter (`/api/gateway/*`); circuit-breaker-style fallback chain on LLM calls; chaos engineering hooks (`/api/admin/chaos/*`) for resilience drills. |

## Confidentiality (C1.x)

| CC | Description | Control evidence |
|---|---|---|
| C1.1 | Identifies confidential information | Audit chain captures every read of prompts + outputs (auto-logged via `recordNodeRun`). |
| C1.2 | Disposes of confidential information | Retention sweeper (`RetentionSweeper`) honours per-org retention days. Audit chain is never swept. |

## Processing Integrity (PI1.x)

| CC | Description | Control evidence |
|---|---|---|
| PI1.1 | Defines data processing requirements | Schema validation (`packages/shared/src/validation.ts`); manifest_dag DAG validator; rate limiter; fallback chain. |
| PI1.5 | Implements policies and procedures | AutoEval registry + LLM-judge evaluators on every trace; gate thresholds configured per-org. |

## Privacy (P1.x — least applicable)

promptsheon is not the data controller for its customers'
data — it processes whatever the customer puts through it. The
customer's privacy notice covers the data. From promptsheon's
side, the relevant controls are:

- `docs/compliance/prompt-security.md` (T2-3): static PII /
  injection scan blocks saves that contain obvious secrets.
- KMS envelope encryption at rest (`/api/vault`).
- Per-org residency — a customer in `eu` residency cannot have
  data leave EU regions.

## Auditable artifacts

The auditor should be able to obtain, without promptsheon's
assistance:

1. The full git history of every change merged to `master`
   (`git log --first-parent master`).
2. The audit chain itself (`GET /api/audit/verify`).
3. The OpenAPI doc at `/api/openapi.json` and `packages/server/API.md`.
4. This compliance directory in `docs/compliance/`.
5. The npm-shrinkwrap of every shipped release (`pnpm-lock.yaml`).
6. The CI workflow definition at `/.github/workflows/ci.yaml`.
7. The SBOM at `docs/compliance/sbom.json` (auto-generated; see
   `scripts/build-sbom.sh`).

## What is NOT in this directory

- Real certification reports: those are issued by the auditor
  (Wyndham / Vanta / Drata). We are not certified yet; this
  document is the **control map**, not the audit report.
- Pen-test findings: `docs/compliance/pen-test-plan.md` is the
  scope + methodology. Actual findings live in the auditor's
  secure portal after a pen-test engagement.
- Customer-specific access logs: those live in the customer's
  SIEM, not here.

See the `_todo-soon` comment in `pen-test-plan.md` for the
12-month roadmap to first SOC 2 Type II report.
