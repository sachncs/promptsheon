# PHASE-6 — Public Pkg with Build-Tag Fence

**14 commits.** Establishes `pkg/promptsheon/*` as the public SDK surface with
a `//go:build promptsheon` build tag. Adds Makefile wrapper, IDE configs, CI
matrix, and grep guard.

## STOP gate after PR-6

Verify `pkg/promptsheon/` builds with `-tags=promptsheon` and the grep guard
catches no `promptsheon/` imports.

## Commits

```
c6.0  chore(build): Makefile flag GOFLAGS=-tags=promptsheon for pkg/ builds
      Refs: PLAN-49/C-7 X-13
c6.1  chore(build): make build-public target (compiles pkg/ with -tags)
      Refs: PLAN-49/X-13
c6.2  chore(build): make check-public target (vet + test pkg/ with -tags)
      Refs: PLAN-49/
c6.3  chore(ide): .vscode/settings.json with gopls buildFlags = ["-tags=promptsheon"]
      Refs: PLAN-49/X-14
c6.4  chore(ide): .idea/compiler.xml excludes pkg/ from default GoLand Go build
      Refs: PLAN-49/X-14
c6.5  refactor(pkg): pkg/promptsheon/client.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.6  refactor(pkg): pkg/promptsheon/types.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.7  refactor(pkg): pkg/promptsheon/errors.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.8  refactor(pkg): pkg/promptsheon/server.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.9  refactor(pkg): pkg/promptsheon/version.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.10 refactor(pkg): pkg/promptsheon/options.go //go:build promptsheon
      Refs: PLAN-49/C-7
c6.11 refactor(pkg): pkg/promptsheon/CHANGELOG.md
      Refs: PLAN-49/L-1
c6.12 refactor(sdk): sdk re-exports from pkg/promptsheon
      Refs: PLAN-49/M-4
c6.13 chore(ci): ci.yaml job: `go build -tags=promptsheon ./pkg/...`
      Refs: PLAN-49/C-7
c6.14 chore(ci): scripts/check-pkg-fence.sh greps pkg/ for promptsheon/ imports
      Refs: PLAN-49/C-7 M-10
```

## Files touched

| File | Commits |
|---|---|
| `Makefile` (add `build-public`, `check-public`) | c6.0-c6.2 |
| `.vscode/settings.json` (new) | c6.3 |
| `.idea/compiler.xml` (new) | c6.4 |
| `pkg/promptsheon/client.go` (new) | c6.5 |
| `pkg/promptsheon/types.go` (new) | c6.6 |
| `pkg/promptsheon/errors.go` (new) | c6.7 |
| `pkg/promptsheon/server.go` (new) | c6.8 |
| `pkg/promptsheon/version.go` (new) | c6.9 |
| `pkg/promptsheon/options.go` (new) | c6.10 |
| `pkg/promptsheon/CHANGELOG.md` (new) | c6.11 |
| `sdk/client.go` (re-export from pkg/) | c6.12 |
| `.github/workflows/ci.yaml` (add pkg-public job) | c6.13 |
| `scripts/check-pkg-fence.sh` (new) | c6.14 |

## Public surface shape

### pkg/promptsheon/client.go

```go
//go:build promptsheon

package promptsheon

import "github.com/sachncs/promptsheon/sdk"

type Client = sdk.Client

func NewClient(baseURL, apiKey string) *Client {
    return sdk.New(baseURL, apiKey)
}
```

### pkg/promptsheon/types.go

```go
//go:build promptsheon

package promptsheon

import "github.com/sachncs/promptsheon/sdk"

type (
    Workspace          = sdk.Workspace
    Project            = sdk.Project
    Capability         = sdk.Capability
    Version            = sdk.Version
    Release            = sdk.Release
    Execution          = sdk.Execution
    Dataset            = sdk.Dataset
    DatasetCase        = sdk.DatasetCase
    Precondition       = sdk.Precondition
    EvalRun            = sdk.EvalRun
    APIKey             = sdk.APIKey
    CreateWorkspaceRequest   = sdk.CreateWorkspaceRequest
    // ... all request types
)
```

### pkg/promptsheon/errors.go

```go
//go:build promptsheon

package promptsheon

import (
    "errors"

    "github.com/sachncs/promptsheon/promptsheon/errs"
)

var (
    ErrNotLeader       = errs.ErrNotLeader
    ErrProviderMissing = errs.ErrProviderMissing
    ErrQuorum          = errs.ErrQuorum
    ErrSelfVote        = errs.ErrSelfVote
    // ... all Err* sentinels
)

type APIError = sdk.APIError

func IsAPIError(err error) (*APIError, bool) {
    return sdk.IsAPIError(err)
}
```

### pkg/promptsheon/server.go

```go
//go:build promptsheon

package promptsheon

import (
    "github.com/sachncs/promptsheon/backend"
)

type Server = backend.Server
type Option = backend.Option

var (
    WithAuth             = backend.WithAuth
    WithProviders        = backend.WithProviders
    WithVault            = backend.WithVault
    WithOAuth            = backend.WithOAuth
    // ... all options
)

func New(opts ...Option) (*Server, error) {
    return backend.New(opts...)
}
```

## Makefile additions

```makefile
# Public SDK build
.PHONY: build-public
build-public:
	GOFLAGS=-tags=promptsheon go build -o $(BIN)/promptsheon-sdk ./pkg/promptsheon

.PHONY: check-public
check-public:
	GOFLAGS=-tags=promptsheon go vet ./pkg/promptsheon/...
	GOFLAGS=-tags=promptsheon go test -race -count=1 ./pkg/promptsheon/...

.PHONY: check
check: fmt vet lint test check-public
	@echo "all checks passed"
```

## CI matrix job (c6.13)

```yaml
jobs:
  pkg-public:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - name: Build public SDK
        env:
          GOFLAGS: -tags=promptsheon
        run: go build ./pkg/promptsheon/...
      - name: Vet public SDK
        env:
          GOFLAGS: -tags=promptsheon
        run: go vet ./pkg/promptsheon/...
      - name: Test public SDK
        env:
          GOFLAGS: -tags=promptsheon
        run: go test -race -count=1 ./pkg/promptsheon/...
      - name: Fence check
        run: bash scripts/check-pkg-fence.sh
```

## scripts/check-pkg-fence.sh (c6.14)

```bash
#!/usr/bin/env bash
# Fails if any pkg/promptsheon/*.go imports promptsheon/ directly.
set -euo pipefail
violations=$(grep -rn 'github.com/sachncs/promptsheon/backend' pkg/promptsheon/ --include='*.go' || true)
if [ -n "$violations" ]; then
    echo "FAIL: pkg/promptsheon/* must not import promptsheon/*"
    echo "$violations"
    exit 1
fi
echo "PASS: pkg/promptsheon/* is fenced"
```

## Exit criterion

```bash
make check
make build-public
make check-public
bash scripts/check-pkg-fence.sh
go build ./pkg/promptsheon/...  # without -tags, must FAIL
GOFLAGS=-tags=promptsheon go build ./pkg/promptsheon/...  # with -tags, must SUCCEED
```

## STOP gate

After PR-6 closes:
1. `make build-public` succeeds
2. `make check-public` succeeds
3. `bash scripts/check-pkg-fence.sh` returns PASS
4. Default `go build ./...` still works (pkg/ is no-op)
5. CI matrix job `pkg-public` is green

## Parallelization

2 agents:

| Agent | Files |
|---|---|
| 6A | Makefile, .vscode/settings.json, .idea/compiler.xml, .github/workflows/ci.yaml, scripts/check-pkg-fence.sh |
| 6B | pkg/promptsheon/*.go (new), sdk/client.go (re-export) |