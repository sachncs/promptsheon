# Immutable agent specifications

`AgentSpecification` is the Phase 1 source of truth for an agent definition.
It is a strict, versioned Zod contract in
`packages/shared/src/agent-specification.ts` and contains role, objective,
prompt, context, guardrails, capabilities, permissions, model/routing policy,
execution, memory, evaluation, resource budget, and lifecycle policy.

## Content addressing

The content address is the lowercase hexadecimal SHA-256 digest of the UTF-8
canonical JSON representation. Canonicalization applies the schema defaults,
trims and NFC-normalizes strings, normalizes CRLF to LF, sorts object keys
recursively, preserves array order, and excludes runtime metadata such as
revision timestamps. The algorithm is implemented by
`canonicalizeSpecification()` and `hashAgentSpecification()`; consumers must
not hash an ad-hoc serialization.

The CAS stores the canonical bytes atomically: a unique temporary file is
written, flushed with `fsync`, and renamed into the sharded object path. Reads
recompute the SHA-256 digest and reject corruption. Identical content is stored
once even when concurrent callers create the same revision.

SQLite stores tenant-scoped revision metadata in `agent_specifications`:
workspace, parent hash, author, change reason, lifecycle status, and timestamps.
Publishing updates only that metadata row; the CAS content has no update path.
Use `create`, `validate`, `get`, `diff`, `lineage`, and `publish` through the
workspace-scoped API or the SDK/CLI workflows.

The repository keeps a bounded 1,024-entry parsed-value cache, but still reads
and verifies CAS bytes on every request so cache hits cannot hide disk
corruption.
