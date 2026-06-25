# Security

This page is the user-facing summary of the security model. For rationale and trade-offs see the ADRs listed in [Design Decisions](design-decisions.md). For the implementation details of individual algorithms (vault, audit chain, HMAC), see [Algorithms](algorithms.md).

## Threat model

The system assumes the following:

- **The host running `promptsheond` is trusted.** The server process and the database file are inside the trust boundary. An attacker with read/write access to the database file can rewrite the audit chain.
- **The network is hostile.** API requests come from untrusted clients. Webhook destinations are attacker-controllable URLs.
- **LLM providers are partially trusted.** They will sometimes fail, return errors, and occasionally return malformed or hostile content. They are not relied on for confidentiality.

Out of scope (and not provided): defence against a compromised host, defence against a compromised OS package, and key management. The vault key is a single env-var and is the operator's responsibility.

## Authentication and authorisation

- **API key authentication.** When `PROMPTSHEON_AUTH=true` (the default), every request except `/health`, `/ready`, `/metrics`, and the OAuth callback paths must carry `Authorization: Bearer <api_key>`. The key is checked against the `api_keys` table; the suffix of the key is the lookup index. The full key is returned once at creation time and is never stored.
- **Role-based access control.** API keys are bound to a user and a role. The roles are `admin`, `editor`, and `viewer`. See `internal/auth/`.
- **OAuth (Google, GitHub).** OAuth flow is supported for browser-based admin. The OAuth client credentials live in `PROMPTSHEON_OAUTH_*` env vars.
- **No API key in query string.** The `?api_key=` query parameter is disabled. Use the `Authorization` header.

## Transport and HTTP hardening

| Control | Where | Default | Why |
|---|---|---|---|
| `ReadHeaderTimeout` | `http.Server` | 10s | Slowloris defence. |
| `ReadTimeout` | `http.Server` | 30s | Bound the request read window. |
| `WriteTimeout` | `http.Server` | 60s | Bound the response write window. |
| `IdleTimeout` | `http.Server` | 120s | Bound the keep-alive idle window. |
| Request body size limit | `MaxBytesReader` middleware | 10 MB | Bound the per-request body to reject OOM attempts. |
| CORS | `CORS` middleware | `*` (configurable) | Default `*` is for ease of local use; production should set `PROMPTSHEON_CORS_ORIGINS` to an explicit list. |
| Security headers | `SecurityHeaders` middleware | always | Adds `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, and a strict CSP. |
| Panic recovery | `Recovery` middleware | always | Recovers panics in handlers, logs them, returns `500`. |
| Structured logging | `Logging` middleware | always | One `slog` line per request with method, path, status, latency, request ID. |

## Encryption at rest

- **Provider API keys** are encrypted with **AES-256-GCM** before they touch the database. The key is the 32-byte value of `PROMPTSHEON_VAULT_KEY` (64 hex chars). See [Algorithms — Vault](algorithms.md#vault-aes-256-gcm) and ADR [0004](adr/0004-aes-256-gcm-vault.md).
- **The all-zero key is rejected at startup.** A misconfigured key with no entropy would otherwise produce ciphertexts that are trivially decryptable.
- **Rotation** is manual. There is no KMS integration. Re-encrypting stored keys requires a one-off script that decrypts with the old key and re-encrypts with the new one.

## Audit log integrity

Every state-changing request writes an entry to `audit_entries`. The table is hash-chained: each row's `entry_hash` is the SHA-256 of a canonical representation of itself and the previous row's `entry_hash`. The chain is verifiable with `GET /api/v1/audit/verify`.

The chain is **tamper-evident, not tamper-proof.** An attacker with write access to the database can rewrite the chain. The threat model assumes the database itself is not compromised. See ADR [0003](adr/0003-hash-chained-audit-log.md) and [Algorithms — Audit chain](algorithms.md#audit-chain).

## Webhook security

- **HMAC-SHA256 signature.** Every delivery carries `X-Promptsheon-Signature: sha256=<hex>` where `<hex>` is `HMAC-SHA256(secret, body)`. The secret is per-endpoint, generated at registration, and shown to the user once.
- **SSRF policy.** By default, loopback (`127.0.0.0/8`, `::1`), link-local (`169.254.0.0/16`, `fe80::/10`), and private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`) destinations are refused at registration and at every delivery. Set `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE=true` to opt in (logged at startup).
- **Constant-time comparison on the receiver side.** Receivers must verify the signature with `hmac.Equal` or equivalent.

See ADR [0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md) and [Algorithms — Webhook HMAC signing](algorithms.md#webhook-hmac-signing).

## Shell tool policy

The workflow engine has a `shell` tool that can execute commands. The policy is fail-closed:

- The tool is enabled only if **both** `PROMPTSHEON_SHELL_ENABLED=true` **and** `PROMPTSHEON_SHELL_ALLOWLIST` contains at least one command.
- An empty allowlist with the tool "enabled" is treated as disabled (the server logs a warning and forces the enabled flag to `false`). This is deliberate: an empty allowlist that behaves as enabled is the foot-gun case.
- The allowlist is matched against the first token of the command (the program name). Arguments are not constrained — wrap the allowed program in a script if you need argument constraints.

## Rate limiting

- Per-API-key token bucket. Configurable via `PROMPTSHEON_RATE_LIMIT`, `PROMPTSHEON_RATE_BURST`, `PROMPTSHEON_RATE_LIMIT_INTERVAL`.
- `0` disables rate limiting. This is **not** recommended in production; use it only for local development.
- The limiter is per-process. With multiple server instances, each one applies its own limit; the effective per-key limit is `N_instances * limit`.

## SQL injection

All queries use parameterised statements via the `database/sql` package. There are no string-concatenated SQL paths in the server. The audit chain SHA-256 inputs are typed and separated by `0x1f` bytes, so hash collisions across columns are not possible.

## Secret management

- **Do not** commit `.env` files. The repository ships `.env.example` only.
- **Do not** log API keys. The server masks the `Authorization` header in the structured log output (the value is replaced with `Bearer ***`).
- **Do not** log full webhook secrets. Only the secret's ID is logged.

## Vulnerability reporting

If you discover a security vulnerability:

- **Do not** open a public GitHub issue.
- Use the GitHub Security Advisory workflow: `https://github.com/sachn-cs/promptsheon/security/advisories/new`.
- Or email the maintainers at the address in `CODEOWNERS`.

We will acknowledge within 48 hours, give an initial assessment within 1 week, and aim to ship a fix within 2 weeks depending on severity.

## Security checklist for operators

- [ ] `PROMPTSHEON_AUTH=true` (default).
- [ ] `PROMPTSHEON_VAULT_KEY` is set to a 32-byte hex string from a real source of entropy. **Not** all zeros.
- [ ] `PROMPTSHEON_CORS_ORIGINS` is an explicit list, not `*`.
- [ ] `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE` is `false` unless this is a local development environment.
- [ ] The shell tool is enabled only if the allowlist is non-empty.
- [ ] The database file is owned by the server user with mode `0640` or stricter.
- [ ] The vault key is not stored in the same place as the database.
- [ ] HTTPS is terminated upstream (nginx, Caddy, a load balancer).
- [ ] Logs are shipped to a separate aggregator and retained per the data-retention policy.
