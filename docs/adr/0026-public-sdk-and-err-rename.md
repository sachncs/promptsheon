# ADR-0026: Public SDK surface and Error* → Err* rename

- **Status:** Accepted, 2026-08-01
- **Updated:** 2026-08-01 (L-1: sdk/ removed entirely)
- **Context:** PLAN-49 v1.0.0 release

## Context

The PLAN-49 audit found:
1. **No stable public import path.** The canonical Go SDK lived
   under `github.com/sachncs/promptsheon/sdk`, an internal package
   path. External consumers had no fence; a refactor inside `sdk/`
   could break their builds.
2. **Non-idiomatic error naming.** 35+ sentinels used the `Error*`
   prefix (`ErrorApprovalNotFound`, `ErrorBudgetCapExceeded`,
   etc.). Go convention is `Err*`.
3. **Two competing routes to the same surface.** A would-be consumer
   could `import ".../sdk"`, but the SDK also lived partially under
   `pkg/promptsheon/` (started in v0.1.x, abandoned mid-flight).
4. **Build-tag discipline missing.** The previous daemon binary
   shipped the SDK facade source in its compile graph even though
   it wasn't used at runtime — a small but real cost.

## Decision

### 1. Single canonical import path; sdk/ removed

The public Go SDK now lives at
`github.com/sachncs/promptsheon/pkg/promptsheon`. v1.0.0
**removes** the legacy `github.com/sachncs/promptsheon/sdk`
package entirely (L-1: the user requested "no backward shims,
breaking changes welcomed"). The facade is no longer a
re-export — it is the implementation.

Migration:

    import "github.com/sachncs/promptsheon/sdk"
becomes
    import "github.com/sachncs/promptsheon/pkg/promptsheon"

The change is mechanical: every exported symbol in sdk/ is
present in pkg/promptsheon with the same name and behaviour. A
codemod is provided in `tools/codemod-sdktopkgshe-on/`.

Why remove sdk/ rather than deprecate it? The legacy path made
every internal type public, which prevented refactors inside
promptsheon/. The //go:build promptsheon fence in pkg/promptsheon is
the only public surface; sdk/ would have been a backdoor.

### 2. Build-tag fence

`pkg/promptsheon/*.go` all carry:

```go
//go:build promptsheon
```

Default `go build ./...` and `go test ./...` skip the facade. To
build the SDK consumer package, set `GOFLAGS=-tags=promptsheon`
(or use the `make build-public` / `make check-public` targets).

Rationale: keeps the daemon's main binary lean, prevents
accidental import of facade-only types into the runtime.

### 3. Error sentinel naming

All sentinels in `promptsheon/errs/` follow `Err*`. The original
`Error*` prefix was non-standard. 35 sentinels renamed across
PR-2 (commit 1868370) and PR-6 (commit 021d03d, the follow-up
sweep). Renames:

| Old                                | New                              |
|------------------------------------|----------------------------------|
| `ErrorApprovalCreatorVoted`        | `ErrSelfVote`                    |
| `ErrorApprovalQuorumNotMet`        | `ErrQuorum`                      |
| `ErrorBudgetCapExceeded`           | `ErrBudget`                      |
| `ErrorBudgetCapNotPositive`        | `ErrBudgetInvalid`               |
| `ErrorCapabilityInvalidBlastRadius`| `ErrInvalidBlastRadius`         |
| `ErrorCapabilityEmptyContract`     | `ErrEmptyContract`               |
| `ErrorCapabilityInheritanceTooDeep`| `ErrInheritanceTooDeep`          |
| `ErrorCapabilityEmptyManifest`     | `ErrEmptyManifest`               |
| `ErrorContextBudgetExhausted`      | `ErrContextExhausted`            |
| `ErrorElectionNotLeader`           | `ErrNotLeader`                    |
| `ErrorEventBusAlreadyCanceled`     | `ErrEventBusCanceled`            |
| `ErrorHarnessPreconditionFailed`   | `ErrPrecondition`                |
| `ErrorLineageUnknownSource`        | `ErrLineageUnknown`              |
| `ErrorLineageSelfReference`        | `ErrLineageSelfRef`              |
| `ErrorLineageDuplicateEdge`        | `ErrLineageDuplicate`            |
| `ErrorLineageInconsistentCapability`| `ErrLineageInconsistent`        |
| `ErrorMCPEmptyName` / `BadName` / `BadURL` / `UnknownName` | `ErrMCP*` |
| `ErrorManifestEmpty` / `BadName` / `BadUDS`               | `ErrManifest*` |
| `ErrorQuotaLimitNotPositive`        | `ErrQuotaInvalid`                |
| `ErrorQuotaOverLimit`              | `ErrQuota`                       |
| `ErrorReasoningConstraintViolation`| `ErrReasoningConstraintViolation` |
| `ErrorRecommendationUnknownOutcome`| `ErrRecommendationUnknown`      |
| `ErrorReleaseNotPending`           | `ErrReleaseNotPending`           |
| `ErrorScheduleInvalidCron`         | `ErrInvalidCron`                 |
| `ErrorStoreConflict`               | `ErrStoreConflict`               |
| `ErrorStoreIdempotencyMiss`        | `ErrStoreIdempotencyMiss`        |
| `ErrorVaultKeyUnavailable`         | `ErrVaultKeyUnavail`             |
| `ErrorVaultKMSClientRequired`      | `ErrKMSClient`                   |

No alias period. Direct, no-deprecation shim. The audit confirmed
in c0.0 that no external consumers depend on the old names.

### 4. Repositories → DB god-struct

The `Repositories` god-struct on `store` (which embedded every
repository interface for handler convenience) was renamed to
`DB`. The `NewRepositories` constructor became `NewDB`. The
field on `Server` (formerly `s.repos`) is now `s.db *store.DB`.
22 files updated.

Rationale: a single-word god-struct is consistent with the rest
of the codebase (`Server`, `Builder`, `Registry`, ...). The
qualified name `store.DB` is short and unambiguous inside the
package.

## Consequences

- One canonical import path: `github.com/sachncs/promptsheon/pkg/promptsheon`.
- The legacy `sdk/` package is removed; `go get
  github.com/sachncs/promptsheon/sdk` returns "module ... not
  found".
- No external breakage from the sentinel rename (no external
  consumers confirmed in c0.0).
- The daemon binary stays lean — `go build ./...` excludes the
  facade.
- The `s.repos → s.db` rename is internal-only.

## References

- PLAN-49 audit: `docs/research/audit-2026-07-26.md`
- Build-tag fence: `pkg/promptsheon/*.go` (`//go:build promptsheon`)
- Makefile targets: `build-public`, `check-public`, `build-e2e`
- Renames: commits `1868370` (initial sweep) + `021d03d` (follow-up)
- SDK location: `pkg/promptsheon/` (was `sdk/`, removed in v1.0.0)
