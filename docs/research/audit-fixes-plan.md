# Plan — Six audit fixes

> Generated 2026-XX-XX against HEAD `4d629f20`. Scope set by user:
> P0-1, P0-2, P0-3, P0-4, P1-1, v0.4.0, P3 (trace coverage).

## What the audit turned up that changes the plan

Before writing code I re-verified the audit. Six new facts matter:

1. **CI itself uses `golangci/golangci-lint-action@v9` with `version: latest`**
   (`.github/workflows/ci.yaml`). That action pulls the same v2.x binary
   we have locally, so P0-1 is broken in CI as well as locally.
2. **CI runs `staticcheck` as a separate step** (not as a sub-step of
   golangci-lint). That is already a working lint gate in CI; the broken
   one is the local `make lint` target and the `golangci-lint-action`.
3. **CI has `sdk-python` and `sdk-typescript` jobs that call
   `bash sdk/python/scripts/codegen.sh`** — those scripts **do not
   exist** on disk. So the `sdk-python` and `sdk-typescript` CI jobs
   fail at the codegen step today. Deleting the SDK directories without
   removing these CI jobs would *not* regress them — they already fail.
4. **`tests/contract/contract_test.go` has `TestSDKExposesMandatoryMethods`**
   driven by `sdkMandatoryMethods()`. That list contains 30 names
   matching the existing Client methods exactly. So there *is* a parity
   gate, but it only asserts the current 30 methods exist — it does not
   enforce "every route in the spec has a corresponding Client method".
   P1-1 is therefore "extend the gate to cover all 69 spec paths", not
   "create a new gate".
5. **The curated bench list `scripts/benchmarks.txt` includes
   `BenchmarkSelect`**, which is the bandit/Thompson-sampling
   benchmark. The bandit code is now in `promptsheon/recommendation/`
   and has no benchmark. So P0-3 fix is "add `BenchmarkSelect` back
   into `promptsheon/recommendation/`", not "rename something else to
   satisfy the gate".
6. **`promptsheon/trace` is 673 LoC of production code with zero
   tests.** The other two zero-coverage packages (`testutil`,
   `testutil/harnessrepo`) are test helpers by design — adding tests
   for them is mostly low-value. Trace is the only one with a real gap.

## Plan, six PRs in dependency order

### PR 1 — Fix P0-1: lint gate

**Goal.** `make lint` exits 0 and `golangci-lint-action` in CI does not
fail the build.

**Decision.** Drop the broken local `make lint` target and the broken
`golangci-lint-action` step in CI, replacing both with the
already-working `staticcheck` step CI already runs. This is the path
of least resistance and least surprise — `staticcheck` was already
authoritative.

**Changes.**
- Delete `make lint` target from `Makefile`. Replace `check: ... lint
  ...` with `check: ... lint-staticcheck ...`.
- Replace `.golangci.yml` with a `scripts/run-staticcheck.sh` wrapper
  that installs + runs staticcheck with the same rules the old
  golangci-lint config enabled (gofmt-suggest, govet, errcheck,
  ineffassign, unused, staticcheck).
- Remove the `golangci-lint` job from `.github/workflows/ci.yaml`.
- Update `AGENTS.md` "Quality Gates" section to drop golangci-lint and
  point at staticcheck.
- Update `README.md` "Quality Gates" reference if any.

**Verification.** `make lint-staticcheck` exits 0; CI's `lint`
job passes without the `golangci-lint` step.

**Risk.** `staticcheck` may flag legacy debt that golangci-lint
previously suppressed. Mitigate by running staticcheck against the
diff only first (CI's existing pattern), and accepting that any
pre-existing warnings get fixed in PR 2 / 3 alongside their owners.

---

### PR 2 — Fix P0-3: bench gate

**Goal.** `make bench` exits 0.

**Decision.** Add `BenchmarkSelect` to `promptsheon/recommendation/`
covering the Thompson-sampling selection path. This is the benchmark
the curated list was *intending* to track per its docstring, and the
bandit code now lives in `recommendation/`.

**Changes.**
- New file `promptsheon/recommendation/benchmark_test.go` with
  `func BenchmarkSelect(b *testing.B)` driving the existing
  recommendation selection through a small fixture (Workspace, 10
  arms).
- Verify `make bench` runs all 8 curated benchmarks exactly once and
  exits 0.

**Verification.** `make bench` exits 0; each of the 8 names in
`scripts/benchmarks.txt` reports `-1   Ns/op` exactly once.

---

### PR 3 — Fix P0-2: dist-check gate

**Goal.** `make dist-check` exits 0.

**Decision.** Rebuild the embed. The root cause is that `frontend/src`
was modified after `cmd/promptsheond/frontend/dist/index.html` and
the embed was not regenerated. There is no design choice here — the
embed must reflect the source. The fix is one command.

**Changes.**
- `make web-build && git add cmd/promptsheond/frontend/dist`.
- Verify `make dist-check` exits 0.

**Verification.** `make dist-check` exits 0; `find frontend/src
-newer cmd/promptsheond/frontend/dist/index.html -type f | wc -l`
returns 0.

**Risk.** `web-build` requires node 22 and a fresh `npm ci`. On a
clean runner it is ~30s. If the npm build fails for unrelated reasons
(frontend dep churn), the fix has to wait for that. Confirm node and
`npm ci` succeed before committing the regenerated dist.

---

### PR 4 — Fix P0-4: kill the non-existent SDKs

**Goal.** No Python or TypeScript SDK advertised anywhere. Only the Go
SDK ships.

**Decision.** Delete the empty directories, drop the failing CI jobs,
update the README and ROADMAP. The alternative (generate real Python +
TypeScript clients) is also legitimate but is a multi-day body of work
in its own right; the user explicitly chose to *delete* the SDKs.

**Changes.**
- Delete `sdk/python/` and `sdk/typescript/` entirely.
- Remove the `make sdk` and `make sdk-check` targets from
  `Makefile`. (The `check` target depends on neither, so removing
  them is safe.)
- Remove the `sdk-python` and `sdk-typescript` jobs from
  `.github/workflows/ci.yaml`. Remove the `SDK Python codegen + compile`
  and `SDK TypeScript codegen + typecheck` steps from the `test`
  job.
- `README.md`: replace every reference to "Python SDK" / "TypeScript
  SDK" with "Go SDK only". The README has the claim in the Quick Start
  section. Note in the changelog that v1.0.0 ships Go SDK only.
- `ROADMAP.md`: remove the sentence "the Go SDK, Python SDK, and
  TypeScript SDK each cover every /api/v1 route" from v0.3.0 acceptance.
  Add a note that the Python + TypeScript SDKs were removed in v1.0.0
  pending future generator work; only the Go SDK ships.
- `pkg/promptsheon/CHANGELOG.md`: add an entry for v1.0.0 noting the
  Python + TypeScript SDK directories were removed (they were already
  empty — only the generated spec copy lived there).
- Update `AGENTS.md` Phase 4 references to "the SDKs" to refer to
  "the Go SDK" where appropriate.

**Verification.** `find sdk -maxdepth 2 -type d` shows only `pkg/`
remains. CI's `test` job no longer has SDK codegen steps; the
`sdk-python` and `sdk-typescript` jobs are gone.

**Risk.** This is a user-visible breaking change. Anything published
that pointed at `sdk/python` or `sdk/typescript` will break. The
directories are empty today, so no published artefact contains real
SDK code — only the OpenAPI spec copy. The breaking change is
*honest*: the directories were misleading.

---

### PR 5 — Fix P1-1: extend the SDK parity gate to cover every spec path

**Goal.** Every route in `promptsheon/spec/spec.yaml` has a
corresponding `*promptsheon.Client` method, and the contract test
fails CI on drift.

**Decision.** Two-part change:
- **Part A:** Add the missing ~39 `Client` methods to
  `pkg/promptsheon/client.go` (or a new file in the same package).
  Each method is a thin wrapper around an HTTP call to its route. The
  shape of each wrapper is deterministic from the spec — same path,
  same method, same request/response types.
- **Part B:** Replace the static `sdkMandatoryMethods()` slice with a
  *generated* list derived from `promptsheon/spec/spec.yaml` at test
  time. The contract test should fail if any spec path lacks a Client
  method, regardless of whether anyone remembered to update a list.

For Part B, the spec has zero `operationId` fields (verified). The
generator's natural mapping is `path + method → Client method name`.
We need a deterministic mapping. Options:
  - (a) Add `operationId` to every path in the OpenAPI generator.
  - (b) Derive the name from path + method at test time, e.g.
    `GetHealth` ↔ `GET /health`, `PostSetup` ↔ `POST /api/v1/setup`,
    `DeleteCapability` ↔ `DELETE /api/v1/capabilities/{id}`.
  - (c) Hand-author a `mandatoryMethods()` slice keyed off the spec
    and update both sides on every change.

(a) is the right call. The OpenAPI generator already exists
(`scripts/genopenapi`); extend it to emit an `operationId` per path
using the same path-method convention the existing Client methods
already use. Then the contract test's parity check becomes
mechanical: "for every operationId in the spec, a method of that name
exists on `*Client`".

**Changes.**
- Extend `scripts/genopenapi` to emit `operationId` for every path.
- Add the ~39 missing Client methods to `pkg/promptsheon/client.go`.
- Replace `sdkMandatoryMethods()` with a generated equivalent in
  `tests/contract/contract_test.go` that walks the spec and checks
  each operationId against the Client's reflect.TypeOf method set.
- Document the new convention in `pkg/promptsheon/CHANGELOG.md`.

**Verification.** `go test -count=1 ./tests/contract/...` passes; the
parity test fails if any spec path has no Client method; the spec
generator emits `operationId` for every path.

**Risk.** This is the largest PR in the plan. It is also the highest-
leverage — once in place, future SDK drift is mechanically caught.
Estimated scope: ~39 new methods (1-3 LoC each on average) +
~30 LoC of contract test changes + ~20 LoC in `scripts/genopenapi`.
Total ~150 LoC of new code; the heavy lifting is in writing the
boilerplate correctly. Per AGENTS.md, this will be split into reviewable
sub-PRs if it grows beyond ~200 LoC.

---

### PR 6 — v0.4.0

**Goal.** Land at least one of the v0.4.0 deliverables from the
ROADMAP.

**Decision (committed).** Land **Canary Release primitive** as the
v0.4.0 milestone. Reasoning:
- **Canary** has the lowest blast radius: it is a runtime switch over
  existing releases, not a new storage backend or transport. A
  regression is contained to the routing layer.
- **pgx** requires the user to run a real Postgres instance to
  verify — I do not have one available in this environment, so I
  cannot honestly claim "pgx backend complete" at the end.
- **gRPC** requires the user to author a new plugin host and a
  compatibility shim over `net/rpc`; that's a days-long rewrite
  with no way to validate end-to-end without UDS test harnesses.
- **Multi-region** is multiple weeks of work even before considering
  consistency semantics.
- **LLM-judge production wiring** is the runtime version of an
  already-shipped primitive; lower-impact than canary.

Canary is the right size for "one body of work", and
`docs/reference/canary.md` already exists as the design doc — the
PR is "implement the spec that's already written".

**Scope.** Per `docs/reference/canary.md`, the runtime is:
- New field on the release table (`canary_percent`).
- New field on the invoke path: when a Capability is invoked, the
  resolver routes `canary_percent`% of traffic to the new Version
  and the rest to the current stable Version.
- Promote (existing) atomically supersedes the canary.
- Audit chain records the canary creation, promotion, and rollback.

**Changes** (sketch — refined in the PR):
- Add `canary_percent INT NOT NULL DEFAULT 0` column to the
  releases table; SQLite migration `003_canary_schema.go`.
- Extend `Release` struct, SQLite store, and handler to accept
  and persist `canary_percent`.
- Extend the invoke-path resolver to pick between current and
  canary with weighted sampling (deterministic per-call so tests
  can pin it).
- Extend audit chain with canary lifecycle events.
- Tests: unit (resolver sampling), integration (handler), contract
  (route reachability via the SDK method added in PR 5).
- Document in `docs/reference/canary.md` + `CHANGELOG.md`.

**Verification.** `go test -race -count=1 ./...` passes; the contract
test reaches the canary route; the canary schema migrates cleanly on
a v0.3.0 database.

**Risk.** This is the largest single PR. If it exceeds ~500 LoC of
new code, split into PR 6a (schema + store) and PR 6b (resolver +
audit).

---

### PR 7 — P3: trace test coverage

**Goal.** `promptsheon/trace` has non-zero statement coverage.

**Scope.** Three files (`exporter.go`, `otel.go`, `tracer.go`,
673 LoC total). Tests should cover:
- `exporter.go`: span export against a stub `SpanSink`
  (interface in the file).
- `otel.go`: OTel tracer provider construction + shutdown (the
  pre-existing `testutil/otel.go` has a TODO referencing this).
- `tracer.go`: span start / end / context propagation through the
  in-process recorder.

**Changes.**
- New file `promptsheon/trace/exporter_test.go`.
- New file `promptsheon/trace/otel_test.go`.
- New file `promptsheon/trace/tracer_test.go`.
- Tests use the existing `trace/internal/recorder` if present, or
  add a tiny in-memory recorder if not.

**Verification.** `go test -count=1 -coverprofile=cover.out
./promptsheon/trace/...` shows `coverage: 60.0% of statements` or
higher; `make coverage`'s per-package floor (`trace` is in
`promptsheon/trace/` which the per-package script checks) passes.

---

### Deferred — compliance-refactor backlog

The user asked to pick v0.4.0 *and then* the compliance-refactor
backlog. PRs 1-7 do not touch the compliance items (C/D/F/G/H/I).
The backlog can be tackled after PR 7 lands, but it is not in scope
for this plan.

Specifically deferred from the backlog:
- F1–F10 (move tests to `tests/`) — needs a separate plan. The
  "single test runner" is already gated by `-tags=tests_migration`,
  but the scattered `*_test.go` files still own the suite.
- D1–D2 (handlers split) — blocked by `oauthStateStore` cross-package
  reference.
- I2–I14 (fmt.Errorf → promptsheon.Errorf) — blocked by import-cycle
  risk that was reverted in commit `35c59c6d`.

These are real but secondary.

---

## Sequencing and dependencies

PR 1 (lint) and PR 3 (dist-check) and PR 2 (bench) are independent
and can run in any order. PR 4 (delete SDKs) is independent of PR 5
(SDK parity) but easier to review *after* PR 5 lands the parity gate
— otherwise reviewers will ask "but you just said the SDK doesn't
exist for non-Go languages, why are you adding parity?". So the
ordering is:

  PR 1 (lint)        ─┐
  PR 2 (bench)        ─┼─►  PR 4 (delete SDKs)  ─►  PR 6 (canary)  ─►  PR 7 (trace)
  PR 3 (dist-check)   ─┘                          │
                                                   └──►  PR 5 (parity) — last, lands on top of PR 4

PR 5 *must* land after PR 4 so the README is honest when the parity
gate is reviewed. PR 6 (canary) needs PR 5's new Client method for
the canary route; PR 7 (trace) is independent of all of the above.

---

## What I will NOT do

- I will not modify any production code in this turn beyond what the
  plan describes. If I discover an additional issue while
  implementing, I will surface it, not silently fix it.
- I will not touch the v0.5.0 items. The user picked v0.4.0.
- I will not land the compliance-refactor items. The user said "then",
  i.e. after these land.
- I will not regenerate the Python + TypeScript SDKs as an
  alternative to deletion. The user explicitly chose deletion.

---

## Review check

This plan satisfies AGENTS.md Phase 3 ("Before implementation, create
a plan") and Phase 4 (Design Review: prefer extending existing
components over creating new ones — see PR 5's "extend the existing
parity gate" and PR 1's "use the existing staticcheck step").

Items that warrant your attention before I start:

1. **PR 1:** is dropping `golangci-lint` in favor of `staticcheck`
   acceptable, or do you want me to keep golangci-lint and fix its
   config (pin v1.x binary in CI)?
2. **PR 4:** confirm you want the SDK directories deleted from disk,
   not just emptied.
3. **PR 6:** confirm Canary is the v0.4.0 deliverable you want. The
   alternatives are pgx, gRPC, multi-region, LLM-judge wiring.
4. **PR 7:** confirm you want trace tests in the same plan, not as a
   follow-up.

If any of those is wrong, tell me which and I'll adjust before
touching code.
