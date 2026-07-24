# Phase 9 — Bug Fixes

Concrete bugs. Each is a one-line or one-function change.

- [x] **BUG-1** Rename the shadowed `s` variable in `release/service.go:197` to `succeeded`.
  - **Where**: `internal/release/service.go:197`.

- [x] **BUG-2** Single policy on `Precondition.TimeoutSec`. Pick: `Validate` accepts 0 as "default 60s"; `runOne` no longer defaults.
  - **Where**: `internal/harness/precondition.go:167-170` and `internal/harness/harness.go:95-109`.

- [x] **BUG-3** `VerifyAuditChain` cross-checks against `audit_chain_state` and rowid sequence. (See Phase 1 SEC-CHAIN-1.)

- [x] **BUG-4** Seed system user `id="api"` for audit FKs. (See Phase 1 SEC-DB-1.)

- [x] **BUG-5** `Recovery` middleware writes JSON, not text/plain. (See Phase 1 SEC-8a.)

- [x] **BUG-6** Offset-only pagination SQL fix. (See Phase 2 DB-9a.)

- [x] **BUG-7** Replace `generateID` (UnixNano only) with ULID via `github.com/oklog/ulid/v2`.
  - **Where**: `internal/api/helpers.go:8`.

- [x] **BUG-8** Replace `strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")` with `r.PathValue("id")` in `handleRevokeAPIKey`.
  - **Where**: `internal/api/handlers_auth.go:450`.

- [x] **BUG-9** Wrap a panic-safe `recover` so the response writer's already-flushed headers don't break the JSON envelope.
  - **Where**: `internal/api/middleware.go:110-130`.

- [x] **BUG-10** Add nil-guards on `s.spans` in `handlers_traces.go:37,50,60` and `handlers_dashboard.go:15`.
  - **Accept**: Calling any trace handler with `WithTracing` not set returns 503.

- [x] **BUG-11** Add nil-guard on `s.invoker` in every handler that calls it.
  - **Where**: `internal/api/handlers_capabilities.go:529, 544`.

- [x] **BUG-12** Drop the `var _ = fmt.Sprintf` / `var _ = context.Background` / `var _ = json.Marshal` import workarounds; either use the symbols or drop the imports.
  - **Where**: `internal/api/handlers_capabilities.go:30`, `handlers_harness.go:286`, `handlers_workflow.go:45`, `idempotency.go:183`.

- [x] **BUG-13** Fix the duplicate env vars in `tests/e2e/daemon_e2e_test.go:119-126`.
  - **Accept**: Each env var is `t.Setenv`'d exactly once.

- [x] **BUG-14** Drop the duplicated `// WithTracing attaches a trace store and metrics collector to the server.` doc comment at `internal/api/server.go:143-144`.

- [x] **BUG-15** `handleTestProvider` requires explicit `model`; drop the `gpt-3.5-turbo` default.
  - **Where**: `internal/api/handlers_providers.go:60`.

- [x] **BUG-16** `handleDeleteWebhook` returns `204 No Content` (not `200 OK` with `{"deleted": id}`).
  - **Where**: `internal/api/handlers_webhooks.go:70`.

- [x] **BUG-17** `handleDeleteAlertRule` distinguishes 204 (deleted) from 404 (never existed).

- [x] **BUG-18** Audit log dropped entries are surfaced as a metric and a log line per drop.
  - **Where**: `internal/api/server.go:614-618`.

- [x] **BUG-19** `cmd/promptsheond/main.go:330-350` silently swallows `inv.Provider == ""` with a generic error — make the error type and the route's response specific (502 Bad Gateway with a `provider_missing` detail).
  - **Where**: `cmd/promptsheond/main.go:336-345` and `internal/api/handlers_releases.go:201`.

- [x] **BUG-20** `manifestHashForRelease` returns `""` on marshal failure — log the failure and return an error to the handler instead of silently dropping audit data.
  - **Where**: `internal/api/handlers_releases.go:320` and `internal/api/handlers_capabilities.go:287`.

- [x] **BUG-21** `invokeOneWithManifest` records `exec.TotalTokens = 0` and `exec.CostUSD = 0` on failed invokes. Add a `tokens_estimated` field that's populated even on failure when possible.
  - **Where**: `internal/api/handlers_capabilities.go:457-458`.

- [x] **BUG-22** `handleListAudit` silently ignores malformed `limit`/`offset` — return 400 instead.
  - **Where**: `internal/api/handlers_audit.go:39-49`.

- [x] **BUG-23** `handleListSpans` and `handleSearchSpans` silently ignore malformed `since`/`until` — return 400.
  - **Where**: `internal/api/handlers_traces.go:19-35`, `handlers_dashboard.go:76-85`.

- [x] **BUG-24** `Strict-Transport-Security` header is set unconditionally. Only set when TLS is configured.
  - **Where**: `internal/api/middleware.go:193`.

- [x] **BUG-25** `handleGetTraceTree` returns `ErrNotFound` for both "trace missing" and "internal error" — split into 404 vs 500.
  - **Where**: `internal/api/handlers_traces.go:58-64`.

- [x] **BUG-26** `handleGetEval` and similar return `ErrNotFound` for any DB error — split via `translateDBError`.
  - **Where**: `internal/api/handlers_harness.go:272`.

- [x] **BUG-27** Drop `handleBootstrap`'s manual `r.Method != http.MethodPost` check — `mux.HandleFunc("POST ...")` already enforces it.
  - **Where**: `internal/api/handlers_auth.go:286-287`.

- [x] **BUG-28** `handleObservation`'s defensive `if id == ""` check is unreachable (`{id}` never matches empty). Delete it.
  - **Where**: `internal/api/handler_observation.go:21`.

- [x] **BUG-29** `handleWorkflowRun` returns 503 when engine nil; match that pattern across alerting/webhook/health instead of the inconsistent 200/empty and 400/error mix.
  - **Where**: `internal/api/handlers_alerting.go`, `handlers_webhooks.go`.

- [x] **BUG-30** `handleCreateExecution` records the failure execution row with `TotalTokens=0, CostUSD=0` — populate the actual values when partial response is available.
  - **Where**: `internal/api/handlers_capabilities.go:440-450`.
