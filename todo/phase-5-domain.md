# Phase 5 — Domain Cleanup

All domain/dead-code findings. Fast forward: delete, don't keep around "for compatibility."

## Capability

- [ ] **DEAD-1a** Delete `Observation` (`capability/observation.go`) — never called.
- [ ] **DEAD-1b** Delete `EvaluationResult` (`capability/evaluationresult.go`) — never called.
- [ ] **DEAD-1c** Delete `ReleaseProbe`, `ReleaseStatusValue`, the three `ReleaseStatus*` constants, and `DeriveState` (`capability/capability.go:101-113, 56-95`) — never called.
- [ ] **DEAD-1d** Delete the unused `EventType` constants: `EventCapabilityCreated`, `EventCapabilityUpdated`, `EventCapabilityArchived`, `EventVersionCreated`, `EventVersionPromoted`, `EventEvaluationCompleted`, `EventEvaluationThresholdsMet`, `EventObservationGenerated`, `EventRegressionDetected`, `EventRollbackPerformed`, `EventDeploymentRolledBack`.
  - **Where**: `internal/capability/event.go`.

- [ ] **DEAD-2** Remove `req.State = capability.StateDraft` from `handlers_capabilities.go:238`.
  - **Where**: `internal/api/handlers_capabilities.go:238`.

## Release

- [ ] **DEAD-3a** Delete `AllEnvironments()` (`release/release.go:62-64`).
- [ ] **DEAD-3b** Delete `Repository.DeleteRelease` from interface + SQLite impl + handler. Same for `Repository.DeleteApproval`.
  - **Where**: `internal/release/repo.go`, `internal/store/sqlite_releases.go:160, 353`.

- [ ] **DEAD-6** Delete the `runner` interface in `release/service.go:71-83`. Keep the `var _ Service` assertion only if the API layer actually depends on it; otherwise remove.
  - **Where**: `internal/release/service.go`.

- [ ] **DEAD-Rel-1** Delete the deprecated `Release.Approve(...)` wrapper at `release/release.go:208-210`.
  - **Accept**: The codebase no longer compiles against the deprecated entry point.

- [ ] **DEAD-Rel-2** Delete the `castApprovesToVotes` helper used only by the deprecated `ApproveWithApprovalList`.
  - **Where**: `internal/release/release.go:212-223`.

- [ ] **DEAD-Rel-3** Delete `PolicyKind` enum; callers must pass a concrete `approval.Policy`.
  - **Where**: `internal/release/service.go:18-57` and `cmd/promptsheond/main.go:419-431`.

- [ ] **BUG-1** Rename the shadowed `s` variable in `service.go:197` to `succeeded` so the code reads cleanly.
  - **Where**: `internal/release/service.go:197`.

## Approval

- [ ] **DEAD-4** Delete `Repository.DeleteApproval` from interface + SQLite impl.
- [ ] **SEC-1** Make `MakerCheckerPolicy` enforce separation-of-duties internally. (See Phase 1.)
- [ ] **DOC-1** Fix the doc comment at `approval.go:152-155` to describe the actual fail-closed behaviour.

## Harness

- [ ] **DEAD-5** Move `EnvAllowlist` from a package-level var to a method param or a config value.
  - **Where**: `internal/harness/precondition.go:13-39`.
  - **Accept**: Operators can extend the allowlist without editing source.

- [ ] **BUG-2** Decide a single policy on `Precondition.TimeoutSec`: either `Validate` accepts `0` as "use 60s default" or `runOne` enforces non-zero. Pick one and update both.
  - **Where**: `internal/harness/precondition.go:167-170` and `internal/harness/harness.go:95-109`.

- [ ] **TEST-3** Consolidate the two in-memory `Repository` fixtures into one shared `internal/testutil/harnessrepo` package.

## Eval

- [ ] **SEC-3** Make `JSONSchema.ScoreCase` reject unsupported keywords. (See Phase 1.)

## Manifest

- [ ] **DEAD-Manifest-1** Rename package `internal/manifest` to `internal/pluginmanifest` so it doesn't collide with `capability.Manifest`.
  - **Where**: `internal/manifest/`, all importers.

## Store

- [ ] **DEAD-7** Delete `marshalField` legacy helper in `sqlite_capabilities.go:263`.

- [ ] **DEAD-Store-1** Delete unused `GetProviderKey`, `GetProviderKeyByName`, `GetAlertRule`, `GetAlert`, `GetNotificationGroup`, `DeleteNotificationGroup`, `GetWebhookEndpoint` from the SQLite impl and the `store.Repository` interface. Keep `DeleteWebhookEndpoint` (called via interface).
  - **Where**: `internal/store/sqlite.go`, `internal/store/repo.go`.

- [ ] **DEAD-Store-2** Delete unused `CreateSchedule`, `GetSchedule`, `ListSchedulesForRelease`, `DeleteSchedule` from the harness `Repository` impl.
  - **Where**: `internal/store/sqlite_capabilities.go`.

## API

- [ ] **DEAD-API-1** Delete the duplicated `WithRequest` doc comment in `server.go:143-144`.

- [ ] **DEAD-API-2** Delete the `ClassifyInvokeError` indirection layer — it's a single switch; inline it.

- [ ] **DEAD-CLI-1** `cmd/promptsheon-auditbackfill/main.go` — document its purpose and lifecycle, or delete it.
  - **Where**: `cmd/promptsheon-auditbackfill/`.

## Plugin supervisor

- [ ] **DEAD-Plg-1** Drop `Plugin.Binary.Stop`/`Restart` symmetry (currently `Restart` can never succeed because `Stop` sets `stopped=true` permanently).
  - **Where**: `internal/subprocess/subprocess.go:88-180` and `internal/supervisor/supervisor.go:180-216`.

- [ ] **DEAD-Plg-2** Wire the supervisor's lifecycle events to a real publisher instead of nil.
  - **Where**: `cmd/promptsheond/main.go:124-133` and `internal/supervisor/supervisor.go:244-249`.

- [ ] **DEAD-Plg-3** Fix the `net/rpc` reply type mismatch (`*string` vs `*HealthReply`/`*StopReply`).
  - **Where**: `internal/subprocess/subprocess.go:153-189, 267-289`.

## Backwards-compat removals (fast forward)

- [ ] **FC-1** Remove all `Deprecated:` godoc comments and the matching deprecated methods/types from the public API surface.
- [ ] **FC-2** Remove the `WithServerConfig`/`*ServerConfig` history comments.
  - **Where**: `internal/api/server.go:285-291`.
- [ ] **FC-3** Remove the `LLMFallback` config field (no consumer).
  - **Where**: `internal/config/config.go:43` and `cmd/promptsheond/main.go:123`.
- [ ] **FC-4** Remove the `migration_test.go` summary file (replaced by per-migration tests).
  - **Where**: `internal/store/migration_test.go`.
- [ ] **FC-5** Remove the `splitResource` helper in `internal/store/split_resource.go` (one-off migration helper, no longer needed).
