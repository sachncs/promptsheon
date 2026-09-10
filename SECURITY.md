---
layout: page
title: Security
subtitle: Threat model, disclosure policy, and hardening layers.
---

# Security

Promptsheon ships with a documented threat model, a SOC2 control set, and a public audit chain. This page is the operator-facing summary. The full documents live in the repo.

## Threat model

The full threat model — assets, adversaries, trust boundaries, attack vectors, mitigations — lives at [`docs/compliance/threat-model.md`](https://github.com/sachncs/promptsheon/blob/master/docs/compliance/threat-model.md).

The trust boundaries are:

| Boundary | On one side | On the other side |
|----------|-------------|-------------------|
| Browser | The user | The Next.js UI |
| UI ↔ API | The Next.js server-side render | The Fastify backend |
| Backend ↔ Disk | The Node process | The SQLite + CAS on disk |
| Backend ↔ LLM | The Strands SDK | The provider (OpenAI, Anthropic, Bedrock, custom) |

Every boundary has a documented set of mitigations.

## Authentication

Promptsheon supports two mechanisms:

| Mechanism | Header | Use case |
|-----------|--------|----------|
| Bearer API key | `Authorization: Bearer <token>` | Human users and SDK callers |
| SVID | `Authorization: SVID <token>` | Autonomous agents |

When `PROMPTSHEON_AUTH=true` (the production default), any request without a Bearer or SVID header returns `401 UNAUTHORIZED`. The legacy `X-User-Id` fallback is honoured only when `PROMPTSHEON_AUTH=false`. See [`packages/server/src/middleware/auth.ts`](https://github.com/sachncs/promptsheon/blob/master/packages/server/src/middleware/auth.ts).

The SVID is an ed25519-signed JWT carrying the agent's id, organisation, and classification. Verification is enforced against `PROMPTSHEON_SVID_PUBLIC_KEY_PEM`. A successful verification stamps the request with an `Agent` principal that the Cedar gate can authorise against.

## Authorization

Cedar policies under [`packages/server/policies/promptsheon.cedar`](https://github.com/sachncs/promptsheon/blob/master/packages/server/policies/promptsheon.cedar) are the single source of truth for every authorization decision. The `CedarAuthorizer` is loaded at boot and invoked by the gate middleware. The smoke-test route is `/api/orgs/:orgId/teams` (see `routes/org-team.ts`).

Run `pnpm policy:eval` to verify the policy against the regression cases at [`packages/server/test/policy/cases.json`](https://github.com/sachncs/promptsheon/blob/master/packages/server/test/policy/cases.json). The runner exits non-zero on any drift so a policy tweak can never silently weaken coverage.

## Audit chain

Every state transition appends a frame to a hash-linked log. The chain is publicly verifiable at `GET /api/audit/verify`. Tampering with any frame breaks the chain and the endpoint returns `{"valid": false, "brokenAt": N}`.

## Hardening layers

- **Rate limiting** — `@fastify/rate-limit` caps at 100 requests per minute per user (or per IP for unauthenticated calls).
- **CORS** — single origin, configured via `PROMPTSHEON_CORS_ORIGIN`. Defaults to `http://localhost:3000` for local dev.
- **SSRF guards** — outbound LLM URLs are validated against a private-IP blocklist.
- **Input validation** — every request body is parsed through Zod before any handler runs.
- **Maker-checker** — releases require two non-creator approvals before activation.
- **Webhook replay protection** — incoming webhooks carry an HMAC signature and a one-shot nonce.

## SOC2 controls

The control set lives at [`docs/compliance/SOC2-controls.md`](https://github.com/sachncs/promptsheon/blob/master/docs/compliance/SOC2-controls.md). Every CC control maps to:

1. The policy that enforces it.
2. The test that exercises it.
3. The audit-chain frame that records it.

## Penetration testing

The pen-test plan — scope, methodology, exit criteria, reporting format — lives at [`docs/compliance/pen-test-plan.md`](https://github.com/sachncs/promptsheon/blob/master/docs/compliance/pen-test-plan.md). Internal pen-test cadence is documented in the same file.

## Incident response

The on-call runbook — severity ladder, escalation contacts, post-mortem template — lives at [`docs/compliance/incident-response.md`](https://github.com/sachncs/promptsheon/blob/master/docs/compliance/incident-response.md).

## Prompt-security benchmark

The 53-case OWASP-LLM benchmark dataset and the per-rule RESULTS.md live at [`docs/security/benchmark/`](https://github.com/sachncs/promptsheon/tree/master/docs/security/benchmark). Run the scanner with `pnpm --filter @promptsheon/server bench:security`. The runner exits non-zero on any regression.

## FIPS mode

Set `PROMPTSHEON_FIPS_MODE=true` and start the server on a FIPS-validated Node build. The audit chain's sha256 calls go through the FIPS provider and the server refuses to boot otherwise.

## Reporting a vulnerability

Follow the disclosure process in [`SECURITY.md`](https://github.com/sachncs/promptsheon/blob/master/SECURITY.md). Do **not** open a public GitHub issue. We acknowledge within 48 hours, give an initial assessment within 1 week, and aim to ship a fix within 2 weeks depending on severity.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
