# ADR 0012: Providers and pricing are injected, never global

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The `internal/llm` package — central to the project — declared two
package-level mutable values:

```go
var global = newRegistry()              // in registry.go
var pricingTable = map[string]ModelPricing{...}  // in cost.go
```

Both were reached via free functions (`llm.Default()`, `llm.CalculateCost`,
`llm.GetPricing`) instead of through any explicit dependency. The
consequences were:

1. **Hidden coupling.** Callers could not tell from a function
   signature whether it consulted the global registry. Tests could
   not isolate providers from each other without resorting to
   convoluted setup/teardown of the global state.
2. **No-args grep for "what registers a provider" gave the wrong
   answer.** `llm.Register` was a method on `Registry`, not a free
   function — adding a provider required editing `newRegistry`, not
   extending via a side channel.

The Project Philosophy (charter) lists five non-negotiable
principles. Two of them are violated by these globals:

- **Principle 5 — Explicit Dependencies.** "No hidden globals. No
  singleton services. No package-level mutable state. All
  dependencies are injected."

This ADR records the move from package-level state to owned values
and explains why it is worth doing now rather than later.

## Decision

1. `Registry` becomes an explicit value. `NewRegistry()` is the
   constructor; `var global` is removed and `Default()` is gone.
   `LoadFromEnv()` is a method on `*Registry`. The HTTP server
   accepts a `*llm.Registry` via a new `api.WithProviders()`
   Option. `cmd/promptsheond` constructs the registry in its
   `buildServer` and passes it through. The CLI (`cmd/promptsheon`)
   constructs a local registry inside the affected subcommands and
   uses it directly.

2. `PricingTable` becomes an explicit value. `NewPricingTable()`
   is the constructor with the built-in OpenAI, Anthropic, and
   Ollama pricing baked in. `Calculate(model, usage)` and
   `Lookup(model)` are methods on `*PricingTable`. The free
   functions `CalculateCost` and `GetPricing` are removed; the
   only callers (`tokenizer.go`, `middleware.go`) are updated.
   `tokenizer.EstimateCost` keeps its free-function signature for
   backwards compatibility but constructs a `PricingTable` on every
   call (the cost path is sparse; the allocation is negligible). The
   Instrumented provider wrapper adds an optional `pricing
   *PricingTable` constructor argument so cost collection can be
   opt-in.

3. A **structural guard** ships alongside the refactor:
   `scripts/check-no-package-state.go` is an AST walker that fails
   CI when any domain package under `internal/` introduces a
   package-level non-error, non-import-pin `var`. The check fires in
   the `lint` job in `.github/workflows/ci.yaml` as a step before
   `golangci-lint`. `internal/release/release.go`'s `var
   AllEnvironments = [...]` was the lone violator; it is replaced
   with a `func AllEnvironments() []Environment` and a switch-based
   `Valid()`.

## Options considered

1. **Encapsulate as singleton with reset hooks.** (a) Hide the
   package-level globals behind `SetProvider(...)`-style setters
   with `t.Cleanup`-driven resets in tests. Rejected: (a) is exactly
   the kind of hidden-state scaffolding the charter rejects; tests
   still see shared state across test functions in the same binary
   unless every test remembers to call the cleanup.
2. **Move state to a `Singleton` interface implemented by `Runtime`.**
   Server holds a `Runtime`; `Runtime` holds the registry and the
   pricing table. Rejected as a separate ADR: the layering hides the
   value of doing it now. We make the smallest change that
   satisfies the charter and stop.
3. **Today: explicit values, owned by the caller, injected through
   the Option pattern. (Chosen.)** Minimal blast radius; aligns
   with the existing injection pattern in `internal/api.Server`
   (`WithAuth`, `WithEvalRunner`, `WithContextManager`, etc.).

## Consequences

Positive:

- The `internal/llm` package now satisfies Charter Principle 5 in
  full. Two meaningful violations removed.
- The CI guard prevents regression. A developer who adds a
  `var Foo = ...` to a domain package will see the build fail.
- Tests that need a registry no longer race against package-level
  state. The existing registry tests already used the unexported
  `newRegistry()`; renaming it to exported `NewRegistry()` makes
  the test author write the same code a production caller would
  write — a small but meaningful "tests use the public API" win.
- The PricingTable can now be augmented at runtime by a plugin (a
  provider advertises its pricing; the registry owns the table;
  cost computed against the unified table) without editing this
  package.

Negative:

- `PricingTable` is now constructed per-`EstimateCost` call. This is
  a tiny allocation but still a real cost; future PRs may move
  pricing construction to `cmd/promptsheond` once and thread it
  through to anyone who needs it (M0 leaves it as is for the smallest
  blast radius).
- Plugin authors that ship a custom provider must also ship a
  pricing registration (via `PricingTable.Register`) if they want
  cost metrics. The Charter Principle ("plugins replace built-ins")
  implies we will eventually publish a helper that derives pricing
  from a provider metadata table; we defer that to M1+.
- `llm.Default()`, `llm.CalculateCost`, and `llm.GetPricing` are
  removed. External SDK consumers (none today) would see a breaking
  change. Today the SDK only imports types from `llm`, not the
  helper functions, so the blast radius is zero.

## References

- `internal/llm/registry.go` — `NewRegistry`, no globals
- `internal/llm/cost.go` — `*PricingTable`, no globals
- `internal/llm/middleware.go` — `NewInstrumented(..., pricing)`
- `internal/llm/tokenizer.go` — `EstimateCost` keeps its signature
- `internal/api/server.go` — `WithProviders` Option
- `internal/api/handlers_providers.go` — uses `s.providers`
- `cmd/promptsheond/main.go` — `providers := llm.NewRegistry()` + `providers.LoadFromEnv()`
- `cmd/promptsheon/main.go` — same construction at CLI subcommand scope
- `internal/release/release.go` — `AllEnvironments()` switched to a function
- `scripts/check-no-package-state.go` — structural guard
- `Makefile` — `lint-domain` target
- `.github/workflows/ci.yaml` — lint job runs `make lint-domain`
- Charter Principle 5 ("Explicit Dependencies")
