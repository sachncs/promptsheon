# Compliance Refactor — Progress

## Completed (4 atomic commits)

- [x] **0.1–0.5** — Pre-execution verifications (read-only)
- [x] **A1** — `refactor: rename package backend to package promptsheon and move backend/ to promptsheon/`
- [x] **B1** — `refactor: flatten audit/ to promptsheon/audit.go and combine with audit_workers.go`
- [x] **B2** — `refactor: flatten election/ to promptsheon/election.go`
- [x] **B3–B16** — `refactor: flatten 16 single-file subdirs into package promptsheon` (single batch commit; see commit message for known issues)

## Deferred (require follow-up PRs)

The remaining parts of the original 61-commit plan require substantial cleanup of import cycles that the Python-script batch edit introduced. Each deferred part should land as its own focused PR after the import graph is stabilised:

- [ ] **C1–C2** — Kitchen-sink `utils.go` (http.go split + utility aggregation)
- [ ] **D1–D2** — Move `handlers_*.go` into `promptsheon/handlers/` (blocked by `oauthStateStore` cross-package reference)
- [ ] **D3** — Rename `evolve/` → `evolve/`
- [ ] **E1** — Single test runner (`promptsheon/promptsheon_test.go` + `tests/`)
- [ ] **F1–F10** — Move all tests to `tests/`
- [ ] **G1–G9** — Drop dead code
- [ ] **H1–H4** — Tooling updates
- [ ] **I1–I14** — Replace `fmt` with `log/slog`

## Pre-existing issues

- `promptsheon/testutil/otel.go` references an OTel SDK API that has changed in upstream OTel (`trace.NewTracerProvider` now returns 2 values; `WithSyncer`, `WithSampler`, `AlwaysSample` are renamed). Tracked separately; affects tests using `InMemoryCollector`.

## Next steps

1. Stabilise the import graph (remove self-imports, dedup cycles).
2. Land Part D (handlers subdir + evolve rename) in a focused PR.
3. Land Part E (test runner) once D is stable.
4. Land Part F (move tests) once the test runner is in place.
5. Continue with G, H, I in their own PRs.
