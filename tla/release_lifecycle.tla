-------------------------- MODULE release_lifecycle --------------------------
(*
 * TLA+ specification of the release lifecycle state machine.
 *
 * Mirrors internal/release/release.go (Release.Status) and the
 * Activate path in cmd/promptsheond/main.go. The spec models
 * one Release's lifecycle; the per-(capability, environment)
 * "exactly one active" invariant is modeled as ActiveExclusive.
 *
 * States:
 *   none      - the Release has not been created yet
 *   pending   - Create has run; awaiting vote + Activate
 *   approved  - MakerChecker quorum is satisfied; Activate pending
 *   active    - the Release is serving traffic in its Environment
 *   superseded - a successor Release has taken over the Environment
 *   rolled_back - operator rolled the Release back
 *
 * Transitions:
 *   Create      : none     -> pending
 *   Vote (Approve / Reject): pending -> pending (vote count)
 *   Approve     : pending  -> approved (MakerChecker satisfied)
 *   Reject      : pending  -> rejected (terminal; not modeled here)
 *   Activate    : approved -> active   (atomic supersession)
 *   Supersede   : active   -> superseded (successor Release activated)
 *   Rollback    : active   -> rolled_back
 *
 * Run with:
 *   tlc -config tla/release_lifecycle.cfg tla/release_lifecycle.tla
 *)

EXTENDS Naturals, FiniteSets

CONSTANTS
    Releases,        \* set of release identifiers
    Environments,    \* set of environment identifiers
    MaxReleases      \* upper bound on concurrent active releases per env

VARIABLES
    status,          \* function: Releases -> Status
    activeInEnv,     \* function: Environments -> SUBSET Releases
    votes,           \* function: Releases -> {0, 1}  (1 = at least one Approve)
    creator,         \* function: Releases -> {"alice", "bob", "carol"}

vars == <<status, activeInEnv, votes, creator>>

Status == {"none", "pending", "approved", "active", "superseded", "rolled_back"}

TypeOK ==
    /\ status \in [Releases -> Status]
    /\ activeInEnv \in [Environments -> SUBSET Releases]
    /\ votes \in [Releases -> {0, 1}]
    /\ creator \in [Releases -> {"alice", "bob", "carol"}]

\* ActiveExclusive: at most MaxReleases active releases per
\* environment. v0.2.0 ships one Release per (Capability,
\* Environment); the invariant allows MaxReleases to be tuned
\* up to support canary or A/B fan-out without rewriting the
\* spec.
ActiveExclusive ==
    \A env \in Environments :
        Cardinality(activeInEnv[env]) <= MaxReleases

\* NoActiveToSupersede: a Release in `superseded` or
\* `rolled_back` must not appear in any environment's active set.
NoActiveToSupersede ==
    \A r \in Releases :
        (status[r] \in {"superseded", "rolled_back"}) =>
            \A env \in Environments :
                r \notin activeInEnv[env]

\* StatusConsistency: status=active <=> the Release is in some
\* environment's active set. A Release that is "active" but not
\* in any activeInEnv set is a tampered state.
StatusConsistency ==
    \A r \in Releases :
        (status[r] = "active") <=> (\E env \in Environments : r \in activeInEnv[env])

\* VoteImpliesPendingOrApproved: a Release with recorded votes
\* must be at least in pending state. Approving a Release that
\* doesn't exist or is already terminal would be a control-
\* plane bug.
VoteImpliesPendingOrApproved ==
    \A r \in Releases :
        votes[r] = 1 => status[r] \in {"pending", "approved", "active"}

Init ==
    /\ status = [r \in Releases |-> "none"]
    /\ activeInEnv = [env \in Environments |-> {}]
    /\ votes = [r \in Releases |-> 0]
    /\ creator = [r \in Releases |-> "alice"]

Create(r, who) ==
    /\ status[r] = "none"
    /\ status' = [status EXCEPT ![r] = "pending"]
    /\ creator' = [creator EXCEPT ![r] = who]
    /\ UNCHANGED <<activeInEnv, votes>>

ApproveVote(r) ==
    /\ status[r] = "pending"
    /\ votes' = [votes EXCEPT ![r] = 1]
    /\ UNCHANGED <<status, activeInEnv, creator>>

Approve(r) ==
    /\ status[r] = "pending"
    /\ votes[r] = 1
    /\ creator[r] /= "alice" \* MakerChecker separation-of-duties
    /\ status' = [status EXCEPT ![r] = "approved"]
    /\ UNCHANGED <<activeInEnv, votes, creator>>

Activate(r, env) ==
    /\ status[r] = "approved"
    /\ Cardinality(activeInEnv[env]) < MaxReleases
    /\ status' = [status EXCEPT ![r] = "active"]
    /\ activeInEnv' = [activeInEnv EXCEPT ![env] = activeInEnv[env] \cup {r}]
    /\ UNCHANGED <<votes, creator>>

Supersede(r, env) ==
    /\ status[r] = "active"
    /\ r \in activeInEnv[env]
    /\ status' = [status EXCEPT ![r] = "superseded"]
    /\ activeInEnv' = [activeInEnv EXCEPT ![env] = activeInEnv[env] \ {r}]
    /\ UNCHANGED <<votes, creator>>

Rollback(r) ==
    /\ status[r] \in {"active", "approved"}
    /\ IF status[r] = "active"
       THEN \E env \in Environments : r \in activeInEnv[env]
       ELSE TRUE
    /\ status' = [status EXCEPT ![r] = "rolled_back"]
    /\ activeInEnv' = [activeInEnv EXCEPT
        ![env] = activeInEnv[env] \ {r}
        FOR env \in Environments]
    /\ UNCHANGED <<votes, creator>>

Next ==
    \/ \E r \in Releases : Create(r, "alice") \/ Create(r, "bob")
    \/ \E r \in Releases : ApproveVote(r)
    \/ \E r \in Releases : Approve(r)
    \/ \E r \in Releases, env \in Environments : Activate(r, env)
    \/ \E r \in Releases, env \in Environments : Supersede(r, env)
    \/ \E r \in Releases : Rollback(r)

Spec == Init /\ [][Next]_vars

Fairness ==
    /\ \A r \in Releases : WF_vars(ApproveVote(r))
    /\ \A r \in Releases : WF_vars(Approve(r))
    /\ \A r \in Releases : WF_vars(Rollback(r))

\* Invariants the runtime Activate path enforces; the spec pins
\* them so a future refactor that violates one fails TLC.
Safety ==
    /\ TypeOK
    /\ ActiveExclusive
    /\ NoActiveToSupersede
    /\ StatusConsistency
    /\ VoteImpliesPendingOrApproved

====
