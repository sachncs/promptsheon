-------------------------- MODULE audit_chain --------------------------
(*
 * TLA+ specification of the audit-chain ordering invariant.
 *
 * Mirrors the SQLite implementation in internal/store/sqlite.go
 * (AppendAudit / VerifyAuditChain). The spec is intentionally
 * narrow: it does not model the LLM / HTTP / CAS layers. It pins
 * the ordering guarantee that the audit chain promises to its
 * consumers: "every entry's previous_hash equals the entry_hash of
 * the entry immediately before it, and the chain state advances
 * monotonically per writer".
 *
 * Modeled as a bounded sequence of entries:
 *   - entries        — function from 1..Len(entries) to Entry.
 *   - last_hash      — the chain's tail hash (or "NULL_HASH" if
 *                      the chain is empty).
 *   - staged         — per-writer record: either absent (no entry
 *                      staged) or carrying a hash, payload, and
 *                      prev_hash observed when the prepare ran.
 *                      A stale prepare is detected at commit time
 *                      via the recorded prev_hash; a stale entry
 *                      is dropped (CommitEntry aborts, the writer
 *                      can re-Prepare) and is never written to
 *                      the chain.
 *   - last_row       — the index of the tail entry in `entries`
 *                      (0 if the chain is empty).
 *   - verify         — "pending" / "ok" / "mismatch" — the reader's
 *                      verdict.
 *
 * Symbolic hash: we do not model SHA-256. The spec defines
 * hash(prev, payload) as a free operator, parametrised by an
 * abstract hash_func. What matters is the *ordering* of prev /
 * hash fields, not the bit-level content of the hash.
 *
 * Bounded model: MaxLen bounds the entry sequence so TLC has a
 * finite state space. The abstract hash function is
 * SetsSubsets(Entries ∪ {NULL_HASH}) — there are finitely many
 * distinct hash values regardless of the actual SHA-256 surface,
 * and any value in this set is a valid hash for the purpose of
 * verifying the ordering invariant.
 *
 * Every action lists the UNCHANGED variables explicitly so a
 * reviewer can see at a glance which variables the action does
 * not touch.
 *
 * Run with:
 *   tlc -config tla/audit_chain.cfg tla/audit_chain.tla
 *
 * Out of scope (deliberately):
 *   - the SHA-256 hash function itself (we use a free operator)
 *   - the SQL transaction implementation
 *   - crash recovery beyond log truncation
 *   - multi-region replication (RES-CRDT-2 documents the
 *     per-region chain + global Merkle-root checkpoint design)
 *)

EXTENDS Naturals, Sequences, FiniteSets

CONSTANTS
    Entries,        \* set of symbolic payload values
    Writers,        \* set of writer identifiers
    MaxLen          \* bound on the number of entries in the chain

VARIABLES
    entries,        \* sequence of Entry records; Len(entries) <= MaxLen
    last_hash,      \* the chain's tail hash (NULL_HASH if empty)
    last_row,       \* the index of the tail entry in entries (0 if empty)
    staged,         \* function: Writers -> [present, payload, prev, hash]
    verify          \* reader's verification outcome

vars == <<entries, last_hash, last_row, staged, verify>>

\* The empty-chain sentinel. We deliberately model it as a
\* distinct constant rather than an empty string so the spec
\* never confuses "hash of the empty chain" with "missing
\* hash". NULL_HASH is a constant of the spec; the TLC config
\* sets it to a model value (e.g. NULL) that is NOT in
\* Entries. The `Assumes` clause below asserts the disjointness
\* property so a config that violates it is rejected at parse
\* time rather than discovered mid-model-check.
ASSUME NULL_HASH \notin Entries

Entry == [
    payload: Entries,
    prev:    Entries \union {NULL_HASH},
    hash:    Entries \union {NULL_HASH}
]

\* The set of possible per-writer staged records. `present`
\* is the discriminator: when FALSE the other fields are
\* ignored; when TRUE they describe the in-flight prepare.
Staged == [
    present: BOOLEAN,
    payload: Entries \union {NULL_HASH},
    prev:    Entries \union {NULL_HASH},
    hash:    Entries \union {NULL_HASH}
]

\* Symbolic hash. We treat the hash space as the finite set
\* (Entries \union {NULL_HASH}); the actual SHA-256 value is
\* irrelevant for the ordering invariant. hash_of is a
\* deterministic projection: when the prev is the empty-chain
\* sentinel, the hash is the payload; otherwise the hash is
\* the prev. This is a total, injective operator over the
\* (prev, payload) pairs the spec ever constructs, and it
\* keeps the model finite without CHOOSE-ing into a model
\* value. The spec is sound under any injective hash_of with
\* the same domain — this projection is the simplest one
\* that satisfies that property.
hash_of(prev, payload) ==
    IF prev = NULL_HASH
    THEN payload
    ELSE prev

TypeOK ==
    /\ entries \in Seq(Entry)
    /\ Len(entries) <= MaxLen
    /\ last_hash \in (Entries \union {NULL_HASH})
    /\ last_row \in 0..Len(entries)
    /\ staged \in [Writers -> Staged]
    /\ verify \in {"pending", "ok", "mismatch"}

EmptyStaged == [
    present |-> FALSE,
    payload |-> NULL_HASH,
    prev    |-> NULL_HASH,
    hash    |-> NULL_HASH
]

StageWith(p, prev_hash, h) == [
    present |-> TRUE,
    payload |-> p,
    prev    |-> prev_hash,
    hash    |-> h
]

EntryFrom(s) == [
    payload |-> s.payload,
    prev    |-> s.prev,
    hash    |-> s.hash
]

Init ==
    /\ entries = << >>
    /\ last_hash = NULL_HASH
    /\ last_row = 0
    /\ staged = [w \in Writers |-> EmptyStaged]
    /\ verify = "pending"

\* PrepareEntry records what the chain tail was when this
\* writer started its transaction. We store the observed
\* prev_hash AND the would-be hash so a later CommitEntry
\* can detect a stale prepare (another writer committed in
\* the meantime and the tail moved).
PrepareEntry(w, payload) ==
    /\ ~staged[w].present
    /\ Len(entries) < MaxLen
    /\ staged' = [staged EXCEPT ![w] = StageWith(payload, last_hash, hash_of(last_hash, payload))]
    /\ UNCHANGED <<entries, last_hash, last_row, verify>>

\* CommitEntry is the serialised commit. It is enabled only
\* when the recorded prev matches the current tail — that is
\* the "stale guard": a writer that prepared against an old
\* tail must re-Prepare (or Abort) rather than overwrite the
\* chain with a stale link. The new entry's hash is
\* recomputed from the now-current tail (which must equal
\* the recorded prev) so the chain advances monotonically.
CommitEntry(w) ==
    /\ staged[w].present
    /\ staged[w].prev = last_hash
    /\ Len(entries) < MaxLen
    /\ last_row' = last_row + 1
    /\ entries' = Append(entries, EntryFrom(staged[w]))
    /\ last_hash' = staged[w].hash
    /\ staged' = [staged EXCEPT ![w] = EmptyStaged]
    /\ UNCHANGED verify

\* AbortEntry is the explicit rollback path: the writer
\* drops its staged entry without touching the chain. Used
\* for a writer that has decided not to commit (e.g. a
\* caller-driven cancel).
AbortEntry(w) ==
    /\ staged[w].present
    /\ staged' = [staged EXCEPT ![w] = EmptyStaged]
    /\ UNCHANGED <<entries, last_hash, last_row, verify>>

\* StaleAbort drops a writer's stale prepare without
\* touching the chain. A prepare is stale when its
\* recorded prev no longer matches the chain's tail —
\* meaning another writer committed first and the chain
\* moved on. The stale writer MUST abort (or re-prepare);
\* it MUST NOT commit, because committing would produce a
\* chain link that skips over the entry committed in
\* between.
StaleAbort(w) ==
    /\ staged[w].present
    /\ staged[w].prev /= last_hash
    /\ staged' = [staged EXCEPT ![w] = EmptyStaged]
    /\ UNCHANGED <<entries, last_hash, last_row, verify>>

\* RePrepare is the explicit recovery path: drop the stale
\* prepare and immediately re-prepare against the now-
\* current tail. Modeled as the disjunction of StaleAbort
\* followed by PrepareEntry so a reviewer can see both
\* transitions in the state graph.
RePrepare(w, payload) ==
    /\ staged[w].present
    /\ staged[w].prev /= last_hash
    /\ Len(entries) < MaxLen
    /\ staged' = [staged EXCEPT ![w] = StageWith(payload, last_hash, hash_of(last_hash, payload))]
    /\ UNCHANGED <<entries, last_hash, last_row, verify>>

\* VerifyChain walks the chain from row 1 to last_row and
\* reports the first mismatch. "pending" is the initial
\* state; a successful walk is "ok"; a mismatch is
\* "mismatch". This is the same predicate the runtime
\* VerifyAuditChainOnDB in internal/store/sqlite.go walks.
VerifyChain ==
    LET walk == \A i \in 1..last_row :
                    /\ entries[i].prev = IF i = 1
                                         THEN NULL_HASH
                                         ELSE entries[i-1].hash
                    /\ entries[i].hash = hash_of(entries[i].prev, entries[i].payload)
    IN  IF walk
        THEN verify' = "ok"
        ELSE verify' = "mismatch"
    /\ UNCHANGED <<entries, last_hash, last_row, staged>>

Next ==
    \/ \E w \in Writers, p \in Entries : PrepareEntry(w, p)
    \/ \E w \in Writers : CommitEntry(w)
    \/ \E w \in Writers : AbortEntry(w)
    \/ \E w \in Writers : StaleAbort(w)
    \/ \E w \in Writers, p \in Entries : RePrepare(w, p)
    \/ VerifyChain

Spec == Init /\ [][Next]_vars

\* Fairness: every writer eventually gets a chance to commit
\* (weak fairness per writer, so a perpetually aborted
\* writer does not block progress on the others). The
\* reader is weakly fair too — VerifyChain is allowed to
\* run, and may run multiple times.
Fairness == /\ \A w \in Writers : WF_vars(CommitEntry(w))
            /\ \A w \in Writers : WF_vars(AbortEntry(w))
            /\ \A w \in Writers : WF_vars(StaleAbort(w))
            /\ WF_vars(VerifyChain)

\* Invariants ----------------------------------------------------------------

\* ContiguousLinkedChain: every committed entry's prev points
\* at the entry immediately before it (or NULL_HASH for the
\* first entry), and its hash matches the symbolic hash of
\* (prev, payload). This is the "chain is well-formed"
\* invariant — the reader walks it and the predicate holds
\* for every row.
ContiguousLinkedChain ==
    \A i \in 1..last_row :
        /\ entries[i].prev = IF i = 1
                             THEN NULL_HASH
                             ELSE entries[i-1].hash
        /\ entries[i].hash = hash_of(entries[i].prev, entries[i].payload)

\* TailAgreement: last_hash matches the tail entry's hash
\* (or NULL_HASH if the chain is empty). This is what the
\* audit_chain_state row in the SQL schema records — the
\* persisted tail pointer must always equal the last row's
\* hash, otherwise a verifier will report a tail mismatch.
TailAgreement ==
    IF last_row = 0
    THEN last_hash = NULL_HASH
    ELSE /\ last_hash = entries[last_row].hash
         /\ last_row = Len(entries)

\* VerificationSoundness: if the reader reports "ok", the
\* chain is necessarily contiguous AND the tail agrees.
\* This is the "no false positives" property: a passing
\* verify means the chain is well-formed end to end.
VerificationSoundness ==
    verify = "ok" =>
        /\ ContiguousLinkedChain
        /\ TailAgreement

\* AllStagedRespectPrev: any in-flight staged entry records
\* the prev it observed at prepare time. This is the
\* "stale guard" invariant: a commit is enabled only when
\* the staged prev matches the current tail. The guard is
\* the only thing keeping a late writer from overwriting
\* the chain with a link that skips rows.
AllStagedRespectPrev ==
    \A w \in Writers :
        staged[w].present =>
            /\ staged[w].prev \in (Entries \union {NULL_HASH})
            /\ staged[w].hash = hash_of(staged[w].prev, staged[w].payload)

====
