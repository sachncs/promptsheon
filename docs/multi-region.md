# Multi-region

Promptsheon v0.1.x is **single-region by design**. The codebase
intentionally does not support multi-region replication, and
this page explains why and what the path forward looks like.

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
SQLite deliberately for v0.1.x because it removes an entire
class of operational complexity (no Postgres cluster to
operate, no schema-drift, no DDL locks). Multi-region
replication is the natural follow-on to "replace SQLite with
Postgres", which itself is a follow-on to "decide whether the
audit chain is per-region or global".

## What multi-region WOULD look like (non-goal for v0.1.x)

A future multi-region design is sketched in
`docs/adr/0019-deferred-items.md`. The shape:

- **Audit chain**: per-region chains, with a global
  Merkle-root checkpoint every N rows. `VerifyAuditChain`
  walks the per-region chain for the current region, then
  verifies the global root against a quorum of regional
  checkpoints.
- **CAS (`pkg/cas/`)**: the CAS is content-addressed and
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

## What v0.1.x operators can do today

A single-region deployment can be made highly available
without multi-region replication:

- Run two replicas in the same region, with
  `PROMPTSHEON_LEADER_ELECTION=true`. The leader holds the
  SQLite write lock; the follower serves reads.
- Use a regional PostgreSQL backend once a Postgres driver
  is added (see ADR-0015, currently Deprecated). The
  audit chain becomes per-region; the global Merkle-root
  reconciliation ships in v0.2.x.

For disaster recovery across regions, see
[docs/operations.md](operations.md) and
[docs/upgrade.md](upgrade.md). The cross-region DR story is
"hot standby in another region with hourly snapshot
replication", not "active-active multi-region writes".

## See also

- [ADR-0015 (Deprecated)](adr/0015-postgres-backend-with-rls.md) —
  the Postgres backend that would enable multi-region.
- [ADR-0019](adr/0019-deferred-items.md) — deferred items
  including the multi-region sketch.
- [docs/operations.md](operations.md) — backup / restore.
- [docs/upgrade.md](upgrade.md) — in-place upgrade +
  restore from snapshot.