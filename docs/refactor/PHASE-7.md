# PHASE-7 — Documentation Refresh

**13 commits.** Rewrites all docs to match the post-refactor reality. Adds
ADR-0026 documenting the architectural choices.

## Commits

```
c7.0  docs(adr): ADR-0026 — public SDK module + Error→Err rename rationale
      Refs: PLAN-49/M-5 X-12
c7.1  docs(architecture): rewrite architecture.md for cmd/<name>/ layout
      Refs: PLAN-49/
c7.2  docs(architecture): modules.md (remove backend/bandit)
      Refs: PLAN-49/
c7.3  docs(development): development.md (correct build paths + layout)
      Refs: PLAN-49/
c7.4  docs(development): testing.md (correct test matrix + coverage floors)
      Refs: PLAN-49/
c7.5  docs(reference): sdk.md (rename canonical import path → pkg/promptsheon)
      Refs: PLAN-49/
c7.6  docs(reference): errors.md (Err* sentinels catalog)
      Refs: PLAN-49/L-1
c7.7  docs(security): security.md (supported versions corrected)
      Refs: PLAN-49/
c7.8  docs(security): remove old audit-2026-07-26.md or move to research/
      Refs: PLAN-49/
c7.9  docs(changelog): rewrite Unreleased as v0.4.0 with accurate layout
      Refs: PLAN-49/
c7.10 docs(research): audit-2026-07-26.md (with banner)
      Refs: PLAN-49/
c7.11 docs(readme): full README rewrite
      Refs: PLAN-49/
c7.12 docs(install): install.md (PyPI/npm quick start)
      Refs: PLAN-49/
```

## Files touched

| File | Commits |
|---|---|
| `docs/adr/0026-public-sdk-and-err-rename.md` (new) | c7.0 |
| `docs/architecture/architecture.md` (rewrite) | c7.1 |
| `docs/architecture/modules.md` (edit) | c7.2 |
| `docs/development/development.md` (rewrite) | c7.3 |
| `docs/development/testing.md` (rewrite) | c7.4 |
| `docs/reference/sdk.md` (rewrite) | c7.5 |
| `docs/reference/errors.md` (new) | c7.6 |
| `docs/security/security.md` (edit) | c7.7 |
| `docs/security/audit-2026-07-26.md` (move) | c7.8, c7.10 |
| `CHANGELOG.md` (rewrite Unreleased) | c7.9 |
| `README.md` (rewrite) | c7.11 |
| `docs/install.md` (new) | c7.12 |

## Key documents

### ADR-0026 (c7.0)

```markdown
# ADR-0026: Public SDK Module + Error Sentinel Rename

## Status
Accepted, 2026-XX-XX

## Context
The `promptsheon` repository has grown into a complex codebase with:
- 198 commits planned in PLAN-49
- 14 error sentinel types using `Error*` prefix (non-idiomatic)
- 7-layer internal structure (`backend/` + `cmd/` + `sdk/` + `pkg/`)
- Multiple audit-driven review passes exposing naming inconsistencies

## Decision

### 1. Public SDK surface: `pkg/promptsheon/*` with build tag

The public surface is `pkg/promptsheon/*.go` with `//go:build promptsheon`.
This module:
- Re-exports types from `sdk/` and `errs/` packages
- Is not built by default (`go build ./...` skips it)
- Requires `-tags=promptsheon` to compile (Makefile wrapper: `make build-public`)
- Is fenced by `scripts/check-pkg-fence.sh` against direct `backend/` imports

Alternative considered: separate module `pkg-sheon/`. Rejected because:
- Adds `replace` directive complexity
- Splits git history
- Doesn't fit single-module repo convention

### 2. Error sentinel rename: `Error*` → `Err*`

Direct, no deprecation alias period. All 14 sentinels renamed in one
commit (c2.8) and updated across ~30 call sites.

Why no alias period:
- Internal-only API (no external consumers verified in c0.0)
- Faster to clean up than to maintain aliases
- Aliases add permanent cognitive overhead

## Consequences

- Contributors using `go build ./...` get a no-op build for `pkg/`
- Contributors using `make build-public` get the full SDK
- CI runs both: default + tagged
- All `Error*` symbols are gone; `Err*` is the canonical form
- The banned-identifier linter (c5.0) prevents reintroduction

## References
- PLAN-49 audit
- Golangci-lint custom plugin in `tools/golangci-lint-promptsheon/`
- Build-tag fence in `scripts/check-pkg-fence.sh`
```

### README.md (c7.11)

Key sections:
- Quickstart (3 lines: install, init, run)
- Architecture overview (1 diagram)
- Public SDK import path (`github.com/sachncs/promptsheon/pkg/promptsheon`)
- PyPI (`promptsheon-sdk`) + npm (`@promptsheon/typescript`) packages
- Build instructions (`make build-server`, `make build-public`)
- Test instructions (`make check`)
- Contributing link
- Security policy link
- License (Apache 2.0)
- Roadmap (post-v1.0.1)

### install.md (c7.12)

```markdown
# Installation

## Server (daemon)

### From source
git clone https://github.com/sachncs/promptsheon
cd promptsheon
make build-server
./bin/promptsheond

### Docker
docker pull ghcr.io/sachncs/promptsheon/promptsheond:latest
docker run -p 8080:8080 ghcr.io/sachncs/promptsheon/promptsheond:latest

## SDK

### Go
go get github.com/sachncs/promptsheon/pkg/promptsheon
# (note: requires building server with -tags=promptsheon or using a pre-built release)

### Python
pip install promptsheon-sdk

### TypeScript / JavaScript
npm install @promptsheon/typescript
```

## Exit criterion

```bash
# All docs match the working tree:
ls docs/architecture/architecture.md && head -50 docs/architecture/architecture.md
ls docs/development/development.md && grep -c "./cmd/" docs/development/development.md  # ≥ 1
ls docs/reference/sdk.md && grep -c "promptsheon" docs/reference/sdk.md  # ≥ 5

# ADR-0026 is readable:
cat docs/adr/0026-public-sdk-and-err-rename.md

# No stale paths:
grep -rn "internal/" docs/ --include="*.md"  # must return nothing or only in audit-historical context
grep -rn "pprofAddr.*non-loopback" docs/ --include="*.md"  # none
```

## Parallelization

1 agent (docs need coherence).