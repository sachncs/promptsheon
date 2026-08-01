# PHASE-5B — Domain Renames

**4 commits.** Small renames in domain types. Runs after PR-5A.

## Commits

```
c5.16 rename: Release.CapabilityVersion → Release.Version
      Refs: PLAN-49/
c5.17 rename: MakerCheckerPolicy.Creator → Maker; .Required → Quorum
      Refs: PLAN-49/
c5.18 rename: SelfEvolveConfig.MaxRevisions → MaxAttempts; .TargetEnv → .Target
      Refs: PLAN-49/
c5.19 rename: EvalRunOptions → RunOptions
      Refs: PLAN-49/
```

## Files touched

| File | Commits |
|---|---|
| `promptsheon/release/release.go`, `promptsheon/release/service.go`, `sdk/client.go` | c5.16 |
| `promptsheon/approval/approval.go`, `promptsheon/handlers_releases.go`, `promptsheon/release/service.go` | c5.17 |
| `promptsheon/capability/capability.go`, `promptsheon/capability/evolve.go`, `promptsheon/evolve/evolver.go`, `promptsheon/evolve/types.go`, `sdk/client.go` | c5.18 |
| `promptsheon/harness/runner.go`, `promptsheon/harness/runner_test.go`, `promptsheon/handlers_harness.go` | c5.19 |

## Key renames

### c5.16: `Release.CapabilityVersion` → `Release.Version`

```go
// Before:
type Release struct {
    ID                string
    CapabilityVersion int  // ambiguous
    Status            Status
    // ...
}

// After:
type Release struct {
    ID       string
    Version  int  // unambiguous in package context
    Status   Status
    // ...
}
```

JSON tag: `"capability_version"` (kept for wire compatibility) maps to
`Version` via custom marshal/unmarshal.

### c5.17: `MakerCheckerPolicy.Creator` → `Maker`; `.Required` → `Quorum`

```go
// Before:
type MakerCheckerPolicy struct {
    Creator           string
    RequiredApprovers int
}

// After:
type MakerCheckerPolicy struct {
    Maker  string
    Quorum int
}
```

JSON tags: `"creator"`, `"required_approvers"` (kept for wire compat).

### c5.18: `SelfEvolveConfig.MaxRevisions` → `MaxAttempts`; `.TargetEnv` → `.Target`

```go
// Before:
type SelfEvolveConfig struct {
    MaxRevisions int
    TargetEnv    Environment
}

// After:
type SelfEvolveConfig struct {
    MaxAttempts int
    Target      Environment
}
```

JSON tags: `"max_revisions"`, `"target_env"` (kept for wire compat).

### c5.19: `EvalRunOptions` → `RunOptions`

```go
// Before:
type EvalRunOptions struct {
    DatasetID string
    // ...
}

// After:
type RunOptions struct {
    DatasetID string
    // ...
}
```

Located in `promptsheon/harness/runner.go`. Already in package `harness`, so
`harness.RunOptions` is unambiguous.

## Exit criterion

```bash
golangci-lint run  # no banned identifiers
go build ./...
go vet ./...
go test -race -count=1 ./...
```

## Parallelization

1 agent (single domain area).