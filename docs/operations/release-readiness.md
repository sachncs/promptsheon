# Release readiness record

This record describes the strongest release state that can be verified without
an external LLM credential. It is intentionally not a certification or a
production-capacity claim.

## Verified on 2026-09-30

The following evidence was produced from the repository and the hosted CI run
for commit `e7d0ffe`:

| Gate | Result | Evidence |
| --- | --- | --- |
| Workspace type-check and frontend lint | Pass | CI `36717941030` |
| Public site build, docs, claims, and inventory checks | Pass | CI `36717941030` |
| Software bill of materials validation | Pass | SPDX document with 1,114 resolved packages |
| Server and shared package tests | Pass | 878 tests in CI `36717941030` |
| Production application build | Pass | CI `36717941030` |
| Credential-free browser journey | Pass | 79/79 Playwright tests in CI and locally |
| Container build and readiness smoke check | Pass | CI `36717941030` |
| LLM provider credential usage | Not used | All browser and execution checks use the deterministic simulator |

## Included in this release state

- Durable execution jobs with retries, cancellation, lease recovery, evidence,
  traces, evaluations, mutations, promotion governance, and rollback paths.
- Organization-scoped authentication and authorization, rate limits, payload
  validation, tenant isolation, quotas, and quota reservations.
- Content-addressed manifests and CAS verification, audit-chain verification,
  backup/restore tooling, and health/readiness endpoints.
- Frontend onboarding, control-plane workflows, responsive behavior, keyboard
  focus foundations, and named-control/accessibility checks.

## GA blockers and required evidence

These items are not proven by the credential-free gates above and must remain
open before an enterprise GA declaration:

1. External penetration testing, dependency vulnerability review, and formal
   compliance evidence (SOC 2, ISO 27001, HIPAA, or GDPR as applicable).
2. OIDC/SAML SSO, SCIM provisioning, team-scoped RBAC, and session lifecycle
   controls for enterprise identity providers.
3. Representative production load, soak, stress, provider-outage, worker-crash,
   database-restart, and restore-drill reports.
4. Provider-native token streaming and usage reconciliation; the OpenAI-
   compatible `stream: true` path is currently buffered compatibility SSE.
5. Deployment-managed signing for exported compliance evidence and a documented
   migration rollback procedure for the target hosting platform.
6. Browser compatibility, visual regression, and accessibility review beyond
   the credential-free Chromium foundation checks.

## Release decision

The current state is suitable for a credential-free development or controlled
pilot release. It is not sufficient evidence for an unrestricted enterprise
GA release until the blockers above have owners, test evidence, and explicit
risk acceptance.

## Reproduction commands

```bash
pnpm test
pnpm typecheck
pnpm --dir frontend lint
PROMPTSHEON_E2E_PORT=3022 \
PROMPTSHEON_E2E_BACKEND_PORT=8102 \
pnpm --dir frontend exec playwright test \
  --grep 'tier 1:|tier 2:|tier 3:|tier 4:|tier 5:|tier 6:|tier 7:|tier 8:|tier 9:|tier 10:|tier 11:|tier 12:'
bash scripts/build-sbom.sh
```
