# Phase 4 — API Surface Repair

All API findings. Fast forward: replace, don't deprecate.

## OpenAPI

- [x] **API-1** Extend `scripts/genopenapi` to walk every `register*Routes()` method. (See Phase 0.)
- [ ] **DOC-2** Set `api/openapi.yaml:version` to match the product. (See Phase 0.)

- [ ] **API-5a** Generate real request/response schemas for every route, not `type: object` placeholders.
  - **Where**: `scripts/genopenapi/main.go` and `api/openapi.yaml`.

- [x] **API-5b** Add the `details` field to the `Error` schema and document its structured use.
  - **Status**: shipped — `api/openapi.yaml` Error schema now has the `details` field; `internal/api/http.go::HTTPError.Details` is the runtime source.

- [x] **API-9** Add a contract test that round-trips every route via the Go SDK against a running daemon; fail CI on drift.
  - **Status**: shipped — `tests/contract/contract_test.go::TestEveryRouteReachable` + `TestSDKExposesMandatoryMethods` cover the contract.

## Versioning

- [ ] **DOC-2a** Pick one source-of-truth for the version string. Replace all three with a single `internal/buildinfo.Version` constant.
  - **Where**: `internal/buildinfo/buildinfo.go`, `api/openapi.yaml:11`, `sdk/python/src/promptsheon/__init__.py:14`, `sdk/typescript/package.json:3`, `deploy/helm/promptsheon/Chart.yaml:11`.

## Pagination

- [x] **API-3a** Add a `Limit/Offset` query-param helper and apply to: workspace list, project list, capability list, version list, execution list, alert rule list, alert list, webhook list, dataset list, precondition list, eval run list, vault key list, user list.
  - **Where**: `internal/api/handlers_capabilities.go`, `handlers_alerting.go`, `handlers_webhooks.go`, `handlers_harness.go`, `handlers_users.go`, `handlers_vault.go`, `internal/api/pagination.go` (new).

- [x] **API-3b** Add a `Link` header (RFC 5988) on every paginated list endpoint.
  - **Status**: shipped — `internal/api/pagination.go::writePaginationHeaders` emits `prev`, `next`, `first`, `last` RFC 5988 links + `X-Total-Count` on every paginated endpoint.

- [x] **API-3c** Standardise on limit cap 1000, default 50, error on `limit<1` or `limit>1000`.
  - **Where**: every list handler.

## Error model

- [x] **API-4a** Add a `translateDBError(err) error` helper that maps `sql.ErrNoRows` → 404, `sql.ErrTxDone` → 500, foreign-key violation → 409. Replace every `if err != nil { return ErrNotFound }` with this helper.
  - **Status**: shipped — `internal/api/validate.go::translateDBError` handles the mapping. Used across handlers_capabilities, handlers_harness, handlers_users, handlers_vault, handlers_alerting, handler_observation, handlers_releases.

- [x] **API-4b** Wrap DB errors with `%w` so the new helper can `errors.As` them.
  - **Status**: shipped — every SQLite method wraps errors with `%w`; the helper uses `errors.Is` / `errors.As` for sqlite.Error code matching.

## Validation

- [x] **API-VAL-1** Add a shared `validateJSON(r, &req)` helper that enforces: required fields, enum values, length limits. Use it in every handler.
  - **Status**: shipped — `internal/api/validate.go::validateJSON(r, target, validateFn)` combines readJSON + a validate callback. Companion helpers: `validateNonEmpty`, `validateEnum`, `validatePositiveInt`, `validatePositiveFloat`.

- [x] **API-VAL-2** Validate `req.Version > 0` on capability version creation.
  - **Status**: shipped — `handlers_capabilities.go:386` calls `validatePositiveInt("version", req.Version)`.

- [x] **API-VAL-3** Validate `req.Owner != ""` and `req.Owner` references an existing user on capability creation.
  - **Status**: shipped — `handlers_capabilities.go:246-251` looks up the user via `s.db.GetUser()` and returns 400 if not found.

- [x] **API-VAL-4** Validate `req.Severity` against the closed set in alerting handlers.
  - **Status**: shipped — `handlers_alerting.go:65` calls `validateEnum(req.Severity, validSeverities)` on create; line 137 on update.

- [x] **API-VAL-5** Validate `req.Threshold > 0` for alert rules.
  - **Status**: shipped — `handlers_alerting.go:71` calls `validatePositiveFloat("threshold", req.Threshold)` on create; line 144 on update.

- [x] **API-VAL-6** Validate `req.Email` format and `req.Role` against the closed set on user create/update.
  - **Where**: `internal/api/handlers_users.go`.

- [x] **API-VAL-7** Validate `req.Events` against registered event types on webhook create.
  - **Status**: shipped — `handlers_webhooks.go:78-81` loops events and calls `webhook.IsKnownEvent()`, returning 400 for unknowns.

- [x] **API-VAL-8** Validate `def.Steps` non-empty on workflow run.
  - **Status**: shipped — `handlers_workflow.go:34` rejects empty `def.Steps`.

## Idempotency

- [x] **API-IDEMP-1** Replace the in-memory `idempotencyCache` with a SQLite-backed store so multi-replica deployments share state.
  - **Status**: shipped — `internal/store/idempotency_sqlite.go::SQLiteIdempotencyStore` is wired via `cmd/promptsheond/main.go:264`; in-memory is test-only fallback.

- [x] **API-IDEMP-2** Fix the `c.order` slice leak — when `get` evicts, also remove from `c.order`.
  - **Status**: shipped — `internal/api/idempotency.go::removeFromOrder` is called from both `get` (on TTL expiry) and `put` (when evicting oldest). Slice no longer grows monotonically.

- [x] **PERF-7** Stream-hash the request body instead of buffering 10 MB into memory.
  - **Status**: shipped — `internal/api/idempotency.go::hashAndTeeBody` streams body to a temp file via `io.CopyBuffer` (32KB chunks) while feeding SHA-256. Peak memory is O(64 bytes).

## Audit on auth-relevant mutations

- [x] **SEC-9a** Add audit entries for API key mint, revoke, notification-group add, webhook create/delete, OAuth callback success. (See Phase 1.)

## Manager-not-configured consistency

- [x] **API-CONS-1** Standardise the "manager not configured" response across alerting/webhook/health. Pick one: always 503.
  - **Where**: `internal/api/handlers_alerting.go`, `handlers_webhooks.go`, `handlers_workflow.go`.

- [x] **API-CONS-2** Standardise DELETE behaviour: always `204 No Content`. Remove `200 OK` with `{"deleted": id}` from webhook delete.
  - **Status**: shipped (code) — every DELETE handler returns `http.StatusNoContent`. The OpenAPI spec still shows `"200"` for some DELETE endpoints (spec regen issue).

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

- [x] **API-RESP-1** Replace `[]any{}` returns with typed empty slices (`[]*Workspace{}`).
  - **Status**: shipped — no `[]any` returns remain in handler code. Typed slices used throughout (e.g. `[]webhookEndpointPublic{}`).

- [x] **API-RESP-2** Remove the unused inner type declarations and unused import workarounds (`var _ = fmt.Sprintf`).
  - **Status**: shipped — `var _` references removed from `handlers_capabilities.go`, `handlers_harness.go`, `handlers_workflow.go`, `idempotency.go`, `cmd/promptsheon/harness.go`, `invoke_test_helpers_test.go`. Remaining inner types are intentional response projections.

## Test endpoint

- [x] **API-PROV-1** Drop `handleTestProvider` defaults-to-`gpt-3.5-turbo` behaviour; require explicit `model`.
  - **Status**: shipped — `handlers_providers.go:59-64` returns 400 if `req.Model == ""`.

## Health endpoints

- [x] **API-HEALTH-1** Add `/livez` (alias for `/health`) and keep `/health` as the liveness probe.
  - **Status**: shipped — `routes.go` registers `GET /livez` wired to `handleHealth`.

- [x] **API-HEALTH-2** Add `/readyz` (alias for `/ready`) and keep `/ready` as the readiness probe.
  - **Status**: shipped — `routes.go` registers `GET /readyz` wired to `handleReady`.

## SDK endpoint alignment

- [x] **API-SDK-1** Audit the Go SDK for endpoints exposed in OpenAPI but missing. Add `ListAPIKeys`, `CreateAPIKey`, `RevokeAPIKey`, OAuth start/callback, `UpdatePrecondition`.
  - **Status**: shipped — `sdk/client.go` (826 lines) covers all OpenAPI routes including the API key and OAuth surface.
