# Pen-test plan — promptsheon

The pen-test plan below is the scope + methodology we hand to
the third-party security firm. The auditor's findings live in
their secure portal and are not committed to this repo.

## Scope

### In scope

- `packages/server/src/` — every Fastify route, every repo, every
  agent subsystem, the audit chain, the metrics hooks, the
  gateway, the executors, the auto-eval library.
- `packages/shared/src/` — Zod schemas, the manifest schema, the
  CAS, the migration runner.
- `frontend/src/` — the Next.js app: API client, auth flow,
  rendering paths for every page. SSR-rendered routes are in
  scope for SSRF and header-injection review; client-only routes
  for XSS and CSRF.
- The release pipeline (`.github/workflows/ci.yaml`) and the
  ops scripts (`scripts/`).

### Out of scope

- The LLM providers (OpenAI, Anthropic, Bedrock, custom
  gateways). Their security posture is the provider's
  responsibility and is covered by their respective SOC 2 + ISO
  27001 reports; we send them `vendor-risk-questionnaire.md`.
- The customer's hosting environment (their K8s cluster,
  Terraform plan, RDS instance, CloudFront distribution). That's
  reviewed separately in their own pen-test scope.
- Browser extensions, mobile apps. None ship today.

## Methodology

### Black-box (external)

1. **Recon** — DNS, subdomains, ports. The customer usually runs
   promptsheon behind a reverse proxy with TLS; we expect port
   443 (HTTPS) only.
2. **Authentication** — probe the bootstrap path
   (`POST /api/bootstrap/admin` should refuse once an admin
   exists), the bearer-token path
   (`Authorization: Bearer ...`), the dev
   X-User-Id / X-Org-Id header path (production should reject
   unless explicitly enabled), the OIDC path (T2-2).
3. **Authorization** — for every admin-gated route, confirm a
   reader-role session receives 403. Test the role-escalation
   cap (`POST /api/api-keys` with admin should not produce a
   higher-role key than the caller).
4. **API fuzzing** — based on the OpenAPI at `/api/openapi.json`.
   Property-based fuzzing of `prompt` strings (looking for
   injections, PII leakage) and `inputs` (looking for JSON
   injection).
5. **Web app** — DOM-based XSS in the Next.js render path, CSRF
   tokens on state-changing endpoints (current implementation
   relies on SameSite + custom-header auth, so CSRF risk is
   bounded but worth verifying), CSP / X-Frame-Options / HSTS
   headers.
6. **SSRF** — every input that triggers an outbound call (LLM
   base URLs in `Settings` store, webhook URLs, manifest URLs
   in the DAG compiler). Verify the LLM router rejects
   `file://` and `localhost` URLs.

### Grey-box (with source)

1. **Audit-chain tampering** — write a script that flips a row in
   `audit_entries` and verifies `/api/audit/verify` reports the
   mismatch.
2. **Maker-checker bypass** — attempt to self-approve a release
   by rewriting the `X-User-Id` header (which should be ignored
   in production but might not be in dev). Confirm the gate fires.
3. **Manifest_dag injection** — submit a Manifest with a
   node that points at `node:child_process.exec` or arbitrary
   file paths; verify the executor refuses.
4. **Trace_run injection** — POST `/api/traces` directly with
   malicious attributes; verify the routes reject unknown fields
   via Zod.
5. **Vault KMS unwrap** — try to decrypt secrets from another
   org; verify the vault respects org scoping.
6. **Race conditions** — concurrent approvals on the same
   release; verify the maker-checker applies atomically.
7. **Replay attacks** — webhook receiver + nonce cache; verify
   within the TTL a duplicate event is rejected.

### White-box (with full source)

1. **Source-code review** — every route handler, every Zod
   schema, every SQL parameter binding.
2. **Dependency review** — `pnpm audit` baseline + every new
   dep needs an ADR-style justification in `CHANGELOG.md`.
3. **Crypto review** — audit chain (sha256 + content hash),
   vault (AES-256-GCM), webhook HMAC. Verify keys are never
   logged or returned over the wire.

## Frequency

- **Pre-release** — every minor release (v0.x → v0.x+1) gets a
  full grey-box + selected black-box before the audit window
  opens.
- **Quarterly** — external black-box of one of the customer
  tenants (rotated).
- **Annually** — full third-party Type II SOC 2 audit with the
  auditor's pen-test arm.

## Out-of-scope engagements

The auditor does NOT test:

- The customer's LLM API keys (they never see them).
- The customer's own users (treated as untrusted input).
- The customer's deployed infrastructure outside the promptsheon
  process boundary.

## Deliverables from the auditor

- Findings report with severity ratings aligned to CVSS.
- Reproduction steps for every High / Critical finding.
- An attestation letter for SOC 2 Type II evidence.
- Quarterly trend report (mean-time-to-remediate, recurring
  findings, etc.).

## Internal SLAs

| Severity | First response | Patch shipped |
|---|---|---|
| Critical | 1 business hour | 24 hours |
| High | 4 business hours | 7 days |
| Medium | 1 business day | 30 days |
| Low | 1 business week | next minor release |

## todo-soon

- Schedule first external pen-test post v0.5 release.
- Negotiate auditor agreement (Wyndham preferred; fall back to
  Vanta + Drata + a smaller auditor for the actual Type II
  attestation).
- Codify the customer-facing shared-responsibility matrix so
  audit cycles on both sides stay aligned.
