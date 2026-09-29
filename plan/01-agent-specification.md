# Phase 1 — Immutable agent specifications

## Objective

Make the agent specification the durable, versioned, content-addressed source
of truth for every later execution and improvement decision.

## Responsible

- Domain/backend: schemas, invariants, canonicalization, use cases.
- Persistence/backend: CAS, SQLite repositories, migrations, indexes.
- Security: tenant isolation, sensitive-field policy, signing requirements.
- SDK/CLI: public creation and retrieval workflows.

## Implementation

- Define versioned shared schemas for:
  - role and objective,
  - prompt and context policy,
  - guardrails,
  - capabilities and tools,
  - permissions,
  - model and routing policies,
  - execution and memory policies,
  - evaluation policy,
  - resource budget,
  - lifecycle metadata.
- Separate mutable draft state from immutable published revisions.
- Canonicalize JSON deterministically:
  - stable key ordering,
  - normalized strings,
  - explicit defaults,
  - no runtime-only fields.
- Hash the canonical representation with a documented algorithm and schema
  version.
- Store content in CAS and metadata in SQLite with prepared statements.
- Store parent hash, author, change reason, created time, and schema version.
- Add create, validate, get, diff, list lineage, and publish use cases.
- Reject writes to immutable revisions.
- Add migration tooling for schema versions.
- Expose SDK and CLI commands for creating, inspecting, diffing, and validating
  specifications.

## Reliability and throughput

- Make hash generation pure and deterministic.
- Use atomic CAS writes: write, fsync where required, then publish metadata.
- Verify content hash on read for corruption detection.
- Add indexes for workspace, capability, hash, parent hash, and created time.
- Bound specification size and nesting depth.
- Cache immutable reads safely by hash.
- Keep canonicalization CPU-bounded and benchmark large specifications.
- Avoid duplicating specifications; identical content must deduplicate in CAS.

## Testing

- Schema unit tests for valid, invalid, boundary, and unknown-field cases.
- Golden tests for canonical JSON and hash stability.
- Property tests proving equivalent key order produces the same hash.
- Migration tests across every supported schema version.
- Repository tests for tenant scoping, uniqueness, immutability, and corruption.
- API contract tests for validation and error envelopes.
- Concurrency tests for duplicate writes and simultaneous publication.
- SDK and CLI integration tests against an in-process server.
- Benchmark canonicalization, hashing, CAS writes, and lineage queries.

## Deliverables

- Versioned `AgentSpecification` contract.
- CAS and metadata repositories.
- Lineage and diff APIs.
- Schema migration runner.
- SDK/CLI workflows.

## Exit criteria

- Every published agent has a stable content hash.
- A specification can be reconstructed exactly from its hash.
- No published specification can be modified in place.
- Lineage and diffs are queryable and tested under concurrent writes.

