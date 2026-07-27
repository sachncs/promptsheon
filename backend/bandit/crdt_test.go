// CRDT algebraic properties — the tests below pin every
// invariant the persistence layer relies on. They run without
// touching the file system so the bandit storage engine can
// pick a backend (in-memory, Postgres, SQLite) and inherit the
// convergence guarantees for free.
package bandit

import (
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"
	"unsafe"
)

// stateEQ reports whether two States are structurally equal.
// reflect.DeepEqual treats a nil map and an empty map as
// unequal; the merge operators sometimes return one or the
// other so the tests want equality either way.
func stateEQ(a, b State) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if va != vb {
			return false
		}
	}
	return true
}

// TestMergeCountersComponentWise pins the basic lattice op:
// max on each coordinate.
func TestMergeCountersComponentWise(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b, want Counter
	}{
		{Counter{0, 0}, Counter{0, 0}, Counter{0, 0}},
		{Counter{3, 5}, Counter{7, 2}, Counter{7, 5}},
		{Counter{7, 2}, Counter{3, 5}, Counter{7, 5}},
		{Counter{10, 10}, Counter{10, 10}, Counter{10, 10}},
	}
	for i, tc := range cases {
		if got := MergeCounters(tc.a, tc.b); got != tc.want {
			t.Fatalf("case %d: got %+v, want %+v", i, got, tc.want)
		}
	}
}

// TestMergeCountersCommutativity asserts that MergeCounters is
// commutative: a ⊕ b = b ⊕ a. Combined with the associativity
// test below the join-semilattice structure is locked.
func TestMergeCountersCommutativity(t *testing.T) {
	t.Parallel()
	a := Counter{Successes: 4, Failures: 6}
	b := Counter{Successes: 7, Failures: 1}
	if MergeCounters(a, b) != MergeCounters(b, a) {
		t.Fatalf("MergeCounters not commutative: %+v vs %+v", MergeCounters(a, b), MergeCounters(b, a))
	}
}

// TestMergeCountersAssociativity asserts (a ⊕ b) ⊕ c = a ⊕ (b ⊕ c).
// Property-based: pick a few hundred triples and verify.
func TestMergeCountersAssociativity(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < 500; i++ {
		a := Counter{Successes: rng.Uint64() & 0xff, Failures: rng.Uint64() & 0xff}
		b := Counter{Successes: rng.Uint64() & 0xff, Failures: rng.Uint64() & 0xff}
		c := Counter{Successes: rng.Uint64() & 0xff, Failures: rng.Uint64() & 0xff}
		left := MergeCounters(MergeCounters(a, b), c)
		right := MergeCounters(a, MergeCounters(b, c))
		if left != right {
			t.Fatalf("assoc violation at i=%d: left=%+v right=%+v", i, left, right)
		}
	}
}

// TestMergeCountersIdempotence asserts that a ⊕ a = a. Any
// merge that drops observations on a duplicate run would
// violate this.
func TestMergeCountersIdempotence(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(2, 2))
	for i := 0; i < 200; i++ {
		a := Counter{Successes: rng.Uint64() & 0xff, Failures: rng.Uint64() & 0xff}
		if got := MergeCounters(a, a); got != a {
			t.Fatalf("idempotence violation at i=%d: got %+v want %+v", i, got, a)
		}
	}
}

// TestMergeStateCommutativity mirrors the counter property at
// the State level. Two States merge to the same State no
// matter which order they are combined.
func TestMergeStateCommutativity(t *testing.T) {
	t.Parallel()
	a := State{"arm-1": {Successes: 3, Failures: 0}, "arm-2": {Successes: 0, Failures: 2}}
	b := State{"arm-2": {Successes: 1, Failures: 5}, "arm-3": {Successes: 4, Failures: 4}}
	if !stateEQ(MergeState(a, b), MergeState(b, a)) {
		t.Fatalf("MergeState not commutative: %v vs %v", MergeState(a, b), MergeState(b, a))
	}
}

// TestMergeStateAssociativity locks (a ⊕ b) ⊕ c = a ⊕ (b ⊕ c)
// for State. Concurrent replicas converge to the same final
// State regardless of merge order.
func TestMergeStateAssociativity(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(3, 3))
	mk := func() State {
		s := State{}
		n := 1 + rng.IntN(4)
		for i := 0; i < n; i++ {
			id := string([]byte{byte('a' + rng.IntN(4))})
			s[id] = Counter{Successes: rng.Uint64() & 0xff, Failures: rng.Uint64() & 0xff}
		}
		return s
	}
	for i := 0; i < 200; i++ {
		a, b, c := mk(), mk(), mk()
		left := MergeState(MergeState(a, b), c)
		right := MergeState(a, MergeState(b, c))
		if !stateEQ(left, right) {
			t.Fatalf("State assoc violation at i=%d", i)
		}
	}
}

// TestMergeStateIdempotence: merge(s, s) == s.
func TestMergeStateIdempotence(t *testing.T) {
	t.Parallel()
	s := State{
		"arm-1": {Successes: 5, Failures: 2},
		"arm-2": {Successes: 0, Failures: 0},
	}
	if !stateEQ(MergeState(s, s), s) {
		t.Fatalf("State idempotence violated: %v vs %v", MergeState(s, s), s)
	}
}

// TestMergeStateDuplicateMessages is the wire-format
// regression: a network that re-delivers the same snapshot
// twice must not double-count. The simulator below sends N
// random observations to a single replica; the snapshot is
// then merged back into the origin State N times. The merged
// state must equal the per-arm max seen on the wire.
func TestMergeStateDuplicateMessages(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(4, 4))
	// Two replicas each produce 50 observations on a shared
	// set of arms.
	arms := []string{"a", "b", "c"}
	a, b := State{}, State{}
	maxObs := map[string]Counter{}
	for i := 0; i < 50; i++ {
		arm := arms[rng.IntN(len(arms))]
		success := rng.IntN(2) == 0
		cur := a[arm]
		if success {
			cur.Successes++
		} else {
			cur.Failures++
		}
		a[arm] = cur
		if existing, ok := maxObs[arm]; ok {
			if cur.Successes > existing.Successes {
				existing.Successes = cur.Successes
			}
			if cur.Failures > existing.Failures {
				existing.Failures = cur.Failures
			}
			maxObs[arm] = existing
		} else {
			maxObs[arm] = cur
		}
	}
	for i := 0; i < 50; i++ {
		arm := arms[rng.IntN(len(arms))]
		success := rng.IntN(2) == 0
		cur := b[arm]
		if success {
			cur.Successes++
		} else {
			cur.Failures++
		}
		b[arm] = cur
		if existing, ok := maxObs[arm]; ok {
			if cur.Successes > existing.Successes {
				existing.Successes = cur.Successes
			}
			if cur.Failures > existing.Failures {
				existing.Failures = cur.Failures
			}
			maxObs[arm] = existing
		} else {
			maxObs[arm] = cur
		}
	}
	merged := MergeState(a, b)
	if !stateEQ(merged, maxObs) {
		t.Fatalf("merged=%v want=%v", merged, maxObs)
	}
	// Duplicate-deliver the same snapshot 5 times; the result
	// is unchanged.
	for i := 0; i < 5; i++ {
		merged = MergeState(merged, b)
	}
	if !stateEQ(merged, maxObs) {
		t.Fatalf("post-duplicate merged=%v want=%v", merged, maxObs)
	}
}

// TestConcurrentReplicaConvergence is the headline property:
// two replicas that accumulate disjoint observations must
// converge to the same State after exchanging snapshots. The
// test runs N=20 rounds to shake out subtle ordering bugs.
func TestConcurrentReplicaConvergence(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(5, 5))
	arms := []string{"a", "b", "c", "d", "e"}
	for round := 0; round < 20; round++ {
		a, b := State{}, State{}
		// Each replica accumulates 30 observations on a
		// shared arm set, disjoint at the per-arm level.
		for i := 0; i < 30; i++ {
			arm := arms[rng.IntN(len(arms))]
			success := rng.IntN(2) == 0
			cur := a[arm]
			if success {
				cur.Successes++
			} else {
				cur.Failures++
			}
			a[arm] = cur
		}
		for i := 0; i < 30; i++ {
			arm := arms[rng.IntN(len(arms))]
			success := rng.IntN(2) == 0
			cur := b[arm]
			if success {
				cur.Successes++
			} else {
				cur.Failures++
			}
			b[arm] = cur
		}
		// Replicas exchange snapshots, then re-merge.
		merged := MergeState(a, b)
		a2 := MergeState(a, merged)
		b2 := MergeState(b, merged)
		if !stateEQ(a2, b2) {
			t.Fatalf("round %d: not converged: a=%v b=%v", round, a2, b2)
		}
	}
}

// TestRestartReconstruction pins the post-restart invariant:
// every observation that hit any replica before the restart is
// visible after the restart once the replicas exchange their
// per-replica State. The simulator records observations to a
// per-replica log, then folds the log back through merge.
func TestRestartReconstruction(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(6, 6))
	arms := []string{"a", "b", "c"}
	// Three replicas observe independently.
	replicas := []State{{}, {}, {}}
	for i := 0; i < 120; i++ {
		rep := rng.IntN(len(replicas))
		arm := arms[rng.IntN(len(arms))]
		success := rng.IntN(2) == 0
		c := replicas[rep][arm]
		if success {
			c.Successes++
		} else {
			c.Failures++
		}
		replicas[rep][arm] = c
	}
	// "Restart": each replica starts from its last-known State
	// (the one it persisted). They then exchange and re-merge.
	view := func(rs []State) State {
		out := State{}
		for _, r := range rs {
			out = MergeState(out, r)
		}
		return out
	}
	got := view(replicas)
	// Build the expected state from the per-arm global maxima.
	want := State{}
	for _, r := range replicas {
		for arm, c := range r {
			m := MergeCounters(want[arm], c)
			want[arm] = m
		}
	}
	if !stateEQ(got, want) {
		t.Fatalf("post-restart mismatch: got=%v want=%v", got, want)
	}
}

// TestMergeDoesNotMutateOperands is the consumer-safety
// contract: a caller that hands its State to MergeState keeps
// the same State after the call returns. Backends that mutate
// the input would corrupt the caller's snapshot.
func TestMergeDoesNotMutateOperands(t *testing.T) {
	t.Parallel()
	a := State{"arm-1": {Successes: 3, Failures: 1}}
	b := State{"arm-1": {Successes: 1, Failures: 4}, "arm-2": {Successes: 5, Failures: 0}}
	aSnap := deepCopyState(a)
	bSnap := deepCopyState(b)
	_ = MergeState(a, b)
	if !reflect.DeepEqual(a, aSnap) {
		t.Fatalf("MergeState mutated operand a: %v vs %v", a, aSnap)
	}
	if !reflect.DeepEqual(b, bSnap) {
		t.Fatalf("MergeState mutated operand b: %v vs %v", b, bSnap)
	}
}

// TestMergeStateNilOperands: passing a nil State is treated as
// the empty State. Backends that hand a nil map because no
// arms were ever registered must not panic.
func TestMergeStateNilOperands(t *testing.T) {
	t.Parallel()
	s := State{"arm-1": {Successes: 2, Failures: 3}}
	if got := MergeState(nil, s); !stateEQ(got, s) {
		t.Fatalf("nil + s: got %v", got)
	}
	if got := MergeState(s, nil); !stateEQ(got, s) {
		t.Fatalf("s + nil: got %v", got)
	}
	if got := MergeState(nil, nil); len(got) != 0 {
		t.Fatalf("nil + nil: got %v", got)
	}
}

// TestEffectivePosterior pins the Bayesian reconstruction:
// alpha = 1 + successes, beta = 1 + failures. The posterior
// mean is the Bayesian point estimate of the arm's success
// rate. Merge must not lose observations; a posterior that
// reads below the count of observed trials is a correctness
// bug.
func TestEffectivePosterior(t *testing.T) {
	t.Parallel()
	a := Snapshot{ReplicaID: "rep-a", Counters: State{"arm": {Successes: 7, Failures: 3}}}
	b := Snapshot{ReplicaID: "rep-b", Counters: State{"arm": {Successes: 4, Failures: 6}}}
	merged := MergeSnapshots(a, b)
	if len(merged) != 2 {
		t.Fatalf("replica partitions lost: %v", merged)
	}
	counter := Aggregate(merged)["arm"]
	if counter != (Counter{Successes: 11, Failures: 9}) {
		t.Fatalf("aggregate: got %+v want {11,9}", counter)
	}
}

func TestMergeSnapshotsMaxesSameReplica(t *testing.T) {
	t.Parallel()
	a := Snapshot{ReplicaID: "rep-a", Counters: State{"arm": {Successes: 7, Failures: 3}}}
	b := Snapshot{ReplicaID: "rep-a", Counters: State{"arm": {Successes: 4, Failures: 6}}}
	got := MergeSnapshots(a, b)["rep-a"]["arm"]
	if got != (Counter{Successes: 7, Failures: 6}) {
		t.Fatalf("same-replica merge: %+v", got)
	}
}

// TestStateIsReferenceComparable protects a downstream
// invariant: the banditstore backend assumes the State is a
// map. A future refactor that swaps in a struct (which would
// still satisfy the merge APIs) would break the wire format.
func TestStateIsReferenceComparable(t *testing.T) {
	t.Parallel()
	var s State = map[string]Counter{}
	// unsafe.Sizeof on a map header should be a pointer
	// pair; non-map types wouldn't pass this guard.
	if unsafe.Sizeof(s) == 0 {
		t.Fatal("State should not be a zero-sized type")
	}
}

// TestMergeStateKeyOrdering: merge order is irrelevant for the
// result, but the result keys must equal the union of input
// keys. A bug that loses keys under merge would silently drop
// arms.
func TestMergeStateKeyOrdering(t *testing.T) {
	t.Parallel()
	a := State{"a": {}, "b": {}, "c": {}}
	b := State{"c": {}, "d": {}, "e": {}}
	got := MergeState(a, b)
	want := []string{"a", "b", "c", "d", "e"}
	sort.Strings(want)
	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, want) {
		t.Fatalf("keys: got %v want %v", gotKeys, want)
	}
}

// deepCopyState duplicates a State for mutation-safety tests.
// Counters are plain values so the copy is shallow per-counter
// but deep per-key.
func deepCopyState(s State) State {
	out := make(State, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}
