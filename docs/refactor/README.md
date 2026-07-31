# PLAN-49 — Rock-Solid Foundation Refactor

> 198 atomic commits across 14 PRs to fix defects, harden tests, verify data
> processing, drop dead code, and establish a single canonical public surface
> for `promptsheon`.

## Decisions (locked)

| Question | Decision |
|---|---|
| Module / binary / package name | `promptsheon` (unchanged) |
| Error sentinel rename `Error*` → `Err*` | Direct, no alias |
| PR granularity | Atomic commits per phase; one PR per phase |
| God-struct `Repositories` | Rename to `DB`, no alias |
| Test depth | Full matrix (per-handler persisted + audit asserts) |
| Dead dependencies | Drop all (12 direct + indirect, ~30 MB smaller `vendor/`) |
| Public surface | `pkg/promptsheon/*` with `//go:build promptsheon` fence + Makefile wrapper + grep guard |
| Phase 3 (OSS hygiene) | Runs **in parallel** with Phases 2/4/5 |
| Execution model | Solo with PR-3 parallel |

## Time estimate

| Stage | Days |
|---|---|
| PR-0 (defects) | 2-3 |
| PR-1 (build pipeline) | 1-2 |
| PR-2 (public surface + dead deps) | 3-4 |
| PR-3 (OSS hygiene) — **parallel** | 1-2 |
| PR-4A (test infra) | 4-6 |
| PR-4B/C/D (handler tests) | 5-7 |
| PR-5A/B/C (renames + dead code) | 7-10 |
| PR-6 (public pkg) | 2-3 |
| PR-7 (docs) | 1-2 |
| PR-8 (v1.0.0 cut) | 0.5 |
| **Total** | **26-39 days solo, 20-25 days with PR-3 parallel** |

## Per-commit verification

```
go build ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run
make check
```

## Per-PR exit gates

See `PLAN-49.md` for full gates.

## Commit message convention

```
<type>(<scope>): <subject>

<body>

Refs: PLAN-49/<item-id>
```

`<item-id>` is one of the 49 issues tracked in `scripts/plan-coverage.yaml`.
Each commit references at least one item.

## Files in this folder

| File | Contents |
|---|---|
| `PLAN-49.md` | Top-level plan, decisions, verification matrix |
| `PHASE-0.md` | 54 commits — critical defect patches |
| `PHASE-1.md` | 19 commits — build pipeline + frontend embed |
| `PHASE-2.md` | 25 commits — public surface + dead deps + Err rename |
| `PHASE-3.md` | 16 commits — open-source hygiene (parallel-eligible) |
| `PHASE-4A.md` | 10 commits — test infrastructure |
| `PHASE-4B.md` | 5 commits — users/workspaces/projects/capabilities/versions tests |
| `PHASE-4C.md` | 5 commits — releases/executions/harness/audit/settings tests |
| `PHASE-4D.md` | 9 commits — alerting/webhooks/vault/auth/contract/providers/observations/ratelimit/health tests |
| `PHASE-5A.md` | 15 commits — foundational renames + dead code |
| `PHASE-5B.md` | 4 commits — domain renames |
| `PHASE-5C.md` | 3 commits — cross-cutting renames |
| `PHASE-6.md` | 14 commits — public pkg with build-tag fence |
| `PHASE-7.md` | 13 commits — documentation refresh |
| `PHASE-8.md` | 6 commits — v1.0.0 cut + final verification |
| `scripts/plan-coverage.yaml` | 49 critique items as YAML manifest |
| `scripts/check-plan-coverage.sh` | CI gate that fails if any item is unresolved |

## Start here

```
docs/refactor/PLAN-49.md            # overview, decisions, verification
docs/refactor/PHASE-0.md            # first PR to execute
```

## Execution order

Sequential foundation:
```
PR-0 → PR-1 → PR-2 → PR-4A → PR-4B → PR-4C → PR-4D → PR-5A → PR-5B → PR-5C → PR-6 → PR-7 → PR-8
```

Parallel:
```
PR-3 runs concurrent with PR-2 + PR-4 + PR-5
```