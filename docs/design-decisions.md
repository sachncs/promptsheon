# Design Decisions

This page is the user-facing summary of the significant engineering decisions behind Promptsheon. Each section points to the full [Architecture Decision Record (ADR)](adr/README.md) for the rationale, options considered, and consequences.

> **Note on tone.** The decisions below are recorded because they shape the public surface of the system. The goal is to make it possible for a new contributor to understand *why* a piece of code looks the way it does, not to relitigate past debates.

## Storage and data model

| Decision | One-line summary | ADR |
|---|---|---|
| Content-addressable storage for prompt history | Prompts, agents, and tool specs are stored in a Git-style Merkle DAG with SHA-256 content addressing. | [0001](adr/0001-use-cas-for-prompt-history.md) |
| Prompt-binding schema version 0.3.0 | Prompts carry an explicit `binding` block (`provider`, `model`, `parameters`, `api_key_ref`) and a `schema_version` field. | [0009](adr/0009-prompt-binding-version-0-3-0.md) |
| Pure-Go SQLite driver | The only direct dependency is `modernc.org/sqlite`. No CGO, fully static binary. | [0006](adr/0006-modernc-sqlite-no-cgo.md) |

## Search and retrieval

| Decision | One-line summary | ADR |
|---|---|---|
| BM25 over vector search for prompt retrieval | Lexical in-process BM25 (`k1=1.2`, `b=0.75`) with unigrams and bigrams. No embedding model. | [0002](adr/0002-bm25-over-vector-search.md) |

## Security

| Decision | One-line summary | ADR |
|---|---|---|
| Hash-chained audit log | Each `audit_entries` row stores the SHA-256 of itself and its predecessor. Verifiable with `GET /api/v1/audit/verify`. | [0003](adr/0003-hash-chained-audit-log.md) |
| AES-256-GCM for the provider-key vault | Single env-var key (`PROMPTSHEON_VAULT_KEY`), 96-bit random nonce per encryption, all-zero key rejected. | [0004](adr/0004-aes-256-gcm-vault.md) |
| HMAC-signed webhooks with conservative SSRF defaults | `X-Promptsheon-Signature: HMAC-SHA256(secret, body)`. Loopback and private ranges are refused unless `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE=true`. | [0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md) |

## Observability

| Decision | One-line summary | ADR |
|---|---|---|
| `log/slog` as the single observability foundation | One `*slog.Logger` per process, JSON to stderr. OTel SDK for traces/metrics when `PROMPTSHEON_OTEL_ENDPOINT` is set. | [0007](adr/0007-slog-as-observability-foundation.md) |

## Execution model

| Decision | One-line summary | ADR |
|---|---|---|
| Topological DAG with level-by-level concurrency | Cycles are rejected. Independent steps in a level run in parallel. Failure propagates to descendants. | [0008](adr/0008-workflow-dag-with-topological-execution.md) |

## Why a separate docs/adr/ directory?

ADRs are immutable, slow-moving, and reference each other. They live in a flat, numbered directory so that the order in which a decision was made is obvious from the file name. The `docs/design-decisions.md` page is the table-of-contents that points into `docs/adr/` from the user-facing docs.

## When to write a new ADR

Write an ADR when a decision:

- Introduces a new dependency or removes an existing one.
- Changes a public schema (the prompt-binding schema, the audit-chain format, the vault envelope).
- Establishes a security control or threat-model assumption.
- Replaces one algorithm with another (e.g. swapping BM25 for vector search, or AES-GCM for ChaCha20-Poly1305).
- Bounds a resource (request body size, rate limit, retention TTL).

Do **not** write an ADR for: a refactor that does not change behaviour, a bug fix with a clear cause, or the choice of a variable name.

## When to supersede an ADR

When a decision is reversed, do not delete the original. Mark its status `Superseded by NNNN`, add a `Superseded by:` footer, and write the new ADR. The history is the value.
