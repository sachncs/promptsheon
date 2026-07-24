// Package bandit — grow-only CRDT types for distributed arm
// counters. The persistence layer (`banditstore.Store`) keys
// each arm by replica id; counters on a single (arm, replica)
// pair are append-only on writes but converge under merge as
// the component-wise maximum.
//
// Merge is associative, commutative, idempotent, and monotonic
// over the lattice (N ∪ {∞})^2 — the standard grow-only counter
// CRDT. The test file `crdt_test.go` pins every property; new
// backends (Postgres, SQLite, in-memory) just need to be
// conflict-safe on merge.
//
// Effective arm counts across replicas: the load path on every
// backend Sums per-replica counters into one bucket per arm,
// so the selector sees the global observation total. The
// per-replica MergeCounters still picks the component-wise
// maximum — a single (arm, replica) row never regresses, and
// duplicate snapshots from the same replica are no-ops.
package bandit

// Counter is the per-(arm, replica) Beta(1,1) prior plus a
// tally of observed successes and failures. Both counts are
// grow-only — they never decrease — so the merge operator
// (component-wise max) is well-defined and joins the lattice
// (N ∪ {∞})^2. Effective posterior: alpha = 1+Successes,
// beta = 1+Failures.
type Counter struct {
	Successes uint64
	Failures  uint64
}

// MergeCounters returns the component-wise maximum of two
// counters. MergeCounters is the binary operation for the
// grow-only counter CRDT and is scoped to a single
// (arm, replica) row — the backend never merges two
// different replicas' counters through this function, it
// sums them on Load instead.
func MergeCounters(a, b Counter) Counter {
	if b.Successes > a.Successes {
		a.Successes = b.Successes
	}
	if b.Failures > a.Failures {
		a.Failures = b.Failures
	}
	return a
}

// State is a per-arm Counter map shared between replicas. The
// keys are arm IDs; the values are the merged counters across
// every replica that has contributed observations.
type State map[string]Counter

// MergeState merges two States arm-by-arm. An arm that exists
// in only one State is propagated unchanged; an arm that exists
// in both is reconciled by MergeCounters.
func MergeState(a, b State) State {
	if a == nil && b == nil {
		return State{}
	}
	if a == nil {
		return cloneState(b)
	}
	if b == nil {
		return cloneState(a)
	}
	out := make(State, len(a)+len(b))
	for id, c := range a {
		out[id] = c
	}
	for id, c := range b {
		out[id] = MergeCounters(out[id], c)
	}
	return out
}

// cloneState returns a deep copy of s so the merge result is
// decoupled from either operand. Without the copy a downstream
// mutator could clobber the operand.
func cloneState(s State) State {
	out := make(State, len(s))
	for id, c := range s {
		out[id] = c
	}
	return out
}

// Snapshot is one replica's full view of every (arm, counter)
// pair at a moment in time. It is the on-the-wire shape
// replicas exchange during convergence.
type Snapshot struct {
	ReplicaID string
	Counters  map[string]Counter
}

// ReplicaState preserves each replica's arm counters.
type ReplicaState map[string]State

// MergeSnapshots combines snapshots without collapsing replica partitions.
// Counters are max-merged only when both snapshots belong to the same replica.
func MergeSnapshots(a, b Snapshot) ReplicaState {
	out := ReplicaState{}
	if a.ReplicaID != "" {
		out[a.ReplicaID] = cloneState(a.Counters)
	}
	if b.ReplicaID != "" {
		out[b.ReplicaID] = MergeState(out[b.ReplicaID], b.Counters)
	}
	return out
}

// Aggregate sums distinct replica partitions into effective arm counters for
// posterior calculation.
func Aggregate(replicas ReplicaState) State {
	out := State{}
	for _, state := range replicas {
		for arm, counter := range state {
			current := out[arm]
			current.Successes += counter.Successes
			current.Failures += counter.Failures
			out[arm] = current
		}
	}
	return out
}
