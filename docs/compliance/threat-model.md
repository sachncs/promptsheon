# Threat model — promptsheon

Built using STRIDE-class categories. Updated quarterly. The list
below is the inventory we walk through before any new architecture
proposal lands; it's also the lens we use during pen-test
retrospectives (see `docs/compliance/pen-test-plan.md`).

## Assets

| Asset | Where it lives | Sensitivity |
|---|---|---|
| Customer prompts + outputs | `trace_spans.input_text / output_text`; persisted for replay (T3-1). | **High** — may contain PII, credentials, or trade secrets. |
| Audit chain | `audit_entries` (append-only, hash-linked). | **High** — integrity is the audit's whole point. |
| Vault secrets (API keys, LLM keys) | `vault_secrets` table, AES-256-GCM. | **Critical** — leak = full account takeover. |
| Operator signing keys (ed25519) | `signing_keys` table. | **Critical** — leak = impersonation of releases. |
| Releases / capability manifests | `manifest_dag` table. | **High** — product-defining. |
| Maker-checker approvals | `manifest_approvals` table. | **High** — governance integrity. |
| Org settings (residency, KMS provider) | `org_settings`. | **Medium** — affects cross-region data flow. |

## STRIDE-class threats

### Spoofing

- **S-1**: An attacker with access to the dev X-User-Id / X-Org-Id
  header path can impersonate any user. **Mitigation**:
  `PROMPTSHEON_NODE_ENV=production` rejects these headers unless
  explicitly enabled; OIDC SSO is the production auth path
  (T2-2). Tested in `test/auth-middleware.test.ts`.
- **S-2**: Forge an audit-chain entry. **Mitigation**: the chain
  is hash-linked; `/api/audit/verify` flags any mutation.

### Tampering

- **T-1**: A malicious actor with DB access rewrites an audit
  entry. **Mitigation**: append-only enforcement at the
  application layer + verification endpoint + tamper-evident
  hash chain.
- **T-2**: An attacker injects a node into a DAG that executes
  arbitrary code. **Mitigation**: the DAG validator
  (`validateDag`) checks structural invariants; the executor
  routes node invocations through the Strands SDK which only
  understands the registered tool adapters.

### Repudiation

- **R-1**: A user denies they approved a release. **Mitigation**:
  audit chain + maker-checker log both the request and the
  approval event with actor_id + timestamps.

### Information disclosure

- **I-1**: Trace spans leak customer prompts to the LLM provider.
  **Mitigation**: the executor only forwards to the configured
  provider; the prompt-security scanner (T2-3) flags obvious
  PII / injection patterns before save.
- **I-2**: Vault secrets leak via log lines. **Mitigation**: pino
  redact list covers `apiKey`, `authorization`, `*.token`.
- **I-3**: Customer A reads customer B's data. **Mitigation**:
  every repo query is scoped by `organization_id` (enforced in
  the `orgContextMiddleware`); cross-org reads are impossible.

### Denial of service

- **D-1**: An attacker overwhelms the gateway with cheap
  cache-busting requests. **Mitigation**: rate limiter per
  actor + per-IP, plus the content-hash cache.
- **D-2**: A large manifest_dag query starves the executor.
  **Mitigation**: pagination on every list endpoint; the
  executor runs in-process with bounded concurrency.

### Elevation of privilege

- **E-1**: A reader-role user mints an admin API key. **Mitigation**:
  role-escalation cap on `POST /api/api-keys`; even an admin
  can't mint a key higher than their own.
- **E-2**: A user edits a manifest to include a prompt injection
  that escalates to a system-level tool call. **Mitigation**:
  prompt-security scanner (T2-3) flags obvious patterns; the
  executor's tool list is the allowlist.
- **E-3**: An attacker exploits the OIDC callback to inject a
  role claim. **Mitigation**: OIDC token validation verifies
  the signature against the IdP's JWKS; we never trust
  unverified claims (T2-2).

## Out of scope for promptsheon

- The customer's own cloud account / IAM.
- The LLM provider's security posture (covered by their SOC 2 +
  the `vendor-risk-questionnaire.md` we send them).
- Network-layer attacks between the customer and promptsheon
  (handled by the customer's reverse proxy + TLS).

## Update cadence

- Full STRIDE walk: once per major release.
- Spot-checks: every commit that touches auth, audit, vault,
  signing, or release governance.
- Pen-test retrofit: after every external pen-test
  (`pen-test-plan.md`).
