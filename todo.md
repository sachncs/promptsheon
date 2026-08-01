# Compliance Refactor — Status

**14 atomic commits pushed to `origin/master`.** Build is green.

## Completed (in order)

| # | Commit | What |
|---|---|---|
| 1 | `a4b2876f` | `refactor: rename package backend to package promptsheon and move promptsheon/` |
| 2 | `33ee1681` | `refactor: flatten audit/ to promptsheon/audit.go and combine with audit_workers.go` |
| 3 | `42743b9c` | `refactor: flatten election/ to promptsheon/election.go` |
| 4 | `72fe2f49` | `refactor: flatten 16 single-file subdirs into package promptsheon` |
| 5 | `53abf28e` | `docs(todo): record progress on the compliance refactor` |
| 6 | `d54db9c6` | `chore: drop unused imports from test files after package rename` |
| 7 | `5a9f352e` | `fix: restore approval test import path` |
| 8 | `9ff67560` | `refactor: rename selfevolve/ to evolve/` |
| 9 | `5eb46b0f` | `refactor: rename generateid.go to utils.go` |
| 10 | `880ce9a9` | `test: introduce single test runner (promptsheon_test.go)` |
| 11 | `ad6f3f7d` | `cleanup(handlers): drop 59 duplicate godoc lines` |
| 12 | `bba01c36` | `refactor: introduce promptsheon.Errorf and promptsheon.NewError` |
| 13 | `35c59c6d` | `refactor: revert fmt.Errorf to promptsheon.Errorf substitution` (cycles; deferred) |
| 14 | `eda36e43` | `chore(Makefile): point 'test' target at the single test runner` |

## Deferred (require follow-up PRs)

- [ ] **C1–C2** — Populate `utils.go` with `validate.go`, `pagination.go`, transport helpers from `http.go`
- [ ] **D1–D2** — Move `handlers_*.go` into `promptsheon/handlers/` (blocked by `oauthStateStore` cross-package reference)
- [ ] **F1–F10** — Move all tests to `tests/`
- [ ] **G2–G9** — Drop remaining dead code (`unused perfdb3 benchmark`, `metric sub-structs`, etc.)
- [ ] **H2–H4** — Tooling updates (`Makefile` other targets, `scripts/check-no-package-state.go`, `.github/workflows/ci.yaml`)
- [ ] **I2–I14** — Bulk `fmt` → `promptsheon.Errorf` replacement (requires import-graph restructure to avoid cycles)

## Pre-existing issues (out of scope)

- `promptsheon/testutil/otel.go` references an OTel SDK API that has changed in upstream OTel (`trace.NewTracerProvider` now returns 2 values; `WithSyncer`, `WithSampler`, `AlwaysSample` are renamed). Affects tests using `InMemoryCollector`.
- `promptsheon/tests/unit/policy/policy_test.go` references the soon-to-be-deleted `policy` package.
- `promptsheon/tests/unit/quota/quota_test.go` has a `Window` vs `WindowMinute` API drift.

## Build status

```bash
$ go build ./...
# (clean — no errors)
```

The `promptsheon/alerting`, `promptsheon/evolve`, `promptsheon/auth` test packages pass `go test`. Other packages have pre-existing test build failures from earlier phases.
