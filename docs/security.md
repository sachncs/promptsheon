# Security

This page covers the threat model, the auth model, the audit
chain, the vault, and the SSRF + webhook hardening. The
contribution policy for security issues is in
[`SECURITY.md`](../SECURITY.md); report vulnerabilities via
GitHub Security Advisories — **do not open a public issue**.

## Threat model

Promptsheon is a control plane: it stores customer prompts,
manages API keys to upstream LLM providers, and emits audit
events. The threat model is:

| Threat | Mitigation |
|--------|------------|
| **Credential theft** — an attacker with access to the daemon host reads `promptsheon.db` and harvests provider API keys. | AES-256-GCM vault (or KMS-backed `KeyProvider` for production). TLS on every non-loopback bind. |
| **Tamper of audit chain** — an attacker modifies a past audit row to hide a malicious action. | Hash chain (each row records the previous row's `entry_hash`; tampering breaks the chain). `VerifyAuditChain` walks the chain on demand. |
| **Privilege escalation** — a non-admin user creates an admin release or approves their own. | `MakerCheckerPolicy` self-enforces separation of duties (no vote from the creator, configurable RequiredApprovers). `mature_creator_can_vote` is closed-set. |
| **SSRF via webhook URL** — an attacker registers a webhook to `http://169.254.169.254/latest/meta-data/` and harvests IAM credentials. | Webhooks refuse non-HTTPS URLs to non-private / non-loopback / non-link-local / non-multicast / non-unspecified addresses. The `allow_private` per-endpoint flag was removed (SEC-4). |
| **BOLA / IDOR** — a user accesses another Workspace's data. | The auth model is workspace-scoped; permission checks happen at every handler. Per-workspace budgets + quotas are the billing boundary. |
| **Prompt injection** — a user message tricks the LLM into exfiltrating data. | Built-in `injection` Guardrail. Heuristics catch the obvious cases ("ignore all previous instructions", role-confusion attacks); production deployments layer an LLM-judge behind the same Guardrail interface. |
| **PII exfiltration** — a user submits PII in a prompt and the LLM provider ingests it. | Built-in `redactor` Guardrail strips email addresses, US SSNs, and other patterns at the pre-LLM and post-LLM boundaries. |
| **Webhook replay** — an attacker captures a signed webhook and replays it. | `X-Promptsheon-Signature: sha256=<hex>` HMAC includes a timestamp; the daemon rejects signatures older than 5 minutes. |

## Authentication

The daemon expects an `Authorization: Bearer <api_key>` header
on every authenticated endpoint. The OpenAPI spec lists the
permission required for each route; the SDK and CLI pass
those permissions through the key's role.

Three roles ship in v0.2.0:

| Role | Permissions |
|------|-------------|
| `admin` | All permissions including audit read, key manage, user manage, release activate. |
| `writer` | Capability create/update/delete, version add, release create, vote, activate, rollback, eval run, dataset/precondition create. |
| `reader` | All GETs (capability list/get, release get, etc.). Cannot mutate. |

The `admin` role is also the bootstrap role: the first caller
of `POST /api/v1/setup` (when auth is off or when
`PROMPTSHEON_BOOTSTRAP_TOKEN` is set) gets an admin key.

When `PROMPTSHEON_AUTH=false` is set with a non-loopback bind,
the daemon **refuses to start** with a clear error. This closes
the SEC-CONTAINER-1 foot-gun (a public bind with auth off
would mint an admin key to the first network-adjacent caller).

## Audit chain

The audit log is a **hash chain**, not an append-only list.
Each row records `previous_hash` (the previous row's
`entry_hash`) and `entry_hash` (sha256 of the row's canonical
JSON). `store.VerifyAuditChain` walks the chain from rowid 1
forward and asserts the invariant.

### Verify

`GET /api/v1/audit/verify` (admin-only) returns:

```json
{
  "ok": true,
  "tail_mismatch": false,
  "last_row_id": 42,
  "last_hash": "abc123...",
  "reason": ""
}
```

A failed verification (`ok: false`, `tail_mismatch: true`)
indicates that rows were removed out-of-order. The audit
archive (`audit_archive` table, migration 011) is the
retention target: expired rows are copied there and the
source row is preserved so the chain survives.

### Retention

The retention manager (`internal/observability`) runs on a
configurable interval (default 60 minutes). It enforces
`PROMPTSHEON_TRACE_TTL_DAYS` (min 30) and
`PROMPTSHEON_AUDIT_TTL_DAYS` (default 90). On a chain mismatch
the archive is skipped that cycle so the operator can
investigate.

`promptsheon_audit_dropped_total` and
`promptsheon_audit_queue_latency_seconds` surface the worker
pool's health. Drops are a 5xx-class event; the
`PromptsheonAuditChainBroken` alert fires on any drop.

## Vault

API keys for upstream LLM providers live in the vault
(`internal/vault`). The default implementation is AES-256-GCM
with a master key from `PROMPTSHEON_VAULT_KEY` (32-byte hex).
Production deployments should use a `KeyProvider` backed by a
managed-key service (AWS KMS, HashiCorp Vault, etc.) via the
`PROMPTSHEON_VAULT_KEY_PROVIDER` interface.

The vault never holds plaintext keys in the database; only
ciphertext + the key version + the algorithm identifier.

## Webhooks

Webhook secrets are encrypted at rest in the vault
(ADR-0027). Each delivery is signed with
`X-Promptsheon-Signature: sha256=<hex>` and includes a
timestamp; the daemon rejects signatures older than 5 minutes.

URL validation refuses:

- non-HTTPS schemes (SEC-4 removed the `allow_insecure` flag);
- loopback / private / link-local / multicast / unspecified
  addresses;
- hostnames that resolve to any of the above (DNS-rebinding
  mitigation).

The validation runs at registration AND at delivery time.

## Guardrails

Two built-in Guardrails ship in v0.2.0:

- `internal/redactor` — strips PII patterns (emails, US SSNs,
  phone numbers, etc.) at the pre-LLM and post-LLM boundaries.
- `internal/injection` — flags role-confusion attacks and
  common injection patterns. Heuristic today; production
  deployments layer an LLM-judge behind the same Guardrail
  interface.

Both ship via the plugin supervisor and can be disabled by
removing their entries from `PROMPTSHEON_PLUGINS_FILE`.

## Rate limiting

The `ratelimit` middleware enforces a per-client rate cap
(per-User or per-IP, configurable via `RATELIMIT_TRUSTED_PROXIES`).
The default is 100 req / 60s with a burst of 50. Rate-exceeded
responses include a `Retry-After: 60` header.

## CSRF / CORS

The daemon enforces CORS via the `cors` middleware. Operators
set `PROMPTSHEON_CORS_ORIGINS` to a comma-separated allowlist;
the wildcard `*` is rejected on non-loopback binds (it would
allow any browser to make credentialed cross-origin requests).
No origin == no CORS headers == the browser blocks the
response.

State-changing endpoints require the same `Authorization`
header regardless of origin. The `Sec-Fetch-Site` header is
inspected for additional hardening; `same-origin` and
`same-site` requests are allowed without question, and
`cross-site` requests are rejected unless explicitly
allowlisted.

## Operator checklist

1. `curl /api/v1/audit/verify` — returns `{ok: true, last_row_id,
   last_hash}`. Schedule this on a cron.
2. `curl /metrics | grep promptsheon_audit` — drop counter is
   zero; queue-latency histogram is bounded.
3. `PROMPTSHEON_AUTH=true`, `PROMPTSHEON_TLS_CERT_FILE` and
   `PROMPTSHEON_TLS_KEY_FILE` set on every non-loopback bind.
4. `PROMPTSHEON_VAULT_KEY` rotated quarterly; secrets read
   through a KMS-backed `KeyProvider` for production.
5. `PROMPTSHEON_HARNESS_PRECONDITIONS=true` only after
   preconditions have been audited. Default is `false`.

## Production secret layout

The recommended secret layout for production tenants. Every
secret lives in the operator's secret manager; the daemon
reads them at boot, never from disk.

| Secret | Where it lives | How the daemon gets it |
|--------|----------------|------------------------|
| TLS cert + key (`PROMPTSHEON_TLS_CERT_FILE`, `PROMPTSHEON_TLS_KEY_FILE`) | cert-manager `Certificate` resource; mounted as a `Secret` in the pod. | `volumeMounts` in the chart. |
| `PROMPTSHEON_VAULT_KEY` | AWS KMS / HashiCorp Vault / GCP Secret Manager. Sealed-secrets in-cluster. | `envFrom.secretRef` in the chart. Rotated quarterly. |
| `PROMPTSHEON_BOOTSTRAP_TOKEN` | Same secret store as `VAULT_KEY`. | `envFrom.secretRef`. Set once at install; the bootstrap endpoint issues the first admin key, then the operator rotates. |
| `PROMPTSHEON_OPENAI_API_KEY` / `PROMPTSHEON_ANTHROPIC_API_KEY` | LLM provider's dashboard. | `envFrom.secretRef`. Rotated by the operator when an employee with access leaves. |
| OIDC client secret (`PROMPTSHEON_OAUTH_GOOGLE_CLIENT_SECRET`, `PROMPTSHEON_OAUTH_GITHUB_CLIENT_SECRET`) | OIDC provider's console. | `envFrom.secretRef`. |
| Audit DB | PVC (regional block storage). | The StatefulSet mounts a `volumeClaim` for `/data`. |

The rule: no secret is ever committed to git, and no secret
lives in plaintext in the cluster. Sealed-secrets, External
Secrets, or a CSI driver for the secret manager are all
acceptable. The chart's `values.yaml` documents the
`secretStore` integration for the cert-manager and KMS
providers.

## See also

- [docs/configuration.md](configuration.md) — full env-var
  reference.
- [docs/operations.md](operations.md) — backup / restore.