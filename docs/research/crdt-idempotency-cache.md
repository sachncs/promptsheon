# CRDT idempotency cache — design and the smallest transactional reservation path

Research note. Tracks the v0.2.0 work package: the idempotency
cache currently lives in `internal/api/idempotency.go` (a
SQLite-backed cache) and a synchronous path inside the
request handler. The cache is **not** a CRDT today; this
note explains when a CRDT would help, and how to ship the
smallest transactional reservation path that the post-CRDT
design can build on without requiring the CRDT to land.

## 1. The problem in one paragraph

A POST that accepts an `Idempotency-Key` header must return
the same response on every retry of the same key. The current
implementation caches `(scope_key → response_entry)` in
SQLite, with a 5-minute TTL. This works for a single-writer
deployment, but the moment v0.3.0 introduces multi-region
writes, two retries on different replicas write to two
different SQLite databases and one of them returns the
"first" response, the other the "second". The semantics are
not preserved across the boundary.

## 2. Why a CRDT

A CRDT is a data structure that converges to the same state
regardless of the order in which replicas receive updates.
The idempotency cache is a write-once key-value map: each
key is written exactly once (the first request that succeeds
within the TTL), and every later request reads the same
value. That fits the standard "PN-counter / MV-register"
CRDT family:

- a per-key MV-register. The first write wins; later
  writes are ignored. Reads converge across replicas
  because the dot-product of (timestamp, replica_id) is
  monotonic per key.
- a per-key LWW-element-set with a stable tiebreaker
  (replica_id). The TTL is a per-replica clock issue that
  the CRDT layer does not solve; it is solved by the
  resolution rule (clients ignore cached responses whose
  expiry has passed).

The implementation cost is bounded: the existing
`IdempotencyStore` interface (`Get`, `Put`) already gives
us a seam; adding a CRDT layer is a small adapter that
wraps the SQLite store and adds explicit convergence
semantics.

## 3. The smallest transactional reservation path

The CRDT is the right design for multi-region, but multi-
region is v0.3.0 work. We can ship the smallest part of the
design today without the CRDT:

**Reserve-then-commit.** The first request with a given
key writes a `_pending` row instead of a committed entry.
The reservation row carries the `Idempotency-Key`, the
scope (method + path + body-hash), and an expiry timestamp.
A second request with the same key that arrives while the
reservation is pending blocks (or returns 409) rather than
executing the handler again. When the first request's
handler completes, the reservation is promoted to a
committed entry and the response is written alongside.

This is the smallest transactional reservation path that
gives the right semantics **without** requiring a CRDT. It
is one extra SQLite table and one extra query in the hot
path. It does NOT solve the multi-region case (two
replicas can both reserve the same key at the same time),
but it gives the per-replica semantics the SDK already
documents, and it gives the CRDT layer a clean rollback
point if a future commit lands.

### Sketch

```sql
-- migration v0.2.0 (idempotency_reservation.up.sql)
CREATE TABLE idempotency_reservation (
    scope_key    TEXT PRIMARY KEY,
    created_at   TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);
CREATE INDEX idempotency_reservation_expires_at_idx
    ON idempotency_reservation(expires_at);
```

The handler flow becomes:

1. Scope key arrives via `Idempotency-Key` header.
2. `Lookup`: read committed entry first; on hit, replay.
3. `Reserve`: try `INSERT INTO idempotency_reservation
   (scope_key, expires_at) VALUES (?, ?)` with `expires_at
   = now + 5m`. If the INSERT succeeds, run the handler.
4. If the INSERT fails (PK collision), return 409 with the
   `Retry-After` header so the client backs off.
5. On handler completion, write the committed entry into
   the existing `idempotency` table and DELETE the
   reservation.

The retention sweep has a new responsibility: every minute,
drop reservation rows whose `expires_at < now`. The sweep
keeps the reservation table small and avoids the guest
"test never freed its reservation" footgun.

### Why this is the smallest credible path

- It does not require the CRDT to land.
- It does not require multi-region.
- It does not require a new dependency.
- It does not require a new schema migration beyond the
  single table above.
- It gives the SDK the semantics it already promises
  (`Retry-After` + 409) without making the CRDT a
  precondition.

## 4. When the CRDT becomes the right design

The CRDT work becomes priority when:

- two replicas both run the same `Reserve` on the same
  key.
- the cross-region round-trip is part of the SLO (the
  `p(99) < 3000 ms` budget on `/api/v1/...`).
- the operator's DR strategy is "hot standby" rather than
  "snapshot replicate" (the latter can stay SQLite + can
  tolerate per-replica semantics).

The v0.2.0 release does not enable any of those. The
smallest transactional reservation path above is the
v0.2.0 deliverable. The CRDT lands in v0.3.0 alongside
the multi-region work.

## 5. Status

- **Shipped in v0.2.0**: this design doc. No code.
- **NOT shipped in v0.2.0**: the SQLite reservation
  table, the handler changes, the retention sweep. These
  are v0.2.1 / v0.3.0 work, blocked on the design
  review above.
- **For a future PR**: the migration file, the
  `Reservation` method on `IdempotencyStore`, the handler
  integration, the test (`tests/idempotency_reservation`).
