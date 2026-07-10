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
