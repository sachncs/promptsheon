# PHASE-1 — Build Pipeline + Frontend Embed

**19 commits.** Makes the build self-contained across all paths (Makefile,
Docker, goreleaser, CI).

## Commits

```
c1.0  chore(make): inline web-build, drop [ -d ] guard, build-server: web-build
      Refs: PLAN-49/OS-2
c1.1  chore(make): clean target also rm cmd/promptsheond/frontend
      Refs: PLAN-49/X-10
c1.2  chore(make): add dist-check target (find frontend/src -newer)
      Refs: PLAN-49/
c1.3  chore(docker): multi-stage with frontend-build stage (Node 22)
      Refs: PLAN-49/
c1.4  chore(goreleaser): add hooks.pre_build (node check + npm ci + build + cp)
      Refs: PLAN-49/
c1.5  chore(ci): actions/setup-node@v6 (Node 22) + npm cache
      Refs: PLAN-49/
c1.6  chore(ci): npm ci + build before goreleaser in build-release
      Refs: PLAN-49/
c1.7  chore(e2e): frontend build prerequisite in tests/e2e
      Refs: PLAN-49/
c1.8  chore(ci): bench-nightly + nightly-load build paths → ./cmd/<name>
      Refs: PLAN-49/OS-1
c1.9  chore(make): add sdk-check target (diff gen openapi.yaml)
      Refs: PLAN-49/L-4
c1.10 fix(sdk-ts): CI codegen writes _generated/openapi.yaml
      Refs: PLAN-49/OS-5
c1.11 fix(sdk-py): scripts/codegen.sh writes to sdk/python/src/promptsheon/
      Refs: PLAN-49/OS-6
c1.12 chore(gitignore): add sdk/python/src/promptsheon.egg-info/
      Refs: PLAN-49/
c1.13 chore(frontend): vite.config.js adds base: '/' config
      Refs: PLAN-49/X-9
c1.14 fix(frontend): titleFromRoute dead code
      Refs: PLAN-49/X-7
c1.15 fix(frontend): main.js:22-28 unreachable operationsMatch block
      Refs: PLAN-49/X-8
c1.16 chore(gitignore): add sheon/dist (placeholder for pkg-sheon not used)
      Refs: PLAN-49/
c1.17 chore(ci): -coverprofile + -race on default test step
      Refs: PLAN-49/
c1.18 test(e2e): verify //go:build e2e fences the in-memory provider (binary string check)
      Refs: PLAN-49/C-8
```

## Files touched

| File | Commits |
|---|---|
| `Makefile` | c1.0, c1.1, c1.2, c1.9 |
| `Dockerfile` | c1.3 |
| `.goreleaser.yml` | c1.4 |
| `.github/workflows/ci.yaml` | c1.5, c1.6, c1.8, c1.17 |
| `.github/workflows/bench-nightly.yaml` | c1.8 |
| `.github/workflows/nightly-load.yaml` | c1.8 |
| `tests/e2e/` | c1.7 |
| `sdk/typescript/scripts/codegen.sh` | c1.10 |
| `sdk/python/scripts/codegen.sh` | c1.11 |
| `.gitignore` | c1.12, c1.16 |
| `frontend/vite.config.js` | c1.13 |
| `frontend/src/views/index.js` | c1.14 |
| `frontend/src/main.js` | c1.15 |
| `cmd/promptsheond/e2e_provider.go` (new) | c1.18 |
| `cmd/promptsheond/e2e_provider_test.go` (new) | c1.18 |

## Key changes

### Multi-stage Docker (c1.3)

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.23-alpine3.20 AS build-base
RUN apk add --no-cache node=~22 npm=~10

FROM build-base AS frontend-build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build
RUN mkdir -p ../cmd/promptsheond/frontend && cp -r dist ../cmd/promptsheond/frontend/dist

FROM build-base AS go-build
WORKDIR /src
COPY . .
COPY --from=frontend-build /src/cmd/promptsheond/frontend/dist /src/cmd/promptsheond/frontend/dist
RUN go build -o /out/promptsheond ./cmd/promptsheond
```

### Goreleaser pre_build hook (c1.4)

```yaml
hooks:
  pre_build:
    - cmd: sh
      args:
        - -c
        - |
          command -v node >/dev/null 2>&1 || { echo "node missing"; exit 1; }
          cd frontend
          [ -d node_modules ] || npm ci --no-audit --no-fund
          npm run build
          rm -rf ../cmd/promptsheond/frontend/dist
          mkdir -p ../cmd/promptsheond/frontend/dist
          cp -r dist/. ../cmd/promptsheond/frontend/dist/
```

### Build-tag-segregated e2e provider (c1.18)

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

// cmd/promptsheond/e2e_provider.go
package main

func registerE2EProvider() {}
```

## Exit criterion

```bash
# All three paths produce a binary with a working embed:
make build-server
docker build .
goreleaser build --snapshot --clean

# Verification:
git archive HEAD | tar -x  # extract to clean dir
cd <extracted> && make build-server  # must succeed

# Tests:
go test ./...
go test -tags e2e ./cmd/promptsheond/...  # e2e tag produces test binary
```

## Parallelization

3 agents:

| Agent | Files |
|---|---|
| 1A | Makefile, scripts/check-* |
| 1B | Dockerfile, .goreleaser.yml, cmd/promptsheond/ |
| 1C | .github/workflows, tests/e2e, frontend, sdk/ |