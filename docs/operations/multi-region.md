# Multi-region

Promptsheon is **single-region by design** today. The
codebase intentionally does not support multi-region
replication, and this page explains why and what the path
forward looks like.

## Why single-region

The audit chain is a **hash chain** of every row, in order.
A multi-region deployment would require the chain to either:

- Replicate the entire chain synchronously across regions,
  which adds RTT to every audit write (each `audit()` call
  would block on a network round trip), OR
- Maintain per-region chains with periodic reconciliation,
  which breaks the chain invariant during the reconciliation
  window and means a verification call would race between
  regions.

Both options are bad. The synchronous variant makes the
audit worker the bottleneck; the async variant makes
`VerifyAuditChain` non-deterministic.

The second problem is the **SQLite single-writer** model.
SQLite serialises writes; multi-region requires either a
shared disk (latency-bound) or a different store. We picked
SQLite deliberately because it removes an entire class of
operational complexity (no Postgres cluster to operate, no
schema-drift, no DDL locks). Multi-region replication is
the natural follow-on to "replace SQLite with Postgres",
which itself is a follow-on to "decide whether the audit
chain is per-region or global".

## What multi-region WOULD look like (non-goal today)

A future multi-region design would look like:

- **Audit chain**: per-region chains, with a global
  Merkle-root checkpoint every N rows. `VerifyAuditChain`
  walks the per-region chain for the current region, then
  verifies the global root against a quorum of regional
  checkpoints.
- **CAS (`backend/cas/`)**: the CAS is content-addressed and
  immutable; multi-region replication is "just" object
  storage replication. S3-compatible cross-region replication
  works today with a single-line config change.
- **Capability/Version/Release**: these are CRDTs (no
  cross-region conflicts possible because the approval flow
  is sequential within a single Release). Per-region
  leader-election handles the active Release.
- **Observability**: per-region Prometheus + Thanos for
  cross-region aggregation. The SLO alert definitions in
  `deploy/prometheus/promptsheon-alerts.yaml` are regional
  today; the multi-region variant uses Thanos Ruler.

## What operators can do today

A single-region deployment can be made highly available
without multi-region replication:

- Run two replicas in the same region, with
  `PROMPTSHEON_LEADER_ELECTION=true`. The leader holds the
  SQLite write lock; the follower serves reads.
- Wire the Postgres backend (init + RLS bundles; the live
  pgx driver is a follow-on). The Postgres backend does not
  ship in this build. The audit chain becomes per-region;
  the global Merkle-root reconciliation is a follow-on.

For disaster recovery across regions, see
[docs/operations.md](operations.md) and
[docs/upgrade.md](upgrade.md). The cross-region DR story is
"hot standby in another region with hourly snapshot
replication", not "active-active multi-region writes".

## See also

- [docs/operations.md](operations.md) — backup / restore.
- [docs/upgrade.md](upgrade.md) — in-place upgrade +
  restore from snapshot.