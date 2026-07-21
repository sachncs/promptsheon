# Architecture Decision Records

This directory contains the [Architecture Decision Records (ADRs)](https://adr.github.io/) for Promptsheon. Each ADR captures one significant design decision: the context, the options considered, the choice made, and the consequences.

ADRs are immutable. When a decision is superseded, the original ADR is updated with a `Superseded by:` footer pointing at the new ADR, and the new ADR is added with its own number. Never delete an ADR.

## Index

| Number | Title | Status |
|---|---|---|
| [0001](0001-use-cas-for-prompt-history.md) | Use content-addressable storage (CAS) for prompt history | Accepted |
| [0002](0002-bm25-over-vector-search.md) | Use BM25 instead of vector search for prompt retrieval | Accepted |
| [0003](0003-hash-chained-audit-log.md) | Use a hash-chained audit log for tamper evidence | Accepted |
| [0004](0004-aes-256-gcm-vault.md) | Use AES-256-GCM for the provider-key vault | Accepted |
| [0005](0005-hmac-webhooks-with-ssrf-allowlist.md) | Sign webhooks with HMAC and refuse private destinations by default | Accepted |
| [0006](0006-modernc-sqlite-no-cgo.md) | Use `modernc.org/sqlite` instead of `mattn/go-sqlite3` | Accepted |
| [0007](0007-slog-as-observability-foundation.md) | Use `log/slog` as the single observability foundation | Accepted |
| [0008](0008-workflow-dag-with-topological-execution.md) | Execute workflows as a topological DAG with level-by-level concurrency | Accepted |
| [0009](0009-prompt-binding-version-0-3-0.md) | Adopt prompt-binding schema version 0.3.0 | Accepted |
| [0010](0010-version-is-a-manifest-not-a-bundle.md) | Capability Version is a Manifest, not a Bundle | Accepted |
| [0011](0011-release-and-approval-aggregates.md) | Release and Approval are separate aggregates | Accepted |
| [0012](0012-providers-and-pricing-are-injected.md) | Providers and pricing are injected, never global | Accepted |
| [0015](0015-postgres-backend-with-rls.md) | Postgres as a first-class backend with per-workspace RLS | Accepted |
| [0016](0016-plugins-over-grpc.md) | Plugins over gRPC, loopback only | Accepted |
| [0017](0017-approval-release-wiring.md) | Approval→Release wiring closes quorum-reality gap | Accepted |
| [0018](0018-recommendation-loop-wired.md) | End-to-end Recommendation loop wired through Executor → Observation → Producer | Accepted |
| [0019](0019-deferred-items.md) | Deferred architecture review items | Accepted |
| [0020](0020-capability-slos.md) | Capability Service Level Objectives — Goal / Burn-rate / Repository contract | Accepted |
| [0021](0021-bandit-foundation.md) | Multi-armed bandit recommender foundation (Thompson Sampling) | Accepted |
| [0022](0022-plugin-manifest.md) | Plugin manifest (PROMPTSHEON_PLUGINS_FILE) | Accepted |
| [0023](0023-forward-only-cleanup.md) | Forward-only cleanup of the legacy bundle model | Accepted |
| [0024](0024-plugin-transport-uds.md) | Plugin transport — net/rpc over Unix-domain-socket for v0.1.x, gRPC for the M3.5 cut | Accepted |
| [0025](0025-plugin-transport-grpc.md) | Plugin transport — gRPC over loopback UDS for v1.x | Accepted |
| [0026](0026-https-only-webhooks.md) | Webhooks accept only HTTPS; per-endpoint allow_private removed | Accepted |
| [0027](0027-webhook-secret-encryption.md) | Webhook HMAC secret encrypted at rest in the vault | Accepted |
| [0028](0028-maker-checker-self-enforcing.md) | MakerCheckerPolicy self-enforces separation of duties | Accepted |
| [0029](0029-otel-sqlite-fanout.md) | HTTP spans fan out to both SQLite and OTel via trace.Multi | Accepted |
| [0030](0030-leader-election-sqlite.md) | Multi-replica deployments elect a single writer via a SQLite advisory lock | Accepted |

## Status legend

- **Proposed** — under discussion, not yet binding.
- **Accepted** — in force. Changes must update or supersede.
- **Superseded** — replaced by a later ADR. The original is preserved.
- **Deprecated** — no longer relevant, retained for history.

## Authoring a new ADR

1. Copy [`template.md`](template.md) to `NNNN-short-slug.md` where `NNNN` is the next free 4-digit number.
2. Fill every section. Do not leave `Status` as `Proposed` once the decision is implemented.
3. Add a row to the index above.
4. Link to the ADR from [`docs/design-decisions.md`](../design-decisions.md) if it is referenced from a user-facing page.
