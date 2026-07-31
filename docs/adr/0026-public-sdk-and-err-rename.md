# ADR-0026: Public SDK surface and Error* → Err* rename

- **Status:** Accepted, 2026-08-01
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

### 1. Single canonical import path

The public Go SDK now lives at
`github.com/sachncs/promptsheon/pkg/promptsheon`. The legacy
`github.com/sachncs/promptsheon/sdk` path is kept as a
**backward-compatibility shim**: type aliases re-export the same
structs (no behaviour change). A future v1.1.0 may deprecate the
`sdk/` path with a `// Deprecated:` notice.

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

All sentinels in `backend/errs/` follow `Err*`. The original
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

- Consumers get a single canonical import path with a build-tag
  fence.
- No external breakage from the sentinel rename (no external
  consumers confirmed in c0.0).
- The daemon binary stays lean — `go build ./...` excludes the
  facade.
- The `s.repos → s.db` rename is internal-only.
- Future deprecation of `sdk/` path is straightforward (just delete
  the alias).

## References

- PLAN-49 audit: `docs/research/audit-2026-07-26.md`
- Build-tag fence: `pkg/promptsheon/*.go` (`//go:build promptsheon`)
- Makefile targets: `build-public`, `check-public`, `build-e2e`
- Renames: commits `1868370` (initial sweep) + `021d03d` (follow-up)
- Backward-compat shim: `sdk/client.go` (kept as a re-export layer)
