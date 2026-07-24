# Replay-set CRDT — design

Research note. The "replay" subsystem in `internal/replay/`
replays capability executions for forensic analysis. The
v0.2.0 release stops short of multi-region; the replay
output is a single SQLite table per replica. The v0.3.0+
multi-region work needs a CRDT for the replay set so that
two replicas can independently record replays and converge
to the same set.

This note describes the replay-set CRDT: an immutable
add-only set, the conflict semantics, and the retention
policy.

## 1. Shape

A replay-set is an **immutable add-only set**. Once an
entry is added, it cannot be modified or removed by the
CRDT itself. Deletion is a separate retention mechanism
(see §4).

The set's elements are opaque IDs: each execution that is
replayed is tagged with a UUID at the time of replay, and
the UUID is the set element. The element carries the
payload `(execution_id, payload, recorded_at, replica_id)`
where `replica_id` is the recording replica's identifier.

## 2. The CRDT

The replay-set is a 2P-Set (Two-Phase Set) with the
addition tombstone dropped. Concretely:

- A 2P-Set has two sets per replica: an "added" set and
  a "removed" set. The set membership is
  `added \setminus removed`.
- The replay-set drops the "removed" set. Deletion is
  impossible inside the CRDT.
- The set is therefore a G-Set (Grow-only Set) with
  elements that are `(timestamp, replica_id, payload)`.
- The merge is `union-by-payload-id`: a replica that
  receives a CRDT update from another replica adds any
  elements whose `payload_id` it does not already have.

The G-Set is the simplest CRDT in the literature and is
adequate because immutability is an invariant at the
application layer (the replay subsystem never modifies a
record once it is written).

## 3. Conflict semantics

The conflict semantics are "first-write-wins by
`payload_id`":

- Two replicas recording the same `execution_id`
  simultaneously produce two entries with distinct
  `payload_id`s (the UUID includes the recording
  replica's identifier). The merged set has both
  entries; the application layer decides which one is
  the canonical replay.
- Two replicas recording the same `execution_id` and
  the same `payload_id` (the same UUID) trivially
  collapse into a single entry on merge. The merge is
  idempotent.

There is no "last-write-wins" because the CRDT is
immutable. There is no "vote" because the CRDT is a
set, not a counter. The semantics are exactly the
semantics of a `git push` of unrelated objects: each
replica adds its own objects, and the repository is the
union.

## 4. Retention

Retention is **outside** the CRDT. The CRDT is
immutable; retention is a per-replica decision that
deletes entries from the per-replica store. The CRDT
merge does not undo a retention deletion: a replica
that has deleted an entry will simply not contribute
that entry to the merge.

The retention policy is:

- Entries older than the configured retention window
  (`PROMPTSHEON_REPLAY_RETENTION`) are eligible for
  deletion.
- The retention sweep runs once per hour and deletes
  at most N entries per run (N is configurable; the
  default is 10 000).
- Deletion is logged to the audit chain so a
  forensic analysis can still see "the entry was
  deleted at time T" even if the entry itself is gone.

The retention policy is intentionally simple. The CRDT
does not need a sliding-window merge because the
retention policy is a per-replica decision that the
merge does not have to honor.

## 5. Status

- **Shipped in v0.2.0**: this design doc. No code.
- **NOT shipped in v0.2.0**: the replay-set CRDT. The
  v0.2.0 release uses a single SQLite table per replica
  with no cross-replica merge.
- **Tracking**: the v0.3.0 multi-region work packages
  the G-Set implementation; the design above is the
  contract the implementation must satisfy.
- **Replacement criteria**: when the operator's DR
  strategy is "hot standby" and the standby needs to
  serve forensic queries, the CRDT is required. Until
  then, per-replica SQLite is sufficient.
