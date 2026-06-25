# ADR 0003: Use a hash-chained audit log for tamper evidence

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

An audit log is not an audit log if it can be edited in place. For the compliance and security use cases we want to support ("who deployed this prompt?", "who changed the provider key?"), the log must be tamper-evident. A reviewer must be able to verify, on demand, that no entry was inserted, deleted, or modified after the fact.

Plain `INSERT` rows do not satisfy this. A row can be `UPDATE`d and the change will not show up in any later read.

## Decision

We extend the `audit_entries` table with two columns: `previous_hash` and `entry_hash`. For every new entry, we compute `entry_hash` as the SHA-256 of a canonical representation of the entry plus the `entry_hash` of the previous entry. The chain starts with an empty `previous_hash` (zero-length string).

A separate `audit_chain_state` table (migration `021`) holds the latest `entry_hash` so the chain can be verified in O(n) without re-scanning the whole table. The verifier endpoint `GET /api/v1/audit/verify` re-walks the chain and reports the first mismatched row, if any.

The chain is canonical. Every entry is serialised with stable JSON (no whitespace, struct fields in declaration order, sorted map keys) before hashing. The canonicalisation function is the only one that touches the chain, so we cannot accidentally hash a representation that differs from the on-disk row.

## Options considered

1. **Hash chain (chosen).** Tamper-evident, easy to verify, no external dependencies.
2. **Append-only file (e.g. write-once storage).** Durable but awkward to query from SQL.
3. **Merkle tree with periodic checkpoints.** More compact proofs, more complexity. Overkill for a single-server deployment.
4. **External log (e.g. AWS QLDB, immudb).** Strongest guarantees, strongest external dependency.

## Consequences

Positive:

- `GET /api/v1/audit/verify` returns the integrity status of the entire log in one call.
- The chain is verifiable by anyone with read access to the database. No server-side secret is required.
- Migrations `006`, `020`, and `021` together implement the chain without breaking older databases (the columns default to empty, and the chain is rebuilt lazily).

Negative:

- Performance. Each write is one extra hash and one extra read of the previous hash. Negligible for our write rates.
- The chain is linear. Concurrent writes are serialised at the entry-hash level. The mutex around chain extension is the hottest lock in the system; we batch writes when possible.
- A chain is only tamper-evident, not tamper-proof. An attacker with write access to the database can rewrite the entire chain. The threat model assumes the database itself is not compromised.

## References

- `internal/store/migrations/006_audit_chain.sql` — adds `previous_hash` and `entry_hash`
- `internal/store/migrations/020_audit_canonical_ts.sql` — canonical timestamp
- `internal/store/migrations/021_audit_chain_state.sql` — last-hash cache
- `internal/api/handlers_audit.go` — `verify` endpoint
- `docs/algorithms.md` — chain construction and verification
- `docs/security.md` — threat model
