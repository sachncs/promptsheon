# TLA+ specifications

The audit chain is a load-bearing invariant: every audit
entry records the hash of the entry that came before it, so
a tamper with any past entry is detectable by re-walking the
chain. The runtime check is in `backend/store/sqlite.go`'s
`VerifyAuditChainOnDB`; a tamper increments
`audit_chain_verifications_total{result="fail"}` so the
SLO alert fires.

The TLA+ spec here is a small, parallel model of the same
ordering invariant. It is **not** a substitute for the
runtime check; it is a contract that the runtime check is
meant to satisfy. Two writers racing on the SQL transaction
is the interesting case: the spec models the
"commit-wins-second" property that the serialisable
transaction is intended to provide.

## Files

- `audit_chain.tla` — the spec.
- `audit_chain.cfg` — TLC config (model-checking parameters).

## Running

The spec is intended for `tlc` (the TLA+ model checker).
Install it from <https://github.com/tlaplus/tlaplus>.

```bash
tlc -config tla/audit_chain.cfg tla/audit_chain.tla
```

`tlc` is not a build-time dependency of promptsheon; the
spec is checked by reviewers manually when the audit chain
implementation changes. The CI pipeline does not run `tlc`
because the binary is not packaged on the github-hosted
runners — the maintainers run it locally and the result is
referenced in the PR.

## What the spec covers

- a single-writer happy path
- two writers racing (the SQL serialisable transaction
  resolves the race; the second commit observes the first
  commit's `last_hash`)
- a reader walking the chain and reporting the first
  mismatch

## What the spec deliberately does not cover

- the SHA-256 hash function itself (we use a symbolic
  operator)
- the SQL transaction implementation
- crash recovery beyond log truncation
- multi-region replication (RES-CRDT-2 documents the
  per-region chain + global Merkle-root checkpoint design)

## Invariants checked

- `TypeOK` — every state variable is well-typed.
- `ConsistentTail` — `last_hash` always equals the hash of the
  row at `last_row` (or the empty-chain sentinel when
  `last_row = 0`).
- `ChainIsLinked` — `chain[i].prev = chain[i-1].hash` for
  every `i` in `[2..last_row]`.
- `VerificationImpliesLinked` — if the reader reports `ok`,
  the chain is necessarily linked.

## Release lifecycle spec

`release_lifecycle.tla` models the Release state machine
(`pending → approved → active → superseded | rolled_back`) the
runtime Activate path in `backend/release/service.go`
enforces. It pins:

- `ActiveExclusive` — at most `MaxReleases` active Releases
  per Environment.
- `NoActiveToSupersede` — a superseded/rolled-back Release
  must not appear in any active set.
- `StatusConsistency` — `status = active` iff the Release is
  in some Environment's active set.
- `VoteImpliesPendingOrApproved` — a Release with recorded
  Approve votes must be at least in pending state.
- The Maker/Checker separation-of-duties rule (the
  Release's creator cannot be the approver).

`release_lifecycle.cfg` configures TLC with three Releases,
two Environments, and `MaxReleases = 2`. Run:

```bash
tlc -config tla/release_lifecycle.cfg tla/release_lifecycle.tla
```

A Go-side regression test (`tla/release_lifecycle_test.go`)
asserts the spec and config files exist with the required
content so the file structure is preserved across edits.

