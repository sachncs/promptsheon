# Phase 4 — API Surface Repair

All API findings. Fast forward: replace, don't deprecate.

## OpenAPI

- [x] **API-1** Extend `scripts/genopenapi` to walk every `register*Routes()` method. (See Phase 0.)
- [ ] **DOC-2** Set `api/openapi.yaml:version` to match the product. (See Phase 0.)

- [ ] **API-5a** Generate real request/response schemas for every route, not `type: object` placeholders.
  - **Where**: `scripts/genopenapi/main.go` and `api/openapi.yaml`.

- [ ] **API-5b** Add the `details` field to the `Error` schema and document its structured use.
  - **Where**: `api/openapi.yaml:24-29` and `internal/api/server.go:555-557`.

- [ ] **API-9** Add a contract test that round-trips every route via the Go SDK against a running daemon; fail CI on drift.
  - **Where**: `tests/contract/` (new) and `.github/workflows/ci.yaml`.

## Versioning

- [ ] **DOC-2a** Pick one source-of-truth for the version string. Replace all three with a single `internal/buildinfo.Version` constant.
  - **Where**: `internal/buildinfo/buildinfo.go`, `api/openapi.yaml:11`, `sdk/python/src/promptsheon/__init__.py:14`, `sdk/typescript/package.json:3`, `deploy/helm/promptsheon/Chart.yaml:11`.

## Pagination

- [x] **API-3a** Add a `Limit/Offset` query-param helper and apply to: workspace list, project list, capability list, version list, execution list, alert rule list, alert list, webhook list, dataset list, precondition list, eval run list, vault key list, user list.
  - **Where**: `internal/api/handlers_capabilities.go`, `handlers_alerting.go`, `handlers_webhooks.go`, `handlers_harness.go`, `handlers_users.go`, `handlers_vault.go`, `internal/api/pagination.go` (new).

- [ ] **API-3b** Add a `Link` header (RFC 5988) on every paginated list endpoint.
  - **Where**: new helper `internal/api/pagination.go`.

- [x] **API-3c** Standardise on limit cap 1000, default 50, error on `limit<1` or `limit>1000`.
  - **Where**: every list handler.

## Error model

- [ ] **API-4a** Add a `translateDBError(err) error` helper that maps `sql.ErrNoRows` → 404, `sql.ErrTxDone` → 500, foreign-key violation → 409. Replace every `if err != nil { return ErrNotFound }` with this helper.
  - **Where**: `internal/api/handlers_capabilities.go` (15+ sites), `handlers_harness.go`, `handlers_users.go`, `handlers_vault.go`, `handlers_alerting.go`, `handlers_traces.go`, `handlers_dashboard.go`.

- [ ] **API-4b** Wrap DB errors with `%w` so the new helper can `errors.As` them.
  - **Where**: every `*SQLite` method that returns raw errors.

## Validation

- [ ] **API-VAL-1** Add a shared `validateJSON(r, &req)` helper that enforces: required fields, enum values, length limits. Use it in every handler.
  - **Where**: `internal/api/validate.go` (new).

- [ ] **API-VAL-2** Validate `req.Version > 0` on capability version creation.
  - **Where**: `internal/api/handlers_capabilities.go:329`.

- [ ] **API-VAL-3** Validate `req.Owner != ""` and `req.Owner` references an existing user on capability creation.
  - **Where**: `internal/api/handlers_capabilities.go:222`.

- [ ] **API-VAL-4** Validate `req.Severity` against the closed set in alerting handlers.
  - **Where**: `internal/api/handlers_alerting.go:24`.

- [ ] **API-VAL-5** Validate `req.Threshold > 0` for alert rules.
  - **Where**: `internal/api/handlers_alerting.go:24`.

- [x] **API-VAL-6** Validate `req.Email` format and `req.Role` against the closed set on user create/update.
  - **Where**: `internal/api/handlers_users.go`.

- [ ] **API-VAL-7** Validate `req.Events` against registered event types on webhook create.
  - **Where**: `internal/api/handlers_webhooks.go`.

- [ ] **API-VAL-8** Validate `def.Steps` non-empty on workflow run.
  - **Where**: `internal/api/handlers_workflow.go`.

## Idempotency

- [ ] **API-IDEMP-1** Replace the in-memory `idempotencyCache` with a SQLite-backed store so multi-replica deployments share state.
  - **Where**: `internal/api/idempotency.go:68-80` and new `internal/store/idempotency.sql.go`.

- [ ] **API-IDEMP-2** Fix the `c.order` slice leak — when `get` evicts, also remove from `c.order`.
  - **Where**: `internal/api/idempotency.go:54-66`.

- [ ] **PERF-7** Stream-hash the request body instead of buffering 10 MB into memory.
  - **Where**: `internal/api/idempotency.go:138-143`.

## Audit on auth-relevant mutations

- [ ] **SEC-9a** Add audit entries for API key mint, revoke, notification-group add, webhook create/delete, OAuth callback success. (See Phase 1.)

## Manager-not-configured consistency

- [x] **API-CONS-1** Standardise the "manager not configured" response across alerting/webhook/health. Pick one: always 503.
  - **Where**: `internal/api/handlers_alerting.go`, `handlers_webhooks.go`, `handlers_workflow.go`.

- [ ] **API-CONS-2** Standardise DELETE behaviour: always `204 No Content`. Remove `200 OK` with `{"deleted": id}` from webhook delete.
  - **Where**: `internal/api/handlers_webhooks.go:70`.

## OpenAPI SDK

- [ ] **API-7a** Generate the Python SDK from `api/openapi.yaml` via `openapi-python-client`. Remove the hand-written `src/promptsheon/`.
  - **Where**: `sdk/python/scripts/codegen.sh` (already exists; wire into CI).

- [ ] **API-7b** Generate the TypeScript SDK from `api/openapi.yaml` via `openapi-typescript`. Replace the placeholder `src/openapi.ts`.
  - **Where**: `sdk/typescript/scripts/codegen.sh`.

- [ ] **API-7c** Add CI jobs that run `make sdk-python` and `make sdk-typescript` then `pytest` / `tsc --noEmit`.
  - **Where**: `.github/workflows/ci.yaml`.

## Bash example

- [ ] **API-6** Fix `examples/bash/invoke-release.sh:35` to post to `/api/v1/releases/.../invoke`.
  - **Accept**: `./invoke-release.sh <release-id>` succeeds against a running daemon.

- [ ] **API-6b** Add a smoke-test script that runs every `examples/bash/*.sh` against a freshly-started daemon.
  - **Where**: `tests/smoke/` (new) and `.github/workflows/ci.yaml`.

## Response shape

- [ ] **API-RESP-1** Replace `[]any{}` returns with typed empty slices (`[]*Workspace{}`).
  - **Where**: `internal/api/handlers_alerting.go`, `handlers_webhooks.go`, `handlers_providers.go`.

- [ ] **API-RESP-2** Remove the unused inner type declarations and unused import workarounds (`var _ = fmt.Sprintf`).
  - **Where**: `internal/api/handlers_capabilities.go:30`, `handlers_harness.go:286`, `handlers_workflow.go:45`, `idempotency.go:183`.

## Test endpoint

- [ ] **API-PROV-1** Drop `handleTestProvider` defaults-to-`gpt-3.5-turbo` behaviour; require explicit `model`.
  - **Where**: `internal/api/handlers_providers.go:60`.

## Health endpoints

- [ ] **API-HEALTH-1** Add `/livez` (alias for `/health`) and keep `/health` as the liveness probe.
- [ ] **API-HEALTH-2** Add `/readyz` (alias for `/ready`) and keep `/ready` as the readiness probe.

## SDK endpoint alignment

- [ ] **API-SDK-1** Audit the Go SDK for endpoints exposed in OpenAPI but missing. Add `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey`, OAuth start/callback, `UpdatePrecondition`.
  - **Where**: `sdk/client.go` (712 LOC).
