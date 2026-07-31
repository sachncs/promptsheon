# PLAN-49 — Top-Level Plan

## Goal

Establish a **rock-solid foundation** for the `promptsheon` open-source release
by fixing all confirmed defects, hardening the test suite, verifying data
processing, removing dead code, and consolidating the public surface.

## Why 198 commits

Each commit is small enough to revert independently and passes CI in isolation.
Defects, renames, and test additions are sequenced to avoid mid-flight
incompatibility.

## Scope (198 commits across 14 PRs)

| PR | Phase | Commits | Critical |
|---|---|---|---|
| PR-0 | Defects | 54 | yes |
| PR-1 | Build pipeline + frontend | 19 | yes |
| PR-2 | Public surface + dead deps + Err rename | 25 | yes |
| PR-3 | OSS hygiene (parallel) | 16 | yes |
| PR-4A | Test infrastructure | 10 | yes |
| PR-4B | Handler tests A | 5 | yes |
| PR-4C | Handler tests B | 5 | yes |
| PR-4D | Handler tests C | 9 | yes |
| PR-5A | Foundational renames + dead code | 15 | yes |
| PR-5B | Domain renames | 4 | yes |
| PR-5C | Cross-cutting renames | 3 | yes |
| PR-6 | Public pkg with build-tag fence | 14 | yes |
| PR-7 | Documentation refresh | 13 | yes |
| PR-8 | v1.0.0 cut | 6 | yes |

## Addressed audit items (49)

See `scripts/plan-coverage.yaml` for the full manifest. Each commit message
includes `Refs: PLAN-49/<id>` where `<id>` is one of:

| ID | Description | Phase |
|---|---|---|
| C-1 | Missing commits — DEF items with no slot | 0 |
| C-2 | Phase 5.c41 → c5.0 (golangci-lint first) | 5A |
| C-3 | Phase 2 + Phase 5 duplicate work | 2 |
| C-4 | OAuth fixes need co-location | 0 |
| C-5 | mockstore decomposition is one commit | 4A |
| C-6 | Phase 5 commit count unrealistic | 5A/5B/5C |
| C-7 | Build-tag infra missing | 6 |
| C-8 | `inMemoryProvider` needs `//go:build e2e` fence | 1 + 4A |
| C-9 | Verify no external consumers of `release.Resolver` | 0 |
| C-10 | Phase 3 depends on repo-settings | 3 |
| H-1 | Time estimates 3-5x optimistic | all |
| H-2 | No `go vet` discipline per commit | all |
| H-3 | Per-commit CI not specified | all |
| H-4 | Phase 5 collides with `DB` rename | 2 + 5 |
| H-5 | Phase 4 missing infrastructure commits | 4A |
| H-6 | Phase 5.c9 misdescribes existing file | 2 |
| H-7 | Phase 1 doesn't cover `vite.config.js` | 1 |
| H-8 | golangci-lint rule needs authoring | 5A |
| H-9 | No phase for v1.0.0 tag | 8 |
| H-10 | Phase 4 may need re-baselining after c0.12 | 4 |
| M-1 | Phase 2.c10 too coarse | 2 |
| M-2 | Phase 0 doesn't regen vendor | 2 |
| M-3 | Phase 4 conflicts with Phase 5 renames | 4 + 5 |
| M-4 | Phase 6 cross-imports | 6 |
| M-5 | No ADR | 7 |
| M-6 | Phase 3 CODEOWNERS out of sync | 6 |
| M-7 | No benchmarks | deferred to v1.0.1 |
| M-8 | No annotated tag hygiene | 3 + 8 |
| M-9 | All files in pkg/ need consistent tag | 6 |
| M-10 | Don't double-build-tag | 6 |
| L-1 | SDK deprecation timeline | 7 |
| L-2 | Test infra not enumerated | 4A |
| L-3 | vendor/ consistency | 2 |
| L-4 | sdk-check undefined | 1 |
| L-5 | Phase 5.c30 + c5.40 collide | 5A |
| L-6 | `r.headers` cleanup | 0 |
| L-7 | golangci-lint enablement in CI | 0 |
| L-8 | gitleaks config | 3 |
| L-9 | inMemoryProvider scope | 1 |
| L-10 | Plan has 9 phases (not 8) | all |
| X-1 | CODE_OF_CONDUCT.md v2.0 → v2.1 | 0 |
| X-2 | Helm replicaCount policy | 0 |
| X-3 | Frontend `pill()` dedup | deferred |
| X-4 | `ssl` import in `client.py` | 0 |
| X-5 | Frontend script hardcoded paths | deferred |
| X-6 | Dead `BenchmarkSelect` baseline | 0 |
| X-7 | `titleFromRoute` dead code | 1 |
| X-8 | `operationsMatch` unreachable | 1 |
| X-9 | `vite.config.js` base setting | 1 |
| X-10 | `Makefile clean` incomplete | 1 |
| X-11 | Legacy `/internal/` paths in comments | various |
| X-12 | No ADR-0026 | 7 |
| X-13 | No `make build-public` | 6 |
| X-14 | No `gopls` config | 6 |
| X-15 | No `.gitleaks.toml` | 3 |

## Commit message template

```
<type>(<scope>): <subject>

<body explaining the change>

Refs: PLAN-49/<item-id-1> PLAN-49/<item-id-2>
```

## Verification per commit

```
go build ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run
```

## Verification per PR

See each phase file for PR-specific exit criteria.

## Risk mitigations baked in

| Risk | Mitigation |
|---|---|
| 1 — Build-tag opt-in | `//go:build promptsheon` + Makefile wrapper + go.work + IDE configs + CI matrix + grep guard (`scripts/check-pkg-fence.sh`) |
| 2 — Time estimate | Phase 5 split into 3 PRs; Phase 4 split into 4 PRs; STOP gates; defer 12 non-essential commits to v1.0.1 |
| 3 — 203 commits | 14 PRs (each reviewable in 1-2 hours) |
| 4 — Node on CI | Multi-stage Docker + goreleaser pre_build + Node 22 pin in CI |
| 5 — `//go:build e2e` | Build-tag-segregated file + goreleaser default + c1.18 test |
| 6 — Test infra growth | 3 infra commits + meta-test (c4.8) + STOP gate after PR-4A |
| 7 — Manual critique check | YAML manifest + `check-plan-coverage.sh` + `Refs: PLAN-49/<id>` + CI gate |

## Sequencer rules

When parallel agents work disjoint file slices:
1. Each agent commits to its own branch (`pr/<n>/agent-<x>`)
2. Sequencer merges in dependency order
3. Sequencer runs full verification at PR boundary
4. Sequencer runs `bash scripts/check-plan-coverage.sh` to confirm no item is unresolved

## Conflict resolution

If two agents touch the same file:
1. Agent with earlier `Refs: PLAN-49/<id>` priority wins
2. Other agent rebases and re-verifies
3. If same symbol, higher-priority agent takes it; lower-priority agent skips

Priority map:
1. DEF fixes (Phase 0)
2. Renames (Phase 5)
3. Dead-code drops (Phase 2/5)
4. Test additions (Phase 4)
5. Docs (Phase 7)

## STOP gates

| Gate | After | Purpose |
|---|---|---|
| 1 | PR-4A | Verify test seams sufficient |
| 2 | PR-5A | Verify golangci-lint rule catches zero new offenders |
| 3 | PR-6 | Verify `pkg/promptsheon/` builds with `-tags=promptsheon` |

## Deferred to v1.0.1

These items were intentionally deferred to keep v1.0.0 focused:

- Frontend `pill()` deduplication (X-3) — frontend polish release
- Frontend `smoke.mjs` / `browser-e2e.mjs` hardcoded paths (X-5) — tooling release
- `Benchmark*` for streaming + audit CAS paths (M-7) — perf release
- `legacy-notes.md`, `quickstart.md`, `migration.md` refinements (L-1) — post-first-external-feedback

## Reference

- `PHASE-0.md` through `PHASE-8.md` — per-phase commit lists
- `scripts/plan-coverage.yaml` — 49-item manifest
- `scripts/check-plan-coverage.sh` — CI gate