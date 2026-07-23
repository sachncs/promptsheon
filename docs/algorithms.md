# Algorithms

The internals that aren't obvious from the API surface.

## Content-addressed storage (CAS)

Every artifact is a content-addressed blob. The CAS is the
single source of truth for the bytes of a Prompt, Model
Policy, Runtime Policy, Context Contract, Memory,
Guardrail, Tool, Knowledge Source, or MCP server reference.

A blob is keyed by its `SHA-256(canonical_encode(blob))`.
The canonical encoding rules:

- JSON: sorted keys, no insignificant whitespace, UTF-8.
- Text: raw UTF-8 (the artifact is the text).
- Binary: raw bytes (the artifact is the binary).

Two artifacts with the same hash are guaranteed to be
byte-identical; two with different hashes are guaranteed to
differ. The CAS deduplication is a hash table: the Manifest
references the same hash twice and the second reference is
free.

The `pkg/cas` package is the production implementation. It
lives at `pkg/cas/` so other Go projects can import it
without dragging in the rest of Promptsheon.

## Hash-chained audit log

The audit log is a hash chain, not an append-only list.
Each row records:

- `previous_hash` — the previous row's `entry_hash`.
- `entry_hash` — `SHA-256(canonical_encode({id, user_id,
  action, resource, details, timestamp, previous_hash}))`.

`store.VerifyAuditChain` walks the chain from `rowid 1`
forward and asserts the invariant. Any tampering — row
insertion, deletion, or mutation out-of-order — breaks the
chain; `VerifyAuditChain` returns `{ok: false, tail_mismatch:
true, …}`.

`audit_archive` (migration 011) is the retention target.
Rows older than `PROMPTSHEON_AUDIT_TTL_DAYS` are copied
there and the source row is preserved so the chain survives.
Operators archive externally and may truncate the source
table out of band.

`promptsheon_audit_dropped_total` and
`promptsheon_audit_queue_latency_seconds` surface the worker
pool's health. The `PromptsheonAuditChainBroken` alert
fires on any drop in a 5-minute window.

## Evaluation flow

The Eval Runner:

1. Load the Dataset (cases from `dataset_cases`).
2. For each case: invoke the Release, score the output
   against the case's `expected` using the chosen Scorer.
3. Persist each `EvalResult` (`passed`, `actual`, `error?`,
   `latency_ms`).
4. Persist the aggregate `EvalRun` (`passed`, `failed`,
   `total`, `score`, `status`, `started_at`, `finished_at`).
5. Increment the per-case metrics
   (`promptsheon_eval_cases_passed_total`,
   `promptsheon_eval_cases_failed_total`).

The runner is fail-fast by default; `RunAll` runs every
precondition regardless of outcome and aggregates the
errors.

Cases run serially in v0.1.x. Parallel execution ships in
a follow-on.

## Precondition execution

The Precondition Runner:

1. For each enabled Precondition on the Capability:
   1. Resolve the timeout. Zero means use
      `DefaultPreconditionTimeout` (60 seconds).
   2. Build a `context.WithTimeout`.
   3. Filter the daemon's process env to a
      `PROMPTSHEON_*` allowlist.
   4. Set the process group (`Setpgid: true`) so the
      daemon can kill the whole tree on timeout.
   5. `exec.CommandContext("sh", "-c", command)`.
   6. Capture stdout+stderr via `CombinedOutput`.
   7. Truncate to 8 KB; report `passed: true` on exit 0,
      `passed: false` otherwise.
2. If any precondition failed, return a
   `*harness.PreconditionError` aggregating the failures.

The runner is **gated** behind
`PROMPTSHEON_HARNESS_PRECONDITIONS=true`. Default is off so
unconfigured deployments don't accidentally execute hooks.

## LLM call flow

The Invoke path (`internal/invoke.Invoker`):

1. Enforce Quota (atomic counter, in-memory; backed by
   ClickHouse in production scale).
2. Call `executor.Executor.RunRequest(ctx, req, env)`.
3. Enforce Budget (post-call; refused budget returns
   `ErrBudgetExceeded` and the handler maps to 402).

`executor.Executor.RunRequest`:

1. Build an `ExecutionRecord` (Status: "running").
2. Call the registered `Caller` (the LLM provider adapter).
3. On error: set Status: "error", Error, return the record.
4. On success: copy `Output`, `PromptTokens`, `OutputTokens`,
   `Model`, `CostUSD` from the caller's `InvokeResult`; set
   Status: "ok".
5. Return the record.

`metrics.LLMMiddleware` (when observability is wired) wraps
the call to record latency, success/error counts, and an
OTel span.

## Tokens and cost

The provider's `Complete` returns `llm.Usage` (prompt tokens,
completion tokens). The cost is computed by applying the
provider's pricing table to the token counts.

| Provider | Model | Pricing |
|----------|-------|---------|
| OpenAI | gpt-4o | $5/M input, $15/M output |
| OpenAI | gpt-4o-mini | $0.15/M input, $0.60/M output |
| Anthropic | claude-opus-4 | $15/M input, $75/M output |
| Anthropic | claude-haiku-4-5 | $0.80/M input, $4/M output |

Production tenants override the pricing table at startup;
the daemon reads the per-provider config from
`llm.ProviderConfig.Extra`. The `promptsheon_llm_cost_usd_total`
metric surfaces the running total per daemon.

## Source

- `internal/store/sqlite.go` (`AppendAudit`, `computeAuditHash`,
  `VerifyAuditChain`).
- `internal/observation/observation.go` (windowed aggregator).
- `internal/optimizer/rules/rules.go` (rule engine).
- `internal/invoke/invoke.go` (Budget / Quota / LLM call).
- `internal/harness/precondition.go` (Precondition execution).
- `internal/cas/` (`pkg/cas/`, content-addressed store).
- [ADR 0003](adr/0003-hash-chained-audit-log.md) — audit chain
  design.