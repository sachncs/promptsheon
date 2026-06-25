# ADR 0001: Use content-addressable storage (CAS) for prompt history

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

Prompts change. We need a way to store the full history of a prompt — every edit, every variable schema, every model binding — and to address any historical version by a stable, content-derived identifier. The history must survive process restarts, be diffable, and dedupe identical content.

Relational rows with a `version` column and a `created_at` timestamp satisfy part of this, but they do not dedupe, they do not provide tamper evidence, and they cannot be referenced from external systems with a stable ID. We also want prompts, agent configurations, and tool specs to share a single storage model so we can compose them.

## Decision

We use a Git-style content-addressable storage (CAS) as the single storage layer for all versioned assets. Every object is identified by the SHA-256 of its content. Identical content always produces the same hash. Objects are immutable, write-once, and gzip-compressed on disk.

Four object types form a Merkle DAG:

- **Blob** — raw content (prompt text, tool spec, YAML).
- **Tree** — a named mapping of blobs forming a configuration snapshot.
- **Commit** — an immutable node referencing a tree, parent commits, and metadata.
- **Ref** — a mutable pointer (branch or tag) to a commit.

The CAS lives in the `internal/promptsheon` package and is exposed to the HTTP server through a thin wrapper.

## Options considered

1. **CAS (chosen).** Git-style immutable DAG. Stable content hashes. Easy dedupe. Merkle-chain integrity.
2. **Relational versioning.** `version` column + `created_at`. Simple but no content integrity, no dedupe, brittle to schema migrations.
3. **Event-sourced log.** Append-only stream of edits. Replay to reconstruct history. Heavier than needed for a single-server deployment.
4. **Object storage (S3/GCS).** Durable but introduces an external dependency. Not appropriate for a single-binary service.

## Consequences

Positive:

- Dedupe is free. Identical prompts and tool specs share a hash.
- Tamper evidence. Any change to an object invalidates its hash and breaks the chain.
- The CLI can operate on a `.promptsheon` repository offline, just like Git.
- Composability. Agent configs, evaluation datasets, and tool specs are all addressed the same way.

Negative:

- Two storage layers: the CAS for versioned assets, the SQLite store for everything else. Developers must know which one to use.
- Garbage collection of unreachable objects is non-trivial. We currently rely on the retention sweep and on the fact that the only writer is the server.
- Object size is limited by the underlying filesystem.

## References

- `internal/promptsheon/doc.go` — full package documentation
- `internal/promptsheon/store.go` — blob and tree storage
- `internal/promptsheon/repo.go` — branches and refs
- `docs/architecture.md` — object model overview
