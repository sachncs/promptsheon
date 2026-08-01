# Compliance Refactor — Atomic Commit Plan

61 atomic commits on `master`. Single `git push` at the end. No new branch, no PRs.

Each commit is independently buildable and testable. Verification gates after each commit:

```bash
go build ./...
go vet ./...
gofmt -l . | wc -l   # must be 0
goimports -l .       # must be empty
go test -race -count=1 ./promptsheon/...
```

---

## Part 0 — Pre-execution verifications (read-only)

- [ ] **0.1** `rg -l 'sachncs/promptsheon/backend' .` — count files importing the old path; record for blast-radius tracking.
- [ ] **0.2** `rg -l 'package backend' backend/` — list files needing the `package promptsheon` rename.
- [ ] **0.3** `rg -l '_test\.go' .` — list every test file for the move.
- [ ] **0.4** `rg 'package backend' backend/` — confirm `backend` is the only package name in `backend/`.
- [ ] **0.5** Document the package-rename scope in `docs/architecture/compliance.md` (start a new "Phase 7" section).

---

## Part A — Package rename + directory move (1 commit)

- [ ] **A1** `refactor: rename package backend to package promptsheon and move backend/ to promptsheon/`
  - `git mv backend promptsheon`
  - Update `package backend` → `package promptsheon` in every file inside `promptsheon/`
  - Update every `import "github.com/sachncs/promptsheon/backend/..."` → `import "github.com/sachncs/promptsheon/promptsheon/..."` across the repo
  - Verify: `go build ./...` must succeed; full import graph consistent

---

## Part B — Flatten 16 single-file subdirs (16 commits)

Each commit moves one file from its subdir to `promptsheon/`, changes the package to `package promptsheon`, and updates callers.

- [ ] **B1** `refactor: flatten audit/ to promptsheon/audit.go and combine with audit_workers.go`
  - Move `promptsheon/audit/keys.go` → `promptsheon/audit.go` (constants, `AuditField*`, etc.)
  - Merge body of `promptsheon/audit_workers.go` (audit worker pool, StartAuditWorkers, StopAuditWorkers) into the same `audit.go`
  - Delete `promptsheon/audit_workers.go`
  - Update callers: `audit.FieldKeyPref` → `promptsheon.FieldKeyPref`; re-export in `pkg/promptsheon/`
- [ ] **B2** `refactor: flatten election/ to promptsheon/election.go` — `election/election.go` → `election.go`; callers update `election.New` → `promptsheon.NewElector`
- [ ] **B3** `refactor: flatten retention/ to promptsheon/retention.go` — `retention/retention.go` → `retention.go`; callers update
- [ ] **B4** `refactor: flatten quota/ to promptsheon/quota.go` — `quota/quota.go` → `quota.go`
- [ ] **B5** `refactor: flatten rollups/ to promptsheon/rollups.go` — `rollups/rollups.go` → `rollups.go`
- [ ] **B6** `refactor: flatten scheduler/ to promptsheon/scheduler.go` — `scheduler/scheduler.go` → `scheduler.go`
- [ ] **B7** `refactor: flatten approval/ to promptsheon/approval.go` — `approval/approval.go` → `approval.go`
- [ ] **B8** `refactor: flatten eventbus/ to promptsheon/eventbus.go` — `eventbus/publisher.go` → `eventbus.go`
- [ ] **B9** `refactor: flatten errs/ to promptsheon/errors.go` — `errs/errors.go` → `errors.go`; sentinels become `promptsheon.Err*`
- [ ] **B10** `refactor: flatten observation/ to promptsheon/observation.go` — `observation/observation.go` → `observation.go`
- [ ] **B11** `refactor: flatten executor/ to promptsheon/executor.go` — `executor/executor.go` → `executor.go`
- [ ] **B12** `refactor: flatten invoke/ to promptsheon/invoke.go` — `invoke/invoke.go` → `invoke.go`
- [ ] **B13** `refactor: flatten ratelimit/ to promptsheon/ratelimit.go` — `ratelimit/ratelimit.go` → `ratelimit.go`
- [ ] **B14** `refactor: flatten rules/ to promptsheon/rules.go` — `rules/rules.go` → `rules.go`
- [ ] **B15** `refactor: flatten supervisor/ to promptsheon/supervisor.go` — `supervisor/supervisor.go` → `supervisor.go`
- [ ] **B16** `refactor: flatten stringsutil/ to promptsheon/utils.go` — `stringsutil/stringsutil.go` → `utils.go`; the file is the seed of the kitchen-sink `utils.go` (Part C2 will add to it)

Skipped (per user choice): `testdata/` (kept as subdir), `store/sqliteimpl/` (nested under `store/`), `testutil/harnessrepo/` (nested under `testutil/`).

---

## Part C — Kitchen-sink `utils.go` (2 commits)

- [ ] **C1** `refactor: split promptsheon/http.go into utils.go and middleware.go`
  - Move to `promptsheon/utils.go`: `JSON`, `writeJSON`, `readJSON`, `isRequestTLS`, `Func` type, `wrapHandler` method
  - Move to `promptsheon/middleware.go`: `storeAuthAdapter`, `authAuditLogger`, `ReadOnlyMiddleware`
  - Delete `promptsheon/http.go`
- [ ] **C2** `refactor: populate promptsheon/utils.go with all utility code and delete the source files`
  - Move body of `promptsheon/generateid.go` → `utils.go`; delete `generateid.go`
  - Move body of `promptsheon/validate.go` → `utils.go`; delete `validate.go`
  - Move body of `promptsheon/pagination.go` → `utils.go`; delete `pagination.go`
  - Rename `utils.go`'s `SplitCSV` (from B16) to keep it descriptive of the symbol
  - Result: `promptsheon/utils.go` is the kitchen sink for ~340 LOC of utility code

---

## Part D — Move handlers into `promptsheon/handlers/` + rename `selfevolve/` (3 commits)

- [ ] **D1** `refactor: move handlers_*.go into promptsheon/handlers/`
  - 20 source files: `git mv` each, strip `handlers_` prefix
  - All declare `package promptsheon`
  - Result: `promptsheon/handlers/alerting.go`, `audit.go`, `auth.go`, ..., `workflow.go`, `workspaces.go`
- [ ] **D2** `refactor: move routes.go into promptsheon/handlers/`
  - `promptsheon/routes.go` → `promptsheon/handlers/routes.go`
  - Single file move
- [ ] **D3** `refactor: rename selfevolve/ to evolve/`
  - `git mv promptsheon/selfevolve promptsheon/evolve`
  - Update `package selfevolve` → `package evolve` in all 5 source files
  - Update import paths in `cmd/promptsheond/daemon.go`, `cmd/promptsheon/cli_selfevolve.go` (will be `cli/evolve.go` after F5a), tests, Makefile, scripts, CI
  - Verify: `rg selfevolve . | grep -v vendor/` returns no hits

---

## Part E — Single test runner (1 commit)

- [ ] **E1** `test: introduce promptsheon_test.go single test runner`
  - Create `promptsheon/promptsheon_test.go` (real `_test.go` file; `package promptsheon_test`):
    ```go
    package promptsheon_test

    import (
        "testing"
        "github.com/sachncs/promptsheon/tests"
    )

    func TestPromptsheon(t *testing.T) {
        tests.RunAll(t)
    }
    ```
  - Create `tests/promptsheon_tests.go` (new Go package `package tests`):
    ```go
    package tests

    import "testing"

    var AllTests = []func(t *testing.T){} // populated in F10

    func RunAll(t *testing.T) {
        for _, fn := range AllTests {
            fn(t)
        }
    }
    ```
  - Verify: `go test ./promptsheon/...` runs `TestPromptsheon` which calls `tests.RunAll` (empty for now)

---

## Part F — Move all tests to `tests/` (11 commits)

Each commit moves a group of test files, renames them (drop `_test.go` suffix), and updates the package to `package tests`. Functions renamed `TestXxx` → `RunXxx`. Each function is added to `tests.AllTests` as it lands.

- [ ] **F1** `test: move backend/ top-level tests to tests/`
  - 4 files: `pagination_test.go` → `tests/api/pagination.go`; `ws_test.go` → `tests/api/ws.go`; `idempotency_test.go` → `tests/api/idem.go`; `invoke_test_helpers_test.go` → `tests/api/invoke_helpers.go`
  - Create `tests/api/` subdir
- [ ] **F2** `test: move handler tests to tests/`
  - 21 files from `promptsheon/handlers_*_test.go` → `tests/handlers/<name>.go`
  - Each becomes `package tests`; functions renamed `TestXxx` → `RunXxx`
  - `package backend` (white-box) → `package tests` (no, the file is `package tests` because it lives in `tests/`)
  - Create `tests/handlers/` subdir
- [ ] **F3** `test: move colocated package tests to tests/`
  - 28 files from `promptsheon/<sub>/*_test.go` → `tests/<name>.go`
  - E.g. `alerting/manager_test.go` → `tests/alerting.go`; `cas/*_test.go` → `tests/cas.go`; `llm/openai_test.go` → `tests/llm_openai.go`
  - All become `package tests`
- [ ] **F4** `test: flatten backend/tests/unit/ to tests/ (aggressive merge)`
  - ~43 files → ~28 merged files at `tests/<name>.go` (flat, no subdirs)
  - Aggressive merge: small files fold into primary test file (e.g. `capability/contract_test.go` + `capability/manifest_test.go` → `tests/capability.go`)
  - `tests/unit/policy/policy_test.go` is DELETED (per Group G)
  - `tests/unit/harness/perfdb3_test.go` is DELETED (per Group G)
  - Delete `promptsheon/tests/` empty directory
- [ ] **F5a** `refactor(cmd/promptsheon): move CLI subcommand files into cli/ subdir and drop cli_ prefix`
  - 4 files: `cli_cas.go` → `cli/cas.go`; `cli_harness.go` → `cli/harness.go`; `cli_selfevolve.go` → `cli/evolve.go` (also drop `self`); `cli_http.go` → `cli/http.go`
  - All `package main`
- [ ] **F5b** `refactor(cmd/promptsheon): move cli.go dispatcher into cli/ subdir`
  - `cmd/promptsheon/cli.go` → `cmd/promptsheon/cli/cli.go`
  - Cmd root now contains only `main.go`
- [ ] **F5c** `test: move cmd/promptsheon tests to tests/`
  - 3 files: `cli_test.go` → `tests/cli.go`; `cli_selfevolve_test.go` → `tests/evolve.go` (drop `cli_selfevolve_` and `_test.go`); `cli_e2e_test.go` → `tests/e2e_cli.go` (avoid collision with `tests/e2e/` daemon subdir)
  - All become `package tests`
- [ ] **F6** `test: move cmd/promptsheond tests to tests/`
  - 4 files: `daemon_test.go` → `tests/daemon.go`; `daemon_evolver_test.go` → `tests/daemon_evolver.go`; `e2e_provider_test.go` → `tests/e2e_provider.go`; `frontend_test.go` → `tests/frontend_embed.go`
- [ ] **F7** `test: move pkg/, scripts/, tla/ tests to tests/`
  - 3 files: `pkg/promptsheon/public_test.go` → `tests/sdk/public.go`; `scripts/genopenapi/main_test.go` → `tests/genopenapi/main.go`; `tla/release_lifecycle_test.go` → `tests/tla/release_lifecycle.go`
  - Create `tests/sdk/`, `tests/genopenapi/`, `tests/tla/` subdirs
- [ ] **F8** `test: move tests/chaos and tests/contract to tests/`
  - 2 files: `tests/chaos/sqlite_kill_test.go` → `tests/chaos.go`; `tests/contract/contract_test.go` → `tests/contract.go`
  - Delete `tests/chaos/`, `tests/contract/` empty directories
- [ ] **F9** `test: consolidate 17 test-support files into tests/test_support.go`
  - Merge `promptsheon/handlers_test_support_*.go` (17 files) into a single `tests/test_support.go` (~1073 LOC)
  - Drop the "Code generated by tools/refactor-mockstore. DO NOT EDIT." comments; the file is now hand-maintained
- [ ] **F10** `test: build tests.AllTests slice incrementally`
  - Final commit populating `tests.AllTests` with every `Run*` function defined in `tests/`
  - Each function added in the order the source files were moved

---

## Part G — Drop dead code (9 commits)

- [ ] **G1** `cleanup(handlers): drop 60 duplicate godoc lines`
  - 14 handler files: 60 lines total
  - Targeted awk: confirm next non-comment line is `func (s *Server) handle…` before deletion
- [ ] **G2** `cleanup(tests): drop unreachable perfdb3 benchmark`
  - Delete `tests/unit/harness/perfdb3_test.go` (build tag never set in any Makefile/CI target)
- [ ] **G3** `cleanup: drop orphan types in observation/recommendation/alerting/eventbus`
  - Delete `observation.Source`, `recommendation.SourceFunc`, `eventbus.Subscribe`/`Publish` type aliases, `alerting.StatusPending`/`StatusResolved`
- [ ] **G4** `cleanup(trace): drop orphan SpanFilter, UserID* and span ID generators`
  - Delete `SpanFilter`, `UserIDContextKey`, `WithUserID`, `UserIDFromContext`, `WithSpanContext`, `GenerateID`, `GenerateTraceID`, `StatusUnset`
- [ ] **G5** `cleanup(metrics): drop RecordBanditSelection and bandit private fields`
  - Delete `RecordBanditSelection` method + `banditSelections`/`banditMu`/`banditRunID` private fields
- [ ] **G6** `cleanup(metrics): drop unused Summary sub-structs (Review/Guardrail/Hallucination)`
  - Delete `Summary.ReviewMetrics`, `Summary.GuardrailMetrics`, `Summary.HallucinationMetrics` + their source fields
  - Pre-verify: `rg 'HallucinationFunc|GuardrailFunc|ReviewCaseOutcome' .` (excluding vendor)
- [ ] **G7** `cleanup(handlers): drop unused ResolveAndValidateWebhook`
  - Delete `ResolveAndValidateWebhook` from `promptsheon/handlers_webhooks.go` (~25 LOC)
  - Remove its test in `promptsheon/handlers_webhooks_test.go`
  - Pre-verify: `rg ResolveAndValidateWebhook .` (excluding vendor + tests)
- [ ] **G8** `cleanup: inline resetAuditForTest at the two call sites`
  - Move the body into the two test files that use it (the Phase 1.3 audit tests)
  - Delete the `audit_workers.go` definition
  - Pre-verify: `rg resetAuditForTest .` (excluding vendor)
- [ ] **G9** `cleanup: delete orphan policy package`
  - Move `tests/unit/policy/policy_test.go` contents to `promptsheon/policy_test.go` (white-box)
  - Delete `promptsheon/policy.go` (321 LOC) and `tests/unit/policy/` directory
  - Pre-verify: `rg 'backend\.Bundle|backend\.Policy\b' .` (excluding vendor) returns no hits

---

## Part H — Tooling updates (4 commits)

- [ ] **H1** `chore(Makefile): update globs that referenced backend/handlers_*.go`
  - Find and update any `handlers_*.go` globs in `Makefile`
  - Verify: `grep -n 'handlers_\*' Makefile` returns no hits after the move
- [ ] **H2** `chore(scripts): update check-domain-purity.sh and check-no-package-state.go`
  - Update package name lists in the two domain-purity scripts
  - Update `domainPackages` in `scripts/check-no-package-state.go`
- [ ] **H3** `chore(CI): update .github/workflows paths after rename`
  - Update `working-directory` paths in `.github/workflows/ci.yaml`
  - Update test commands to `go test ./promptsheon/...` and `make openapi-check`
- [ ] **H4** `chore(openapi): regenerate spec after rename`
  - Run `make openapi` to regenerate `promptsheon/spec/spec.yaml`
  - Verify: `make openapi-check` exits 0

---

## Part I — Replace `fmt` with `log` (14 commits)

Goal: zero `fmt` imports in the repo except `promptsheon/errors.go`.

- [ ] **I1** `refactor: introduce promptsheon.Errorf and promptsheon.New in promptsheon/errors.go`
  - Add to `promptsheon/errors.go`:
    ```go
    package promptsheon

    import "fmt"  // ONLY fmt import in the repo

    func Errorf(format string, args ...any) error {
        return fmt.Errorf(format, args...)
    }

    func New(text string) error {
        return errors.New(text)
    }
    ```
  - Verify: `rg "^\s*\"fmt\"" --type go . | grep -v vendor` returns only `promptsheon/errors.go`
- [ ] **I2** `refactor(store): replace fmt.Errorf with promptsheon.Errorf`
  - ~10 files in `promptsheon/store/`
  - Drop `"fmt"` import from each; add `"github.com/sachncs/promptsheon/promptsheon"`
  - Verify: `rg "fmt\." promptsheon/store/` returns no hits
- [ ] **I3** `refactor(auth): replace fmt.Errorf with promptsheon.Errorf`
  - ~5 files in `promptsheon/auth/`
- [ ] **I4** `refactor(capability): replace fmt.Errorf with promptsheon.Errorf`
  - ~6 files in `promptsheon/capability/`
- [ ] **I5** `refactor(cas): replace fmt.Errorf with promptsheon.Errorf`
  - ~12 files in `promptsheon/cas/`
- [ ] **I6** `refactor(llm): replace fmt.Errorf with promptsheon.Errorf`
  - ~6 files in `promptsheon/llm/`
- [ ] **I7** `refactor(handlers): replace fmt.Errorf with promptsheon.Errorf`
  - ~14 files in `promptsheon/handlers/`
- [ ] **I8** `refactor(promptsheon root): replace fmt.Errorf with promptsheon.Errorf`
  - ~10 files at the `promptsheon/` root
- [ ] **I9** `refactor(cmd): replace fmt.Errorf with promptsheon.Errorf`
  - ~8 files in `cmd/`
- [ ] **I10** `refactor(cmd): replace fmt.Print* with slog in CLI tools`
  - `cmd/promptsheon/cli/cas.go`, `cli/harness.go`, `cli/evolve.go`, `cli/cli.go`, `cmd/promptsheond/daemon.go`
  - `slog.Default()` configured with `slog.NewTextHandler(os.Stderr, nil)` at startup
  - Add `promptsheon/logging.go::SetupCLIHandler()` helper
- [ ] **I11** `refactor(genopenapi): replace fmt.Fprintf to os.Stderr with slog`
  - `scripts/genopenapi/main.go`: ~40 `fmt.Fprintf(os.Stderr, ...)` calls → `slog.Warn(...)` / `slog.Error(...)` with structured fields
- [ ] **I12** `refactor(genopenapi): replace fmt.Fprintf to bytes.Buffer with writeYAML helper`
  - `scripts/genopenapi/main.go`: ~30 `fmt.Fprintf(buf, "  %s: %s\n", ...)` calls → `writeYAML(buf, indent, key, value)` helper
  - The `writeYAML` helper uses `strings.Repeat`, `strconv.Itoa`, `strconv.Quote`, `json.Marshal` (no `fmt`)
- [ ] **I13** `chore: drop the fmt import from any file that no longer uses it`
  - `goimports -w .` to clean up unused imports
  - Verify: `rg "^\s*\"fmt\"" --type go . | grep -v vendor` returns only `promptsheon/errors.go`
- [ ] **I14** `chore(docs): document the promptsheon/errors.go fmt exception`
  - Update `docs/architecture/compliance.md` with the new "Phase 7" section listing all 5 deliberate exceptions:
    1. Kitchen-sink `utils.go` (AGENTS.md §"File Organization")
    2. Tests in `tests/` with no `_test.go` suffix (Go toolchain convention)
    3. Single `TestPromptsheon` runner (AGENTS.md §"Fail Fast")
    4. `package promptsheon` (AGENTS.md §"Package Naming" — stutters with module name)
    5. Single `fmt` import in `promptsheon/errors.go` (AGENTS.md §"Error Wrapping")

---

## Final gate (after commit I14)

- [ ] **Final** Push to `origin/master`
  ```bash
  git push origin master
  ```

---

## Verification commands (any point)

```bash
# Single fmt import
rg "^\s*\"fmt\"" --type go . | grep -v vendor
# expected: promptsheon/errors.go

# No _test.go outside the runner
rg '_test\.go$' --type go . | grep -v vendor | grep -v "promptsheon/promptsheon_test.go"
# expected: no output

# All tests reachable via the runner
go test ./promptsheon/...
go test -race -count=1 ./promptsheon/...

# Lint, vet, fmt
make fmt
make vet
make lint-domain
make docs-check
make openapi-check
```

---

## Commit-count summary

| Part | Commits |
|---|---|
| 0 — pre-execution | 0 (read-only) |
| A — package rename | 1 |
| B — flatten subdirs | 16 |
| C — kitchen-sink utils.go | 2 |
| D — handlers + selfevolve rename | 3 |
| E — test runner | 1 |
| F — move all tests | 11 |
| G — drop dead code | 9 |
| H — tooling updates | 4 |
| I — replace fmt with log | 14 |
| Final — push | 0 |
| **Total** | **61 atomic commits** |
