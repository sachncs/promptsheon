# Promptsheon Audit — v1.0.0 / v0.3.0

> **Scope.** Generated as a no-code-change audit. Every claim is grounded
> in a file or command in the repository.
>
> **Tree state at audit time.** HEAD is `4d629f20 docs: align README,
> --help, VERSION, CHANGELOG with v1.0.0 / current code`. The working
> tree had 254 unstaged modifications against HEAD (mostly formatting
> and whitespace; only `promptsheon/routes.go` was spot-checked and the
> file-content diffs were whitespace-only). All counts in this audit
> (routes, tests, files) refer to the on-disk state, which matches HEAD
> in shape.
>
> The audit produced exactly one new file: `AUDIT.md`. No source file
> was modified.

## Executive summary

The Go binary **builds, vets, races, and tests cleanly** across 47
packages. None of that is the problem. The product is "incomplete" in a
more pointed way: **the docs and the roadmap claim a surface area that
does not exist in the tree**, and several CI gates that the same docs
hold up as proof-of-quality **fail in this environment**.

There are **zero abandoned TODOs** in production Go code. There are no
known bugs surfaced by the test suite. The incompleteness is at the
*contract layer*: features shipped in docs, features shipped in
adjacent trees (SDKs), and features that the test gates are *supposed*
to enforce but cannot, because either the gate is broken or the
artefact it gates is missing.

Five severities, ranked P0 → P4.

---

## P0 — Broken CI gates the docs advertise as proof

### P0-1 `make lint` fails: `.golangci.yml` version is unreadable

- **Symptom**: `make lint` → `Error: can't load config: unsupported
  version of the configuration: ""`. Exits 3.
- **Evidence**: `.golangci.yml` declares no `version:` field;
  installed golangci-lint rejects it. The README still lists lint as a
  routine CI step.
- **Impact**: Every claim that "lint passes" or "lint is enforced in
  CI" is unverifiable. Either the gate is broken or the config is
  broken — both are P0 because the docs invite readers to trust the
  gate.
- **Recommendation**: Pin a `version: "2"` (or matching the installed
  binary) in `.golangci.yml`, or pin the lint binary in CI. Confirm
  `make lint` exits 0.

### P0-2 `make dist-check` fails: stale frontend embed

- **Symptom**: `FAIL: frontend/src is newer than embed; run make
  web-build`.
- **Evidence**: the embedded `frontend/dist` is older than
  `frontend/src`. `make dist-check` is wired into `make check`.
- **Impact**: Any developer who edits the frontend and commits without
  rebuilding the embed ships a silent UI regression — the daemon
  serves the old bundle, the user sees behaviour that does not match
  the code they read.
- **Recommendation**: `make web-build && make dist-check` in CI as a
  pre-merge gate. Today the gate exists but the workflow tolerates it
  being skipped.

### P0-3 `make bench` fails: benchmark registered but never executes

- **Symptom**: `BenchmarkSelect executed 0 times; expected exactly once`.
- **Evidence**: `make bench` runs `go test -run=^$ -bench=.` (or
  equivalent) on a benchmark that does not match its name; the bench
  script treats "0 executions" as failure.
- **Impact**: `ROADMAP.md` advertises "a curated benchmark set plus a
  k6 p99 gate" as a v0.3.0 acceptance criterion. The Go benchmark
  portion of that gate fails immediately on a clean tree.
- **Recommendation**: Fix the benchmark selection (either rename the
  target or fix the matcher). Distinguish "0 b.N" from "0 executions"
  if the intent is to require the bench to be wired.

---

## P0 — Features claimed in docs that do not exist

### P0-4 Python and TypeScript SDKs are advertised but absent

- **Claim**:
  - `README.md`: "REST API — Full-featured HTTP API". The OpenAPI
    spec `promptsheon/spec/spec.yaml` is generated.
  - `ROADMAP.md`: "the Go SDK, Python SDK, and TypeScript SDK each
    cover every /api/v1 route".
  - `pkg/promptsheon/CHANGELOG.md`: notes removal of the legacy
    `sdk/` import path.
- **Evidence**:
  - `sdk/python/src/promptsheon/` contains directories
    `api/default/`, `models/` and zero `.py` files (only
    `__pycache__/`).
  - `sdk/typescript/` contains only `node_modules/`.
  - `make sdk` only copies `spec.yaml` into
    `sdk/{python,typescript}/src/.../_generated/openapi.yaml`. It
    does not generate any client code.
  - `make sdk-check` only diffs against that copy; passing
    `sdk-check` does not imply any client code exists.
- **Impact**: Anyone who follows the README Quick Start for Python or
  TypeScript, or anyone who runs `pip install promptsheon` /
  `npm install @promptsheon/client` based on the README, will get a
  package with a spec file and zero client code. This is the most
  customer-visible lie in the repo.
- **Recommendation**: Either (a) generate real Python and TypeScript
  clients from the OpenAPI spec (openapi-generator, openapi-typescript,
  etc.) and check the generated code into `sdk/{python,typescript}/`;
  or (b) delete the directories and rewrite the README/ROADMAP to
  state that only the Go SDK ships today. (a) is the correct answer
  because the ROADMAP commits to it.

### P0-5 `make check` claims to run all gates; one of them is the
env-broken `goimports`

- **Symptom**: `make check` runs `goimports -w .` →
  `No such file or directory`.
- **Evidence**: `goimports` is not installed in this environment
  (`/opt/homebrew/bin/goimports` missing). The Makefile uses
  `goimports` without a guard.
- **Impact**: `make check` is not runnable in a fresh environment.
  A developer who follows AGENTS.md's Phase 8 ("make check passes")
  cannot satisfy the gate.
- **Recommendation**: `go install golang.org/x/tools/cmd/goimports@latest`
  in CI bootstrap, or use `gofmt` only.

---

## P1 — Significant SDK / server parity gap

### P1-1 Go SDK `pkg/promptsheon` covers ~43% of server routes

- **Numbers**:
  - Server routes registered in `promptsheon/routes.go`: **75**.
  - OpenAPI paths emitted by `scripts/genopenapi`: **69**.
  - Exported `Client` methods on `pkg/promptsheon/client.go`: **30**.
- **Gap**: ~39 server routes have no Go SDK client wrapper. The
  contract test (`tests/contract/contract_test.go`) probes every
  route over raw HTTP — it does **not** assert that each route has a
  corresponding `Client` method, despite its comment claiming
  "API-SDK-1: the contract test is the CI gate that catches drift
  between the OpenAPI spec and the SDK".
- **Missing from SDK** (sampled): `UpdateCapability`, `DeleteCapability`,
  every `/api/v1/users` CRUD method, every `/api/v1/audit` method,
  `ResolveAlert`, `LinkAlertRuleGroup`, `UnlinkAlertRuleGroup`,
  `CatalogSearch`, `ReasoningCompile`, `ListExecutions`,
  `CreateExecution`, `GetExecution`, `GetCapabilityContract`,
  `GetCapabilityReputation`, `GetCapabilityDiff`, `OAuthCallback`,
  `TestProvider`, all vault key methods, all alert rule methods
  except `GetAlertRule`, all webhook methods, all settings methods,
  `MetricsSummary`, `MetricsDashboard`, `LogsStream`, etc.
- **Impact**: "The SDKs match the server" (ROADMAP) is false even for
  the Go SDK, which is the only SDK that actually ships source.
- **Recommendation**: Either (a) generate the Go Client from the
  OpenAPI spec (it would mechanically close the gap and prevent
  drift), or (b) hand-write the missing ~39 methods and add a
  parity test that fails when a route registration has no
  corresponding Client method.

---

## P2 — Documented but unwired delivery milestones

### P2-1 `ROADMAP.md` v0.4.0 deliverables

| Item | Where it should live | Status |
|---|---|---|
| pgx Postgres backend | `promptsheon/store/postgres/` | **missing** |
| gRPC-over-UDS plugin transport | `promptsheon/plugins/grpc/` | **missing** |
| Multi-region design doc | `docs/research/multiregion.md` | **missing** |
| Canary release runtime | `promptsheon/canary/` | **missing** |
| LLM-judge production wiring | daemon wires JudgeClient through LLM gateway | partial (primitive ships, runtime wiring does not) |

These are *open* and self-described as v0.4.0 work in `ROADMAP.md`,
so they are not "incomplete" in the lie sense — they are honestly
deferred. The audit flags them because the user asked whether the
product is incomplete; the answer is "yes, by design, and here's
exactly which milestones."

### P2-2 `ROADMAP.md` v0.5.0 deliverables

Capability marketplace, reputation-as-market-signal, OTLP trace
visualizer, decision audit replay. All honest deferrals.

### P2-3 `todo.md` deferred compliance-refactor items

| Code | Item | Status |
|---|---|---|
| C1–C2 | Consolidate `utils.go` with validation / pagination / transport helpers | partial (file exists; contents thin) |
| D1–D2 | Move `handlers_*.go` into `promptsheon/handlers/` | blocked |
| F1–F10 | Move all tests to `tests/` | **not started** |
| G2–G9 | Drop dead code (unused `perfdb3` benchmark, `metric` sub-structs) | partial |
| H2–H4 | Update tooling (`Makefile`, `scripts/check-no-package-state.go`, `.github/workflows/ci.yaml`) | partial |
| I2–I14 | Bulk `fmt.Errorf` → `promptsheon.Errorf` (reverted at #13 to avoid import cycles) | blocked |

The "single test runner" is gated by `//go:build tests_migration`
and the `tests/` package has **zero non-test `.go` source files**.
The 124 `*_test.go` files scattered through `promptsheon/` still
own the suite; the runner is scaffolding for a future migration that
has not happened.

---

## P3 — Coverage gaps

`make coverage` reports the following packages at **0.0%** statement
coverage (no test exercises them):

- `promptsheon/testutil` (test-helper package, no tests)
- `promptsheon/testutil/harnessrepo` (test-helper package, no tests)
- `promptsheon/trace` (no test files)

Plus the `tests/unit/*` packages collectively show "[no statements]"
because they import-build but contain no production code of their own.

For context, packages with the *highest* coverage:
`workflow` 89.9%, `vault` 84.7%, `webhook` 73.1%. The repo has
real coverage in domain packages; the gaps are concentrated in
test-helper plumbing and one package (`trace`) that ships without
any test.

Not a P0 because the test suite still exercises end-to-end behaviour;
the gaps are in helper code, not in business logic.

---

## P4 — Minor / informational

- **Two `panic(` calls** in the codebase, both in test files
  (`handlers_middleware_test.go`, `tests/unit/eventbus/publisher_test.go`).
  No production-code panic calls. AGENTS.md compliant.
- **313 raw `_ = expr` discards** in the test corpus, none in
  production code outside tests. AGENTS.md compliant on the
  production side; the test pattern is normal.
- **10 `placeholder` mentions in production `.go`** — all are
  intentional semantic placeholders (`ManifestHashPlaceholder`,
  template variables, no-CAS fallback hashes), not abandoned code.
- **17 `TODO|FIXME|HACK|implement|wire up` markers in non-test,
  non-vendor `.go`** — all are documentation comments about how a
  feature works, not abandoned work. Verified by reading each.
- **Binary artefacts in tree**: `promptsheond` (70 MB), `codemod-sdktopkgshe-on`
  (3.5 MB), `promptsheon.db` (544 KB), and 119 `promptsheon/*`
  files in the root. The root is cluttered with what should be
  build-output. `.gitignore` should keep these out of VCS.

---

## What "complete" would require, ranked by leverage

1. **Fix the three broken CI gates** (P0-1, P0-2, P0-3). Single PR each.
   They are the smallest, highest-confidence changes.
2. **Either generate or delete the Python + TypeScript SDKs** (P0-4).
   Today they are worse than absent: they advertise a capability that
   does not run.
3. **Close the Go SDK route-parity gap** (P1-1). Either generate the
   Client from `promptsheon/spec/spec.yaml` (matches the README's
   "OpenAPI specification is generated" pattern) or add ~39 methods
   by hand and a parity test.
4. **Pick one of: v0.4.0 milestone, v0.5.0 milestone, or the
   compliance-refactor backlog** as the next body of work. Each is
   days-to-weeks, not a single turn.
5. **Coverage on `promptsheon/trace`** (P3) is a small follow-up.

Everything else on this list is documentation honesty (P2) — the
work is honestly deferred in the ROADMAP; the only fix needed is to
make sure the README does not over-claim.

---

## Quality gates observed in this audit

| Gate | Status | Evidence |
|---|---|---|
| `go build ./...` | ✅ exit 0 | ran at SHA `4d629f20` |
| `go vet ./...` | ✅ exit 0 | ran at SHA `4d629f20` |
| `go test -race -count=1 ./promptsheon/...` | ✅ 39/39 packages pass | ran at SHA `4d629f20` |
| `make docs-check` | ✅ exit 0 | ran at SHA `4d629f20` |
| `make purity` | ✅ exit 0 | ran at SHA `4d629f20` |
| `make openapi-check` | ✅ exit 0 (69 paths) | ran at SHA `4d629f20` |
| `make check-public` | not run | Go SDK facade builds clean (manual `go build -tags=promptsheon ./pkg/promptsheon/...` ✅) |
| `make lint` | ❌ golangci-lint config unreadable | P0-1 |
| `make dist-check` | ❌ frontend embed stale | P0-2 |
| `make bench` | ❌ benchmark does not execute | P0-3 |
| `make check` | ❌ `goimports` not installed | P0-5 |
| `make coverage` | ⚠️ `trace`, `testutil/*` at 0% | P3 |
