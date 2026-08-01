# PHASE-5C — Cross-Cutting Renames

**3 commits.** Touches multiple files. Includes a signature change to
`release.Resolver.Invoke`. Runs after PR-5B.

## Pre-PR-5C manual checklist

```bash
go list -m -mod=mod all 2>/dev/null | grep -v promptsheon | grep -v _test
```

Verify no external consumers of `*release.Resolver`. (Already verified in c0.0.)

## Commits

```
c5.20 refactor: release.Resolver.Invoke signature change + fold apiReleaseInvoker
      Refs: PLAN-49/P0-7 C-9
c5.21 rename: handleInvokeOne + invokeOneWithManifest → Server.RecordExecution
      Refs: PLAN-49/P1-9
c5.22 chore(lint): final golangci-lint pass; verify zero banned identifiers
      Refs: PLAN-49/H-3
```

## Files touched

| File | Commits |
|---|---|
| `promptsheon/release/resolver.go` (add Invoke method) | c5.20 |
| `cmd/promptsheond/daemon_release_invoker.go` (delete) | c5.20 |
| `cmd/promptsheond/daemon.go` (use *release.Resolver directly) | c5.20 |
| `promptsheon/harness/runner.go` (ReleaseInvoker interface grows one method) | c5.20 |
| `promptsheon/handlers_releases.go` (consolidate invokeOne) | c5.21 |
| `promptsheon/handlers_executions.go` (consolidate invokeOne) | c5.21 |
| (all files) | c5.22 |

## Key changes

### c5.20: `*release.Resolver.Invoke` signature

```go
// Before:
// *release.Resolver has Load (artifact loader) but no Invoke
// harness.ReleaseInvoker is an interface satisfied by apiReleaseInvoker
type apiReleaseInvoker struct {
    inv      *invoke.Invoker
    svc      *release.Service
    resolver *release.Resolver
}

func (a *apiReleaseInvoker) Invoke(ctx context.Context, releaseID string, inputs map[string]any) (*capability.Execution, error) {
    // 40 lines of glue
}

// After:
func (r *Resolver) Invoke(ctx context.Context, releaseID string, inputs map[string]any) (*capability.Execution, error) {
    // 30 lines of inlined logic
}
```

`harness.ReleaseInvoker` interface unchanged (still has `Invoke` method);
`apiReleaseInvoker` deleted; `daemon.go` passes `*release.Resolver` directly.

### c5.21: Consolidate `invokeOne` and `invokeOneWithManifest`

```go
// Before: two near-duplicate helpers
func (s *Server) invokeOne(ctx, capabilityVersionID, inputs, model, provider) (*capability.Execution, error) {
    // 30 lines
}

func (s *Server) invokeOneWithManifest(ctx, release, inputs) (*capability.Execution, error) {
    // 35 lines, mirrors invokeOne
}

// After: single method
func (s *Server) RecordExecution(ctx context.Context, req RecordExecutionRequest) (*capability.Execution, error) {
    // 25 lines, shared by both paths
}

type RecordExecutionRequest struct {
    CapabilityVersionID string
    ReleaseID           string  // optional; if set, uses release path
    Inputs              map[string]any
    Model               string
    Provider            string
}
```

Both handlers call `s.RecordExecution` with the appropriate request.

## Exit criterion

```bash
golangci-lint run
go build ./...
go vet ./...
go test -race -count=1 ./...
test -f cmd/promptsheond/daemon_release_invoker.go && echo "FAIL: should be deleted" || echo "OK"
```

## Parallelization

1 agent.