# Frequently Asked Questions

## What is Promptsheon?

Promptsheon is the control plane for AI Capabilities. Every
Capability — its Prompt, Model Policy, Runtime Policy, Context
Contract, Memory, Guardrails, Tools, MCP servers, and
Evaluation Suite — is an immutable, content-addressed
Manifest recorded as a Directed Acyclic Graph. Production
tenants manage their Capabilities the way engineers manage
code: with versions, reviews, releases, canary deployments,
and rollbacks.

## What problem does it solve?

Without Promptsheon, an AI capability is a *prompt* plus a
*model* plus a *runtime* in some scripting language. There
is no audit, no version history, no review, no rollback, no
eval loop, no per-workspace cost or rate cap. Promptsheon
is the version-control + release-engineering + observability
layer for AI capabilities, designed for production tenants
who treat AI capabilities as production infrastructure.

## What's the difference between a Capability and a Version?

A **Capability** is the named logical unit ("summariser",
"code review"). A **Version** is an immutable build of a
Capability's Manifest. Two Capabilities that share the same
Manifest share the same underlying artifacts. The
Capability points at one *live* Version per Environment
(via the live Release).

## What's the difference between a Version and a Release?

A **Version** is an immutable build. A **Release** binds a
Version to an Environment (e.g. `prod`, `staging`) and
tracks the approval workflow. Activating a Release transitions
it from `pending` → `active` and supersedes the prior Active
Release in the same Environment.

## What's in a Manifest?

The five required artifacts (Prompt, Model Policy, Runtime
Policy, Context Contract, Memory) plus optional Guardrails,
Tools, Knowledge Sources, and MCP server references. Each
leaf is a content-addressed `(kind, hash)` reference; the
hash is the SHA-256 of the artifact's canonical encoding.
See [docs/algorithms.md](algorithms.md) for the
canonical-encoding rules.

## How do I deploy?

```bash
go build -o promptsheond ./cmd/promptsheond
./promptsheond
```

The first run creates `promptsheon.db` in the current
directory. For a production deployment see
[docs/deployment.md](deployment.md) (Helm chart, Docker
image, systemd units).

## What LLM providers are supported?

**OpenAI** and **Anthropic** in v0.2.0. Azure OpenAI, Ollama,
and NVIDIA NIM were removed in v0.1.0. To add a new provider,
implement the `llm.Provider` interface and register a factory
on the `Registry` in `cmd/promptsheond/main.go`.

## How do I add a Capability?

```bash
# Workspace > Project > Capability.
curl -X POST http://localhost:8080/api/v1/workspaces \
  -H 'Content-Type: application/json' -d '{"name":"acme"}'
curl -X POST http://localhost:8080/api/v1/workspaces/w1/projects \
  -H 'Content-Type: application/json' -d '{"name":"summariser"}'
curl -X POST http://localhost:8080/api/v1/projects/p1/capabilities \
  -H 'Content-Type: application/json' -d '{"name":"summariser"}'
```

See [docs/getting-started.md](getting-started.md) for the full
walkthrough.

## How does the approval workflow work?

The default policy is `MakerCheckerPolicy`: a Release's
creator cannot vote to approve their own release, and at
least `RequiredApprovers` (default 1) other identities must
cast an `approve` vote before `Activate` succeeds. The
alternative `MajorityPolicy` is a flat count-based threshold.

## What's the difference between `idempotency-key` and
`X-Request-ID`?

`Idempotency-Key` is for **retry safety**. A POST with an
`Idempotency-Key` header is cached for 5 minutes; subsequent
POSTs with the same key return the cached response with an
`Idempotent-Replayed: true` header. Use this when a retry
might otherwise cause double-execution.

`X-Request-ID` is for **observability**. The daemon echoes
it on every log line and surfaces it in the audit chain. Use
this when you need to correlate a request across the daemon,
your client, and any downstream services.

## Can I run multiple replicas?

Yes. Set `PROMPTSHEON_LEADER_ELECTION=true` and the daemon
acquires a SQLite advisory lock so only the leader applies
migrations and writes to the audit chain. Reads scale
linearly. SQLite + WAL handles small to medium production
loads; for high-throughput deployments, run a Postgres
backend (a follow-on — v0.2.0 is SQLite-only).

## How does pricing work?

The LLM `Provider` reports `Usage` (prompt tokens,
completion tokens); the daemon applies the provider's
pricing table to compute cost. The cost is recorded on the
Execution and surfaced via `promptsheon_llm_cost_usd_total`
and the per-Execution `cost_usd` field.

## What happens if I exceed budget?

The Invoke path consults the per-workspace Budget *after* the
LLM call completes. If the cost exceeds the cap, the
Execution row is persisted (with the partial cost) and a
`402 Payment Required` is returned. The Release stays
Pending; the next period's window starts fresh.

## What's the audit chain for?

The audit log is a **hash chain**, not an append-only list.
Each row records `previous_hash` and `entry_hash` (sha256 of
the row's canonical JSON). Any tampering — row insertion,
deletion, or mutation out-of-order — breaks the chain. Run
`GET /api/v1/audit/verify` on a cron; alert on
`promptsheon_audit_chain_verifications_total` going flat
(more than 30m without a verification call).

See [docs/security.md](security.md#audit-chain) for the
retention contract.

## Can I migrate to Postgres later?

The `store.Repository` interface is the integration boundary.
A Postgres implementation can be added without touching
the domain packages. v0.2.0 ships SQLite-only as a
deliberate simplification; the `pkg/store` Postgres driver
lives at the same level as the `modernc.org/sqlite` driver
today but isn't wired into the production wiring. See
[docs/architecture.md](architecture.md#storage-backends) for
the storage layer.

## What's the difference between `promptsheon` (CLI) and `promptsheond` (daemon)?

`promptsheond` is the long-running server binary. It owns
the SQLite database, the LLM providers, the audit chain,
and the HTTP API.

`promptsheon` is the CLI binary. It talks to a running
daemon over HTTP; it doesn't own state. See
[docs/cli.md](cli.md) for the full command surface.

## How do I extend the system?

Three extension surfaces:

- **SDK** (`sdk/`) — add a typed client method. The contract
  test in `tests/contract/contract_test.go` catches accidental
  deletions.
- **Plugins** (`PROMPTSHEON_PLUGINS_FILE`) — register a
  remote binary that implements the gRPC-over-UDS
  `PluginServer`. See [docs/architecture.md](architecture.md#plugin-supervisor).
- **Migrations** (`internal/store/migrations/`) — drop a
  `NNN_your_migration.up.sql` and the next boot applies it.