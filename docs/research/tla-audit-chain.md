# TLA+ audit chain spec — research

The TLA+ spec at `tla/audit_chain.tla` is the formal
contract the runtime audit-chain check at
`internal/store/sqlite.go` is meant to satisfy. The
runtime check is the production guarantee; the spec is
the maintainer-facing proof that the contract has been
thought through.

## Why this lands in v0.2.0

The audit chain is the load-bearing invariant for
forensic analysis ("did anyone tamper with this log?").
The runtime check is exercised at every
`/api/v1/audit/verify` request and the
`audit_chain_verifications_total{result="fail"}` SLO
alert fires on a tamper. A TLA+ spec that mirrors the
ordering invariants is the contract the runtime check is
meant to satisfy; without it, the runtime check is only
as trustworthy as the reviewer's reading.

## What it covers

- a single-writer happy path
- two writers racing (the SQL serialisable transaction
  resolves the race; the second commit observes the
  first commit's `last_hash`)
- a reader walking the chain and reporting the first
  mismatch

## What it does not cover

- the SHA-256 hash function itself (we use a symbolic
  operator)
- the SQL transaction implementation
- crash recovery beyond log truncation
- multi-region replication (see
  `replay-set-crdt.md` for the forward-looking design)

## How it runs

```bash
tlc -config tla/audit_chain.cfg tla/audit_chain.tla
```

`tlc` is not a build-time dependency of promptsheon; the
spec is reviewed by hand when the audit chain
implementation changes. The CI pipeline does not run
`tlc` because the binary is not packaged on the
github-hosted runners — the maintainers run it locally
and the result is referenced in the PR.
