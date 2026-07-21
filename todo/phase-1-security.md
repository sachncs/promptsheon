# Phase 1 — Security Hardening

All security findings, atomic. Fast forward: no deprecation shims; old code paths are deleted.

## Bootstrap and key mint

- [x] **SEC-5a** Wrap the empty-user check + admin key mint in a single SQLite transaction with `INSERT ... ON CONFLICT DO NOTHING` so only one caller can win.
  - **Where**: `internal/api/handlers_auth.go:286-381` and a new `BootstrapOnce(ctx) (apiKey, error)` method on `*SQLite`.
  - **Accept**: 100 concurrent `POST /api/v1/setup` calls produce exactly one admin key; the rest get `409 Conflict`.

- [x] **SEC-5b** Remove the unauthenticated `/api/v1/setup` route entirely when `PROMPTSHEON_AUTH=true`.
  - **Where**: `internal/api/server.go:382` and `internal/api/handlers_auth.go`.
  - **Accept**: With auth enabled, `POST /api/v1/setup` returns `404 Not Found` (and `403` if a stale path is reached).

## API-key role on user update

- [x] **SEC-6a** On `UpdateUser` (role change) and `DeleteUser`, mark every non-expired, non-revoked API key belonging to that user as revoked.
  - **Where**: `internal/store/sqlite.go:429-510` and `internal/api/handlers_users.go:86-100`.
  - **Accept**: Demoting a user from `admin` to `reader` invalidates their existing keys within the same transaction; the next request with that key returns 401.

## Webhook secrets

- [x] **SEC-7a** Encrypt `webhook_endpoints.secret` at rest using the same vault key as provider keys.
  - **What**: New `secret_ciphertext BLOB` column; secret is encrypted with AES-GCM before write; decrypted only when signing an outbound payload.
  - **Where**: new `internal/store/migrations/053_webhook_secret_ciphertext.up.sql`; `internal/store/sqlite.go:879-890`; `internal/webhook/webhook.go:57-66`.
  - **Accept**: `SELECT secret FROM webhook_endpoints` returns ciphertext; outbound webhook delivery still verifies correctly.

- [x] **SEC-7b** Drop the `GET /api/v1/webhooks` response's `secret` field; only return `secret_set bool`.
  - **Where**: `internal/api/handlers_webhooks.go:14-22` and `internal/models`.

## Webhook SSRF

- [x] **SEC-4a** Remove the `allow_private` field from `WebhookEndpoint`. All webhooks are blocked from private/loopback/link-local/cloud-metadata by default; override requires `PermWebhookAdmin` and a per-tenant SSRF allowlist.
  - **Where**: `internal/api/handlers_webhooks.go:28-56`, `internal/webhook/webhook.go:289-303`, `internal/models/alert.go:49-57`.
  - **Accept**: A webhook URL of `http://169.254.169.254/...` is rejected with `400 Bad Request` from any caller without `PermWebhookAdmin`.

- [x] **SEC-4b** Add `PermWebhookAdmin` role permission; route `POST /api/v1/webhooks` through it.
  - **Where**: `internal/auth/auth.go`, `internal/api/server.go:434-438`.
  - **Accept**: A `writer` role key gets 403 on webhook creation unless the role has `webhook:admin`.

- [x] **SEC-4c** Reject `http://` URLs at the webhook validator entirely (loopback or not).
  - **Where**: `internal/webhook/webhook.go:304-329`.
  - **Accept**: Any non-`https://` URL returns `400`.

- [x] **SEC-11a** Drop the `allow_insecure` field entirely; nothing is allowed to skip TLS verification.
  - **Where**: `internal/api/handlers_webhooks.go:28-56`, `internal/store/migrations/019_webhook_endpoints.sql`.

## Vault KMS

- [x] **SEC-10a** Make `kmsbyok.Provider` re-read the wrapped blob on cache miss and on every process start.
  - **What**: Drop the in-process plaintext cache; call `Decrypt` on the wrapped blob every time the vault needs a data key. Add an LRU cache (size 16, no negative caching) for performance.
  - **Where**: `internal/vault/kmsbyok/aws.go:35-60` and `internal/vault/kmsbyok/provider.go:64-99`.
  - **Accept**: Re-encrypting a secret with a new KMS key is reflected on the next read; decrypt still works after the wrapped blob rotates.

## Maker-Checker enforcement in the type

- [x] **SEC-1a** Make `MakerCheckerPolicy.Evaluate` consult `VerifySeparationOfDuties` directly.
  - **What**: `MakerCheckerPolicy{RequiredApprovers int, Creator string}` — the policy owns the creator; `Evaluate` returns `ErrCreatorVoted` when any vote's identity equals `Creator`.
  - **Where**: `internal/approval/approval.go:161-180, 189-199`.
  - **Accept**: A release created by `alice` with `alice`'s approve vote returns `ErrCreatorVoted`; the old call-site `VerifySeparationOfDuties` invocation is deleted.

- [x] **SEC-1b** Remove the side-check helper `VerifySeparationOfDuties` and the doc comment that contradicts the code at `approval.go:152-155`.
  - **Where**: `internal/approval/approval.go:185-199`.

## Audit completeness

- [x] **SEC-9a** Add audit entries for: API key mint (`handlers_auth.go:148`), API key revoke (`handlers_auth.go:440`), notification-group add (`handlers_alerting.go:156`), webhook create (`handlers_webhooks.go:24`), webhook delete (`handlers_webhooks.go:61`), OAuth callback success.
  - **Accept**: Every privileged mutation appears in `GET /api/v1/audit`.

## Recovery middleware content-type

- [x] **SEC-8a** Replace `http.Error` with a JSON-aware panic recovery that uses the same envelope as `writeError`.
  - **Where**: `internal/api/middleware.go:110-130`.
  - **Accept**: A handler that panics returns `Content-Type: application/json` with the standard error envelope.

## OAuth

- [x] **SEC-13a** Require `email_verified=true` from the IdP for OAuth logins; reject unverified emails with `403 Forbidden`.
  - **Where**: `internal/auth/oauth.go:35-42` and `internal/api/handlers_auth.go:571-598`.

- [x] **SEC-13b** Protect `OAuthManager.providers` with a `sync.RWMutex`.
  - **Where**: `internal/auth/oauth.go:44-63`.

- [x] **SEC-12a** Compare OAuth state cookie and query with `subtle.ConstantTimeCompare`.
  - **Where**: `internal/api/handlers_auth.go:528-537`.

## Audit chain integrity

- [ ] **SEC-CHAIN-1** Have `VerifyAuditChain` cross-check the final row count and `entry_hash` against `audit_chain_state`; report tampering if they diverge.
  - **Where**: `internal/store/sqlite.go:190-214`.
  - **Accept**: Deleting the last 5 audit rows and re-running the verifier returns "chain tail mismatch".

## Precondition sandbox

- [x] **SEC-2a** Replace the env allowlist with a denylist: pass through every env var except `AWS_*`, `*_KEY`, `*_TOKEN`, `*_SECRET`, `VAULT_ADDR`, `KUBERNETES_*`.
  - **Where**: `internal/harness/precondition.go:13-39`.
  - **Accept**: A precondition can read `HOME`/`PATH`/`LANG`; reading `AWS_ACCESS_KEY_ID` returns empty.

## JSONSchema scorer safety

- [x] **SEC-3a** Make `JSONSchema.ScoreCase` reject any schema that uses unsupported keywords (`allOf`, `oneOf`, `$ref`, etc.) with a typed error.
  - **Where**: `internal/eval/scorer.go:126-143, 164-217`.
  - **Accept**: A schema with only unsupported keywords returns `ErrUnsupportedSchema` from `ScoreCase`; the eval runner maps it to 422.

## Rate limiter hardening

- [ ] **SEC-RL-1** Replace the IP-keyed limiter with a per-user-or-IP key (user when authenticated, IP otherwise). Bucket size is per-key.
  - **Where**: `internal/ratelimit/ratelimit.go:240-271` and `internal/api/server.go:783-794`.

- [x] **SEC-RL-2** Treat `RATE_LIMIT_RATE=0` as "disabled" rather than "1,000,000 burst".
  - **Where**: `internal/ratelimit/ratelimit.go:130-134, 188-212`.

## Supply chain

- [x] **SEC-16a** Add cosign signing to the release pipeline.
  - **Where**: `.github/workflows/ci.yaml:205-225` and `.goreleaser.yml`.
  - **Accept**: Every release artefact carries a cosign signature; `cosign verify` succeeds.

- [x] **SEC-16b** Attach the SBOM to the GitHub Release (not just the workflow artefact).
  - **Where**: `.goreleaser.yml` and `.github/workflows/ci.yaml:179-203`.

- [ ] **SEC-16c** Add SLSA provenance attestation via `slsa-github-generator`.
  - **Where**: `.github/workflows/ci.yaml:205-225`.

## Container

- [ ] **SEC-CONTAINER-1** Add OCI labels to the Dockerfile (`org.opencontainers.image.source`, `.revision`, `.created`, `.version`).
  - **Where**: `Dockerfile`.
  - **Accept**: `docker inspect promptsheon` shows the labels.

- [ ] **SEC-CONTAINER-2** Drop `wget` from the runtime image; use a Go-based healthcheck binary or `curl --fail`.
  - **Where**: `Dockerfile:28, 48-49`.

## LLM provider base URL

- [ ] **SEC-LLM-1** Allow `PROMPTSHEON_OPENAI_BASE_URL` and `PROMPTSHEON_ANTHROPIC_BASE_URL` to be `http://` only when the daemon is bound to a loopback address; reject otherwise.
  - **Where**: `internal/llm/openai.go:28-36`, `internal/llm/anthropic.go:21-29`, `cmd/promptsheond/main.go:cfg.Validate`.
  - **Accept**: Setting an `http://` base URL on a non-loopback bind fails startup.

## DB hygiene

- [x] **SEC-DB-1** Seed a system user with `id="api"` and `role="system"` so audit FKs are satisfied.
  - **Where**: new migration `internal/store/migrations/054_seed_system_user.up.sql`.

- [x] **SEC-DB-2** Add FK `api_keys.user_id → users.id` with `ON DELETE CASCADE`.
  - **Where**: `internal/store/migrations/055_api_keys_user_fk.up.sql`.
