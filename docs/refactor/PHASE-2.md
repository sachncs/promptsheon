# PHASE-2 — Public Surface + Dead Deps + Err Rename

**25 commits.** Drops 12 unused dependencies, renames 14 error sentinels,
consolidates the audit-key constants, and removes dead twin functions.

## Commits

```
c2.0  chore(go.mod): drop ClickHouse clickhouse-go/v2 + ch-go
      Refs: PLAN-49/
c2.1  chore(go.mod): drop aws-sdk-go-v2 config + kms
      Refs: PLAN-49/
c2.2  chore(go.mod): drop google/uuid, dustin/go-humanize, cenkalti/backoff
      Refs: PLAN-49/
c2.3  chore(go.mod): drop grpc-gateway, jsonschema, klauspost/compress
      Refs: PLAN-49/
c2.4  chore(go.mod): drop ordered-map, lz4, yaml/v3, yaml/v4
      Refs: PLAN-49/
c2.5  chore(go.mod): drop modelcontextprotocol/go-sdk
      Refs: PLAN-49/
c2.6  chore(deps): go mod tidy
      Refs: PLAN-49/M-2
c2.7  chore(vendor): go mod vendor -reuse
      Refs: PLAN-49/L-3
c2.8  refactor(errs): rename Error* → Err* (14 sentinels, direct no alias)
      Refs: PLAN-49/M-1
c2.9  refactor(store): Repositories → DB (no alias)
      Refs: PLAN-49/H-4
c2.10 refactor(server): auditDefaultUser → audit.AnonUser
      Refs: PLAN-49/P0-4
c2.11 refactor(middleware): auditKey*/field* consolidation to audit.Field*
      Refs: PLAN-49/P0-5
c2.12 refactor(auth): drop NewAuthenticatorWithLogger
      Refs: PLAN-49/P0-2
c2.13 refactor(alerting): merge NewManager + NewManagerWithDB; drop SetDeliveryFunc
      Refs: PLAN-49/P1-12 P1-13
c2.14 refactor(webhook): drop WithAllowInsecure
      Refs: PLAN-49/3.3
c2.15 refactor(auth): drop activeOAuthStates global; route via Server.oauthStates
      Refs: PLAN-49/P0-1
c2.16 refactor(handlers): drop inputHash/modelRevision/errProviderMissing twins
      Refs: PLAN-49/P0-3 P0-8
c2.17 refactor(handlers): drop handlers_helpers.go; expand capability/hash.go with InputHash/ModelRevision
      Refs: PLAN-49/H-6
c2.18 refactor(handlers): drop marshal_helper.go (rename marshalNoArgs inline)
      Refs: PLAN-49/P1
c2.19 refactor(handlers): drop embed_frontend.go (inline //go:embed)
      Refs: PLAN-49/
c2.20 refactor(cmd): drop daemon_evolver_adapter.go + daemon_release_invoker.go
      Refs: PLAN-49/P0-7
c2.21 refactor(backend): drop UsageTracker + /api/v1/metrics/top-capabilities
      Refs: PLAN-49/
c2.22 refactor(invoke): drop PersistedEnforcer; fold into DefaultEnforcer
      Refs: PLAN-49/
c2.23 refactor(llm): drop tunedTransport, PricingTable + 5 helpers, EstimateTokens, envJudgeProvider
      Refs: PLAN-49/
c2.24 refactor(llm): drop AggregateMetrics, JudgeClient alias, ValidateBaseURLs
      Refs: PLAN-49/
```

## Files touched

| File | Commits |
|---|---|
| `go.mod`, `go.sum` | c2.0-c2.7 |
| `vendor/modules.txt`, `vendor/**` | c2.7 |
| `promptsheon/errs/errors.go` | c2.8 |
| `promptsheon/store/repo.go`, `promptsheon/store/sqlite.go` | c2.9 |
| `promptsheon/server.go`, `promptsheon/audit/keys.go` (new) | c2.10, c2.11 |
| `promptsheon/auth/authenticator.go` | c2.12 |
| `promptsheon/alerting/manager.go` | c2.13 |
| `promptsheon/webhook/webhook.go` | c2.14 |
| `promptsheon/handlers_auth.go` | c2.15 |
| `promptsheon/handlers_helpers.go` (delete), `promptsheon/handlers_executions.go` | c2.16 |
| `promptsheon/capability/hash.go` (expand), `promptsheon/handlers_executions.go` | c2.17 |
| `promptsheon/marshal_helper.go` (delete), `promptsheon/handlers_executions.go` | c2.18 |
| `cmd/promptsheond/embed_frontend.go` (delete), `cmd/promptsheond/main.go` | c2.19 |
| `cmd/promptsheond/daemon_evolver_adapter.go` (delete), `cmd/promptsheond/daemon_release_invoker.go` (delete), `cmd/promptsheond/daemon.go` | c2.20 |
| `promptsheon/usage.go` (delete), `promptsheon/handlers_*.go`, `promptsheon/routes.go` | c2.21 |
| `promptsheon/invoke/persisted_enforcer.go` (delete), `promptsheon/invoke/enforcer.go` | c2.22 |
| `promptsheon/llm/transport.go` (delete), `promptsheon/llm/cost.go` (delete), `promptsheon/llm/tokenizer.go` (delete), `promptsheon/llm/env.go` (delete) | c2.23 |
| `promptsheon/llm/middleware.go` (delete), `promptsheon/llm/judge.go`, `promptsheon/llm/registry.go` | c2.24 |

## Error sentinel renames (c2.8)

Direct, no alias. 14 sentinels:

| Before | After |
|---|---|
| `ErrorApprovalCreatorVoted` | `ErrSelfVote` |
| `ErrorApprovalQuorumNotMet` | `ErrQuorum` |
| `ErrorApprovalNotFound` | `ErrApprovalNotFound` |
| `ErrorApprovalUnknownDecision` | `ErrApprovalUnknown` |
| `ErrorApprovalDuplicateIdentity` | `ErrApprovalDuplicate` |
| `ErrorBudgetCapExceeded` | `ErrBudget` |
| `ErrorBudgetCapNotPositive` | `ErrBudgetInvalid` |
| `ErrorContextBudgetExhausted` | `ErrContextExhausted` |
| `ErrorElectionNotLeader` | `ErrNotLeader` |
| `ErrorEventBusAlreadyCanceled` | `ErrEventBusCanceled` |
| `ErrorExecutorProviderMissing` | `ErrProviderMissing` |
| `ErrorHarnessPreconditionFailed` | `ErrPrecondition` |
| `ErrorCapabilityInheritanceTooDeep` | `ErrInheritanceTooDeep` |
| `ErrorCapabilityEmptyManifest` | `ErrEmptyManifest` |
| `ErrorCapabilityInvalidBlastRadius` | `ErrInvalidBlastRadius` |
| `ErrorCapabilityEmptyContract` | `ErrEmptyContract` |
| `ErrorLineageUnknownSource` | `ErrLineageUnknown` |
| `ErrorLineageSelfReference` | `ErrLineageSelfRef` |
| `ErrorLineageDuplicateEdge` | `ErrLineageDuplicate` |
| `ErrorLineageInconsistentCapability` | `ErrLineageInconsistent` |
| `ErrorMCPEmptyName` | `ErrMCPEmptyName` |
| `ErrorMCPBadName` | `ErrMCPBadName` |
| `ErrorMCPBadURL` | `ErrMCPBadURL` |
| `ErrorMCPUnknownName` | `ErrMCPUnknown` |
| `ErrorManifestEmpty` | `ErrManifestEmpty` |
| `ErrorManifestBadName` | `ErrManifestBadName` |
| `ErrorManifestBadUDS` | `ErrManifestBadUDS` |
| `ErrorRecommendationUnknownOutcome` | `ErrRecommendationUnknown` |
| `ErrorRecommendationNotFound` | `ErrRecommendationNotFound` |
| `ErrorReleaseNotPending` | `ErrReleaseNotPending` |
| `ErrorScheduleInvalidCron` | `ErrInvalidCron` |
| `ErrorStoreNotFound` | `ErrStoreNotFound` |
| `ErrorStoreConflict` | `ErrStoreConflict` |
| `ErrorStoreIdempotencyMiss` | `ErrStoreIdempotencyMiss` |
| `ErrorVaultStopped` | `ErrVaultStopped` |
| `ErrorVaultUnknownSecret` | `ErrVaultUnknownSecret` |
| `ErrorVaultKeyUnavailable` | `ErrVaultKeyUnavailable` |
| `ErrorVaultKMSClientRequired` | `ErrKMSClient` |

## Repositories → DB rename (c2.9)

Direct, no alias. Touches ~50 files.

```go
// Before:
type Repositories struct { ... }
func NewRepositories(db *sql.DB) *Repositories { ... }

// After:
type DB struct { ... }
func NewDB(db *sql.DB) *DB { ... }
```

All call sites updated. `s.repos` field on `Server` becomes `s.db` (collides
with existing `s.db *sql.DB` — rename to `s.store`).

## Audit field consolidation (c2.10, c2.11)

New file `promptsheon/audit/keys.go`:

```go
package audit

const (
    KeyName        = "name"
    KeyStatus      = "status"
    KeyVersion     = "version"
    FieldAPIKey    = "api_key"  // was "key"
    FieldKeyPrefix = "key_prefix"
    FieldKeyName   = "key_name"
    FieldProvider  = "provider"
    FieldModel     = "model"
    FieldValue     = "value"
    FieldUserID    = "user_id"
    FieldEmail     = "email"
    FieldRole      = "role"
    FieldError     = "error"
    FieldOK        = "ok"
)

const AnonUser = "api"
```

## Exit criterion

```bash
go build ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run
grep -rn 'Error[A-Z]' promptsheon/errs/ --include='*.go'  # must return nothing
grep -rn 'Repositories\b' promptsheon/ --include='*.go' | grep -v _test  # must return nothing
grep -rn 'BypassSSRF\|activeOAuthStates\|NewAuthenticatorWithLogger' . --include='*.go'  # must return nothing
du -sh vendor/  # must be ≥ 30 MB smaller than before
```

## Parallelization

3 agents:

| Agent | Files |
|---|---|
| 2A | go.mod, go.sum, vendor/ |
| 2B | promptsheon/errs/, promptsheon/store/, promptsheon/server.go, promptsheon/middleware.go, promptsheon/audit/ (new) |
| 2C | promptsheon/auth/, promptsheon/alerting/, promptsheon/webhook/, promptsheon/handlers_*.go, promptsheon/usage.go, promptsheon/invoke/, promptsheon/llm/, cmd/promptsheond/ |