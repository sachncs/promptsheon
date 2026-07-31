# PHASE-4A — Test Infrastructure

**10 commits.** Decomposes the 953-line `mockRepo` god-mock into per-resource
mocks, adds test-injection seams (clock, janitorSweep, e2e provider), and
raises coverage floors.

## STOP gate after PR-4A

Verify test seams are sufficient before opening PR-4B. If new tests still
need them, add a `PR-4A.1` follow-up (0-5 commits) before continuing.

## Commits

```
c4.0  refactor(test): decompose mockRepo → backend/testutil/mockstore/{users,apikeys,audit,providerkeys,alerting,webhooks,workspaces,projects,capabilities,releases,executions,harness}.go
      Refs: PLAN-49/C-5
c4.1  test(mockstore): unit tests for each per-resource mock
      Refs: PLAN-49/L-2
c4.2  refactor(test): delete backend/handlers_test_support_test.go
      Refs: PLAN-49/
c4.3  test(inject): //go:build e2e seam in cmd/promptsheond (registerE2EProvider file pair)
      Refs: PLAN-49/C-8
c4.4  test(e2e): wire inMemoryProvider into TestCapabilityLifecycle; remove skip branch
      Refs: PLAN-49/F13 F16
c4.5  test(inject): harness.ReleaseInvoker adds janitorSweep + ws injectable test seam
      Refs: PLAN-49/F31
c4.6  test(inject): evalRunner adds Clock injection seam (for F35)
      Refs: PLAN-49/F35
c4.7  test(inject): audit chain adds InjectableClock (for retry-bound test)
      Refs: PLAN-49/
c4.8  test(inject): meta-test asserting test seams are sufficient
      Refs: PLAN-49/
c4.9  test(coverage): raise handlers_*.go floor to 75%, store floor to 65%
      Refs: PLAN-49/
c4.10 test(short): -short opt-out for slow concurrency tests
      Refs: PLAN-49/F40
```

## Files touched

| File | Commits |
|---|---|
| `backend/handlers_test_support_test.go` (delete after c4.2) | c4.0, c4.2 |
| `backend/testutil/mockstore/users.go` (new) | c4.0 |
| `backend/testutil/mockstore/apikeys.go` (new) | c4.0 |
| `backend/testutil/mockstore/audit.go` (new) | c4.0 |
| `backend/testutil/mockstore/providerkeys.go` (new) | c4.0 |
| `backend/testutil/mockstore/alerting.go` (new) | c4.0 |
| `backend/testutil/mockstore/webhooks.go` (new) | c4.0 |
| `backend/testutil/mockstore/workspaces.go` (new) | c4.0 |
| `backend/testutil/mockstore/projects.go` (new) | c4.0 |
| `backend/testutil/mockstore/capabilities.go` (new) | c4.0 |
| `backend/testutil/mockstore/releases.go` (new) | c4.0 |
| `backend/testutil/mockstore/executions.go` (new) | c4.0 |
| `backend/testutil/mockstore/harness.go` (new) | c4.0 |
| `backend/testutil/mockstore/db.go` (new — wires all mocks into a `DB`) | c4.0 |
| `backend/testutil/mockstore/users_test.go` (new) | c4.1 |
| ... one test per mock file | c4.1 |
| `cmd/promptsheond/e2e_provider.go` (edit from c1.18) | c4.3 |
| `cmd/promptsheond/e2e_provider_stub.go` (new) | c4.3 |
| `tests/e2e/daemon_e2e_test.go` | c4.4 |
| `backend/harness/runner.go` (Clock field) | c4.6 |
| `backend/audit_workers.go` (InjectableClock field) | c4.7 |
| `backend/testutil/seams_test.go` (new — meta-test) | c4.8 |
| `scripts/check-coverage.sh` (raise floors) | c4.9 |
| `.github/workflows/ci.yaml` (`-short` flag) | c4.10 |

## Key shapes

### Mockstore skeleton (c4.0)

```go
// backend/testutil/mockstore/users.go
package mockstore

import "context"

type Users struct {
    Store map[string]*models.User
}

func NewUsers() *Users { return &Users{Store: map[string]*models.User{}} }

func (u *Users) CreateUser(ctx context.Context, user *models.User) error {
    u.Store[user.ID] = user
    return nil
}

// ... other methods ...
```

```go
// backend/testutil/mockstore/db.go
package mockstore

type DB struct {
    *Users
    *APIKeys
    *Audit
    // ... all 14 mocks ...
}

func NewDB() *DB {
    return &DB{
        Users:       NewUsers(),
        APIKeys:     NewAPIKeys(),
        // ...
    }
}
```

### E2E provider seam (c4.3)

Already defined in c1.18. c4.3 verifies both files compile together.

```go
//go:build e2e

// cmd/promptsheond/e2e_provider.go
package main

import "github.com/sachncs/promptsheon/invoke"

func registerE2EProvider() {
    invoke.RegisterProvider("e2e-inmemory", invoke.NewInMemoryProvider())
}
```

```go
//go:build !e2e

// cmd/promptsheond/e2e_provider.go (alternative file)
package main

func registerE2EProvider() {}
```

### Clock injection (c4.6, c4.7)

```go
// backend/audit_workers.go
type Server struct {
    // ...
    Clock func() time.Time  // injected; defaults to time.Now
}

func (s *Server) appendAudit(ctx context.Context, entry *models.AuditEntry) error {
    now := time.Now
    if s.Clock != nil {
        now = s.Clock
    }
    entry.Timestamp = now().UTC()
    // ...
}
```

```go
// backend/harness/runner.go
type EvalRunner struct {
    // ...
    Clock func() time.Time
}
```

### Meta-test (c4.8)

```go
// backend/testutil/seams_test.go
package testutil_test

import (
    "go/ast"
    "go/parser"
    "go/token"
    "strings"
    "testing"
)

func TestSeamsAreSufficient(t *testing.T) {
    // Read all *_test.go files in backend/handlers_*.go
    // For each test function, ensure it uses at least one of:
    //   - s.Clock injection
    //   - s.InjectableClock injection
    //   - inMemoryProvider
    //   - janitorSweep direct call
    //   - newTestServerWithSeam helper
    // If a test uses none, fail with "needs a seam"
}
```

## Exit criterion

```bash
go build ./...
go vet ./...
go test -race -count=1 ./...
go test -short ./...  # opt-out for slow tests
golangci-lint run
bash scripts/check-coverage.sh coverage.out  # floors: handlers 75%, store 65%
test -f backend/handlers_test_support_test.go && echo "FAIL: should be deleted" || echo "OK"
```

## STOP gate review

After PR-4A closes, the sequencer reviews:

1. Are all `backend/handlers_*_test.go` tests using at least one seam?
2. Does the meta-test (c4.8) pass?
3. Are there test failures in `tests/e2e/`?

If any answer is "no" or "fails", open PR-4A.1 with additional seams.

## Parallelization

2 agents:

| Agent | Files |
|---|---|
| 4A1 | backend/testutil/mockstore/*.go (new), backend/handlers_test_support_test.go (delete) |
| 4A2 | cmd/promptsheond/e2e_provider*.go, backend/harness/, backend/audit_workers.go, tests/e2e/ |