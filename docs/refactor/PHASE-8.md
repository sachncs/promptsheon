# PHASE-8 — v1.0.0 Cut + Final Verification

**6 commits.** Bumps VERSION, cuts the v1.0.0 tag, and adds the
plan-coverage tooling that prevents the 49 critique items from being lost.

## Pre-PR-8 manual checklist

```
[ ] All Phase 0-7 PRs merged to master
[ ] CI green on master
[ ] Tests pass with -tags=promptsheon
[ ] SDK publish workflows tested with staging credentials (optional)
```

## Commits

```
c8.0  chore(release): update goreleaser prerelease = false
      Refs: PLAN-49/
c8.1  chore(release): bump VERSION 0.4.0 → 1.0.0
      Refs: PLAN-49/H-9
c8.2  chore(release): git-cliff generates CHANGELOG from 0.4.0..1.0.0
      Refs: PLAN-49/
c8.3  chore(release): cut annotated v1.0.0 git tag
      Refs: PLAN-49/M-8
c8.4  chore(release): publish sheon to PyPI + npm via CI (manual approval)
      Refs: PLAN-49/
c8.5  chore(verify): scripts/plan-coverage.yaml + scripts/check-plan-coverage.sh
      Refs: PLAN-49/
```

## Files touched

| File | Commits |
|---|---|
| `.goreleaser.yml` (prerelease: false) | c8.0 |
| `VERSION` (1.0.0) | c8.1 |
| `CHANGELOG.md` (regenerate) | c8.2 |
| `git tag v1.0.0` (annotated) | c8.3 |
| `.github/workflows/publish-python.yml`, `publish-npm.yml` (manual trigger on tag) | c8.4 |
| `docs/refactor/scripts/plan-coverage.yaml` (new) | c8.5 |
| `docs/refactor/scripts/check-plan-coverage.sh` (new) | c8.5 |
| `.github/workflows/ci.yaml` (add plan-coverage job) | c8.5 |

## scripts/plan-coverage.yaml (49 items)

```yaml
# docs/refactor/scripts/plan-coverage.yaml
# Maps 49 critique items to the commits that resolve them.
# Verified by scripts/check-plan-coverage.sh.

items:
  - id: C-1
    description: "Missing commits — DEF items with no slot"
    addressed_by:
      - c0.0
      - c0.3
      - c0.10
      - c0.11
      - c0.12
      - c0.13
      - c0.17
      - c0.19
      - c0.20
      - c0.21
  - id: C-2
    description: "Phase 5.c41 → c5.0 (golangci-lint first)"
    addressed_by:
      - c5.0
  - id: C-3
    description: "Phase 2 + Phase 5 duplicate work"
    addressed_by:
      - c2.10
      - c2.11
      - c5.1
  - id: C-4
    description: "OAuth fixes need co-location"
    addressed_by:
      - c0.3
      - c0.16
  - id: C-5
    description: "mockstore decomposition is one commit"
    addressed_by:
      - c4.0
  - id: C-6
    description: "Phase 5 commit count unrealistic"
    addressed_by:
      - c5.16
      - c5.17
      - c5.18
      - c5.19
      - c5.20
      - c5.21
      - c5.22
  - id: C-7
    description: "Build-tag infra missing"
    addressed_by:
      - c6.0
      - c6.1
      - c6.2
      - c6.3
      - c6.4
      - c6.5
      - c6.6
      - c6.7
      - c6.8
      - c6.9
      - c6.10
      - c6.13
      - c6.14
  - id: C-8
    description: "inMemoryProvider needs //go:build e2e fence"
    addressed_by:
      - c1.18
      - c4.3
  - id: C-9
    description: "Verify no external consumers of release.Resolver"
    addressed_by:
      - c0.0
      - c5.20
  - id: C-10
    description: "Phase 3 depends on repo-settings"
    addressed_by:
      - c3.8
      - c3.9
  - id: H-1
    description: "Time estimates 3-5x optimistic"
    addressed_by:
      - c1.0
      - c2.0
      - c4.0
      - c5.0
  - id: H-2
    description: "No go vet discipline per commit"
    addressed_by:
      - c0.1
      - c0.2
      - c0.3
      - c0.4
      - c0.5
      - c0.6
      - c0.7
      - c0.8
      - c0.9
      - c0.10
      - c0.11
      - c0.12
      - c0.13
      - c0.14
      - c0.15
      - c0.16
      - c0.17
      - c0.18
      - c0.19
      - c0.20
      - c0.21
      - c0.22
      - c0.23
      - c0.24
      - c0.25
      - c0.26
      - c0.27
      - c0.28
      - c0.29
      - c0.30
      - c0.31
      - c0.32
      - c0.33
      - c0.34
      - c0.t1
      - c0.t2
      - c0.t3
      - c0.t4
      - c0.t5
      - c0.t6
      - c0.t7
      - c0.t8
      - c0.t9
      - c0.t10
      - c0.t11
      - c0.t12
      - c0.t13
      - c0.t14
      - c0.t15
      - c0.t16
      - c0.t17
      - c0.t18
      - c0.t19
  - id: H-3
    description: "Per-commit CI not specified"
    addressed_by:
      - c0.1
      - c5.22
  - id: H-4
    description: "Phase 5 collides with DB rename"
    addressed_by:
      - c2.9
      - c5.16
      - c5.17
      - c5.18
      - c5.19
  - id: H-5
    description: "Phase 4 missing infrastructure commits"
    addressed_by:
      - c4.0
      - c4.1
      - c4.3
      - c4.5
      - c4.6
      - c4.7
      - c4.8
  - id: H-6
    description: "Phase 5.c9 misdescribes existing file"
    addressed_by:
      - c2.17
  - id: H-7
    description: "Phase 1 doesn't cover vite.config.js"
    addressed_by:
      - c1.13
  - id: H-8
    description: "golangci-lint rule needs authoring"
    addressed_by:
      - c5.0
  - id: H-9
    description: "No phase for v1.0.0 tag"
    addressed_by:
      - c8.1
      - c8.3
  - id: H-10
    description: "Phase 4 may need re-baselining after c0.12"
    addressed_by:
      - c0.10
      - c4.0
      - c4.1
  - id: M-1
    description: "Phase 2.c10 too coarse"
    addressed_by:
      - c2.8
  - id: M-2
    description: "Phase 0 doesn't regen vendor"
    addressed_by:
      - c2.6
      - c2.7
  - id: M-3
    description: "Phase 4 conflicts with Phase 5 renames"
    addressed_by:
      - c4.0
      - c5.0
      - c5.16
      - c5.17
      - c5.18
      - c5.19
  - id: M-4
    description: "Phase 6 cross-imports"
    addressed_by:
      - c6.5
      - c6.6
      - c6.7
      - c6.8
      - c6.9
      - c6.10
      - c6.12
  - id: M-5
    description: "No ADR"
    addressed_by:
      - c7.0
  - id: M-6
    description: "Phase 3 CODEOWNERS out of sync"
    addressed_by:
      - c3.12
      - c6.5
  - id: M-7
    description: "No benchmarks"
    addressed_by:
      - "(deferred to v1.0.1)"
  - id: M-8
    description: "No annotated tag hygiene"
    addressed_by:
      - c3.1
      - c8.3
  - id: M-9
    description: "All files in pkg/ need consistent tag"
    addressed_by:
      - c6.5
      - c6.6
      - c6.7
      - c6.8
      - c6.9
      - c6.10
  - id: M-10
    description: "Don't double-build-tag"
    addressed_by:
      - c6.14
  - id: L-1
    description: "SDK deprecation timeline"
    addressed_by:
      - c6.11
  - id: L-2
    description: "Test infra not enumerated"
    addressed_by:
      - c4.0
      - c4.1
  - id: L-3
    description: "vendor/ consistency"
    addressed_by:
      - c2.7
  - id: L-4
    description: "sdk-check undefined"
    addressed_by:
      - c1.9
  - id: L-5
    description: "Phase 5.c30 + c5.40 collide"
    addressed_by:
      - c5.15
  - id: L-6
    description: "r.headers cleanup"
    addressed_by:
      - c0.2
  - id: L-7
    description: "golangci-lint enablement in CI"
    addressed_by:
      - c0.1
  - id: L-8
    description: "gitleaks config"
    addressed_by:
      - c3.15
  - id: L-9
    description: "inMemoryProvider scope"
    addressed_by:
      - c1.18
      - c4.3
  - id: L-10
    description: "Plan has 9 phases (not 8)"
    addressed_by:
      - "(meta — see README.md)"
  - id: X-1
    description: "CODE_OF_CONDUCT.md v2.0 → v2.1"
    addressed_by:
      - c0.29
  - id: X-2
    description: "Helm replicaCount policy"
    addressed_by:
      - c0.32
  - id: X-3
    description: "Frontend pill() dedup"
    addressed_by:
      - "(deferred to v1.0.1)"
  - id: X-4
    description: "ssl import in client.py"
    addressed_by:
      - c0.33
  - id: X-5
    description: "Frontend script hardcoded paths"
    addressed_by:
      - "(deferred to v1.0.1)"
  - id: X-6
    description: "Dead BenchmarkSelect baseline"
    addressed_by:
      - c0.23
  - id: X-7
    description: "titleFromRoute dead code"
    addressed_by:
      - c1.14
  - id: X-8
    description: "operationsMatch unreachable"
    addressed_by:
      - c1.15
  - id: X-9
    description: "vite.config.js base setting"
    addressed_by:
      - c1.13
  - id: X-10
    description: "Makefile clean incomplete"
    addressed_by:
      - c1.1
  - id: X-11
    description: "Legacy /internal/ paths in comments"
    addressed_by:
      - c3.12
      - c7.1
  - id: X-12
    description: "No ADR-0026"
    addressed_by:
      - c7.0
  - id: X-13
    description: "No make build-public"
    addressed_by:
      - c6.0
      - c6.1
  - id: X-14
    description: "No gopls config"
    addressed_by:
      - c6.3
      - c6.4
  - id: X-15
    description: "No .gitleaks.toml"
    addressed_by:
      - c3.15
```

## scripts/check-plan-coverage.sh

```bash
#!/usr/bin/env bash
# docs/refactor/scripts/check-plan-coverage.sh
# Reads plan-coverage.yaml; greps git log for commit messages referencing each item.
# Exits non-zero if any item has zero references in the commit log.

set -euo pipefail

if [ ! -f docs/refactor/scripts/plan-coverage.yaml ]; then
    echo "ERR: docs/refactor/scripts/plan-coverage.yaml missing"
    exit 1
fi

if ! command -v yq >/dev/null 2>&1; then
    echo "ERR: yq not installed (https://github.com/mikefarah/yq)"
    exit 1
fi

UNRESOLVED=0
while IFS= read -r line; do
    ID=$(echo "$line" | yq -r '.id')
    COUNT=$(git log --oneline 2>/dev/null | grep -c "Refs:.*$ID" || true)
    if [ "$COUNT" -eq 0 ]; then
        echo "UNRESOLVED: $ID"
        UNRESOLVED=$((UNRESOLVED + 1))
    else
        echo "OK: $ID ($COUNT refs)"
    fi
done < <(yq -c '.items[]' docs/refactor/scripts/plan-coverage.yaml)

if [ "$UNRESOLVED" -gt 0 ]; then
    echo ""
    echo "FAIL: $UNRESOLVED items unresolved"
    exit 1
fi
echo ""
echo "PASS: all items resolved"
```

## Final verification

```bash
go build ./...
cd sheon 2>/dev/null || cd ..  # only if separate module; not used here
go build ./...
go vet ./...
go test -race -count=1 ./...
make check
make build-public
make check-public
golangci-lint run
bash docs/refactor/scripts/check-plan-coverage.sh
gitleaks detect --no-git
gh workflow run scorecard.yml
go run ./tools/golangci-lint-promptsheon --config .golangci.yml ./...
```

## Tag cut

```bash
git tag -a v1.0.0 -m "Release v1.0.0

Critical defect patches, build pipeline fix, public SDK fence, OSS hygiene,
comprehensive test hardening, and dead-code removal.

See docs/refactor/README.md for the full PLAN-49."

git push origin v1.0.0
```

## SDK publish

The `publish-python.yml` and `publish-npm.yml` workflows fire on the v1.0.0
tag push. They have manual approval gates via `environment: production-pypi`
and `production-npm`. After approval:
- PyPI: `promptsheon-sdk` v1.0.0 published
- npm: `@promptsheon/typescript` v1.0.0 published

## Exit criterion

```bash
# All previous PR exits hold
# Plus:
git tag -l 'v1.0.0'  # must show v1.0.0
git describe --tags   # must show v1.0.0
bash docs/refactor/scripts/check-plan-coverage.sh  # PASS
```

## Parallelization

1 agent (release operations are sequential).