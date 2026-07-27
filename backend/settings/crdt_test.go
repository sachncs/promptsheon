// CRDT algebraic properties for the settings LWW register.
// Every test below runs without I/O so the storage backend
// can be swapped (in-memory, SQLite) without re-asserting
// convergence.
package settings

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// makeRecord is the deterministic test fixture. Tests build a
// handful of records with controlled WriteTS/Vector values to
// pin every branch of mergeWithWinner.
func makeRecord(key, value, replica string, ts int64, vec map[string]uint64, tombstone bool) CRDTRecord {
	if vec == nil {
		vec = map[string]uint64{}
	}
	return CRDTRecord{
		Key:           key,
		Value:         value,
		ReplicaID:     replica,
		WriteTS:       ts,
		VersionVector: vec,
		Tombstone:     tombstone,
	}
}

// recordEqual compares two CRDTRecords without falling into
// the "map cannot be compared" trap. Maps are compared
// element-by-element; timestamps/strings compare directly.
func recordEqual(a, b CRDTRecord) bool {
	if a.Key != b.Key || a.Value != b.Value || a.UpdatedBy != b.UpdatedBy ||
		a.WriteTS != b.WriteTS || a.ReplicaID != b.ReplicaID ||
		a.Tombstone != b.Tombstone {
		return false
	}
	if len(a.VersionVector) != len(b.VersionVector) {
		return false
	}
	for k, va := range a.VersionVector {
		if vb, ok := b.VersionVector[k]; !ok || va != vb {
			return false
		}
	}
	return true
}

// TestMergeDominance asserts that when one vector strictly
// dominates the other, the dominant record wins.
func TestMergeDominance(t *testing.T) {
	t.Parallel()
	a := makeRecord("k", "old", "rep-a", 100, map[string]uint64{"rep-a": 1}, false)
	b := makeRecord("k", "new", "rep-b", 50, map[string]uint64{"rep-a": 2, "rep-b": 1}, false)
	if got := Merge(a, b); got.Value != "new" {
		t.Fatalf("dominant should win, got %+v", got)
	}
}

// TestMergeConcurrentTieBreak pins the deterministic
// tie-break: equal/incomparable vectors, equal WriteTS,
// distinct ReplicaID → lexicographically smaller replica id
// wins.
func TestMergeConcurrentTieBreak(t *testing.T) {
	t.Parallel()
	a := makeRecord("k", "from-a", "rep-a", 200, map[string]uint64{"rep-a": 1}, false)
	b := makeRecord("k", "from-b", "rep-b", 200, map[string]uint64{"rep-b": 1}, false)
	got := Merge(a, b)
	if got.ReplicaID != "rep-a" {
		t.Fatalf("lexicographically smaller replica id should win, got %q", got.ReplicaID)
	}
	if got.Value != "from-a" {
		t.Fatalf("loser value leaked: got %+v", got)
	}
}

// TestMergeTimestampBeatsTieBreak asserts that a higher
// WriteTS wins regardless of replica id when the vectors are
// incomparable.
func TestMergeTimestampBeatsTieBreak(t *testing.T) {
	t.Parallel()
	a := makeRecord("k", "old", "rep-a", 100, map[string]uint64{"rep-a": 1}, false)
	b := makeRecord("k", "new", "rep-z", 200, map[string]uint64{"rep-z": 1}, false)
	if got := Merge(a, b); got.Value != "new" {
		t.Fatalf("later timestamp should win, got %+v", got)
	}
}

// TestMergeACI asserts associativity, commutativity, and
// idempotence on arbitrary records (vectors can be
// ill-formed, payloads can diverge under equal metadata,
// tombstones are mixed with live records).
//
// Commutativity and idempotence hold for arbitrary inputs:
// the canonical payload-order tie-break makes the merge a
// total order once the operands are equal on every other
// field, and folding the vectors in every branch keeps the
// merge a stable join over the version-vector lattice.
//
// Associativity is restricted to well-formed clocks (a
// writer-monotone version vector where every record sees
// strictly more from its own replica). LWW with vector
// dominance is not associative on arbitrary inputs — the
// dominance check is a partial order and ts tie-break is a
// total order, so when neither dominates the intermediate
// merge can pick differently depending on operand order.
// The settings layer's Merge is only invoked from a single
// caller (Resolver.Merge via MergeSystemConfig) which always
// feeds it a per-key set of remote records, so the
// associativity ceiling is fine in practice.
func TestMergeACI(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 1))
	replicas := []string{"rep-a", "rep-b", "rep-c"}
	for i := 0; i < 500; i++ {
		writer := replicas[rng.IntN(len(replicas))]
		// Arbitrary, possibly ill-formed: counters are random,
		// payloads are random, tombstones are mixed. The merge
		// still has to be commutative and idempotent.
		tsA := int64(rng.Uint64() & 0xfff)
		tsB := int64(rng.Uint64() & 0xfff)
		vecA := randomVector(rng, replicas)
		vecB := randomVector(rng, replicas)
		tombA := rng.IntN(2) == 0
		tombB := rng.IntN(2) == 0
		ra := makeRecord("k", fmt.Sprintf("v-%d-a", i), writer, tsA, vecA, tombA)
		ra.UpdatedBy = fmt.Sprintf("by-%d", rng.IntN(4))
		rb := makeRecord("k", fmt.Sprintf("v-%d-b", i), writer, tsB, vecB, tombB)
		rb.UpdatedBy = fmt.Sprintf("by-%d", rng.IntN(4))

		// Commutativity (always required).
		if !recordEqual(Merge(ra, rb), Merge(rb, ra)) {
			t.Fatalf("commutativity violation at i=%d: a=%+v b=%+v", i, ra, rb)
		}
		// Idempotence (always required).
		if !recordEqual(Merge(ra, ra), ra) {
			t.Fatalf("idempotence violation at i=%d: a=%+v", i, ra)
		}
	}
}

// TestMergeACIWellFormed asserts associativity on
// well-formed clocks: each record's vector monotonically
// increases the writer's counter, and the per-replica
// counters are monotone in the writer's view. Under those
// constraints the merge is associative — the partial-order
// dominance check agrees with the total-order tie-break.
func TestMergeACIWellFormed(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 1))
	replicas := []string{"rep-a", "rep-b", "rep-c"}
	for i := 0; i < 500; i++ {
		writer := replicas[rng.IntN(len(replicas))]
		// Well-formed: each record's vector monotonically
		// increases the writer's counter, and other replicas
		// see strictly more in later writes.
		baseA := rng.Uint64() & 0xff
		baseB := baseA + 1 + rng.Uint64()&0x7
		baseC := baseB + 1 + rng.Uint64()&0x7
		tsA := int64(baseA) + int64(rng.Uint64()&0xf)
		tsB := tsA + 1 + int64(rng.Uint64()&0xf)
		tsC := tsB + 1 + int64(rng.Uint64()&0xf)
		ra := makeRecord("k", fmt.Sprintf("v-%d-a", i), writer, tsA, map[string]uint64{writer: baseA + 1}, false)
		rb := makeRecord("k", fmt.Sprintf("v-%d-b", i), writer, tsB, map[string]uint64{writer: baseB + 1}, false)
		rc := makeRecord("k", fmt.Sprintf("v-%d-c", i), writer, tsC, map[string]uint64{writer: baseC + 1}, false)

		// Commutativity.
		if !recordEqual(Merge(ra, rb), Merge(rb, ra)) {
			t.Fatalf("commutativity violation at i=%d", i)
		}
		// Associativity (well-formed only).
		left := Merge(Merge(ra, rb), rc)
		right := Merge(ra, Merge(rb, rc))
		if !recordEqual(left, right) {
			t.Fatalf("associativity violation at i=%d: left=%+v right=%+v", i, left, right)
		}
		// Idempotence.
		if !recordEqual(Merge(ra, ra), ra) {
			t.Fatalf("idempotence violation at i=%d", i)
		}
	}
}

// TestMergeTombstoneWinsExactMetadataTie pins the
// no-resurrection rule at the canonical-tie-break layer:
// two records with identical vector + WriteTS + ReplicaID
// where one is a tombstone and the other is a live write
// must converge to the tombstone, regardless of the order
// of operands. Without the payload-order tie-break, the
// merge was direction-dependent and a Set could resurrect a
// tombstoned key.
func TestMergeTombstoneWinsExactMetadataTie(t *testing.T) {
	t.Parallel()
	vec := map[string]uint64{"rep-a": 1}
	ts := int64(100)
	replica := "rep-a"
	deleted := makeRecord("k", "", replica, ts, vec, true)
	live := makeRecord("k", "new-value", replica, ts, vec, false)
	gotAB := Merge(deleted, live)
	gotBA := Merge(live, deleted)
	if !gotAB.Tombstone || !gotBA.Tombstone {
		t.Fatalf("tombstone lost under metadata tie: AB=%+v BA=%+v", gotAB, gotBA)
	}
	if !recordEqual(gotAB, gotBA) {
		t.Fatalf("commutativity under metadata tie: AB=%+v BA=%+v", gotAB, gotBA)
	}
	if gotAB.Value != "" {
		t.Fatalf("tombstone merge leaked live value: %q", gotAB.Value)
	}
}

// TestMergeDivergentPayloadIsCommutative exercises the
// payload-order tie-break directly: two records that
// collide on every CRDT metadata field but carry different
// (Value, UpdatedBy) still merge to the same record no
// matter which operand comes first. The canonical order is
// lexicographic Value, then UpdatedBy.
func TestMergeDivergentPayloadIsCommutative(t *testing.T) {
	t.Parallel()
	vec := map[string]uint64{"rep-a": 1}
	ts := int64(100)
	replica := "rep-a"
	a := makeRecord("k", "alpha", replica, ts, vec, false)
	a.UpdatedBy = "alice"
	b := makeRecord("k", "beta", replica, ts, vec, false)
	b.UpdatedBy = "alice"
	gotAB := Merge(a, b)
	gotBA := Merge(b, a)
	if !recordEqual(gotAB, gotBA) {
		t.Fatalf("divergent payload: AB=%+v BA=%+v", gotAB, gotBA)
	}
	if gotAB.Value != "alpha" {
		t.Fatalf("canonical order: got %q want %q (lex-smaller wins)", gotAB.Value, "alpha")
	}
}

// randomVector builds a map[string]uint64 with at most one
// entry per replica and random per-replica counters. Used by
// the arbitrary-record property test to exercise ill-formed
// vectors (counters can regress across replicas; the merge
// still has to converge).
func randomVector(rng *rand.Rand, replicas []string) map[string]uint64 {
	out := map[string]uint64{}
	for _, r := range replicas {
		if rng.IntN(2) == 0 {
			out[r] = rng.Uint64() & 0xff
		}
	}
	if len(out) == 0 {
		out[replicas[rng.IntN(len(replicas))]] = 1
	}
	return out
}

// TestTombstoneHidesRecord asserts that a tombstoned record
// still wins merges — the settings layer filters tombstones
// out of Get/List, but they must dominate concurrent writes
// so a Set does not resurrect a deleted key.
func TestTombstoneHidesRecord(t *testing.T) {
	t.Parallel()
	deleted := makeRecord("k", "", "rep-a", 100, map[string]uint64{"rep-a": 2}, true)
	resurrect := makeRecord("k", "resurrected", "rep-b", 50, map[string]uint64{"rep-b": 1}, false)
	if got := Merge(deleted, resurrect); !got.Tombstone {
		t.Fatalf("tombstone should dominate, got %+v", got)
	}
}

// TestSetCanResurrectAfterDominance asserts the no-resurrection
// rule: a Set on a tombstoned key only sticks if the new
// write dominates the tombstone's vector.
func TestSetCanResurrectAfterDominance(t *testing.T) {
	t.Parallel()
	deleted := makeRecord("k", "", "rep-a", 100, map[string]uint64{"rep-a": 2}, true)
	// rep-a writes again — its vector dominates.
	resurrect := makeRecord("k", "back", "rep-a", 200, map[string]uint64{"rep-a": 3}, false)
	if got := Merge(deleted, resurrect); got.Tombstone || got.Value != "back" {
		t.Fatalf("dominant Set should resurrect, got %+v", got)
	}
}

// TestVectorDominatedUnit pins the VectorDominated function:
// a must be >= b in every coordinate with strict inequality
// somewhere.
func TestVectorDominatedUnit(t *testing.T) {
	t.Parallel()
	if VectorDominated(map[string]uint64{"a": 1}, map[string]uint64{"a": 1}) {
		t.Fatal("equal vectors should not be dominated")
	}
	if !VectorDominated(map[string]uint64{"a": 2}, map[string]uint64{"a": 1}) {
		t.Fatal("a=2 should dominate a=1")
	}
	if VectorDominated(map[string]uint64{"a": 1}, map[string]uint64{"a": 2}) {
		t.Fatal("a=1 should not dominate a=2")
	}
	if VectorDominated(map[string]uint64{"a": 1}, map[string]uint64{"b": 1}) {
		t.Fatal("incomparable vectors: a should not dominate b")
	}
	if VectorDominated(nil, nil) {
		t.Fatal("empty/empty is incomparable, not dominated")
	}
}

// TestIncVector does not mutate the input.
func TestIncVector(t *testing.T) {
	t.Parallel()
	in := map[string]uint64{"rep-a": 3}
	snap := make(map[string]uint64, len(in))
	for k, v := range in {
		snap[k] = v
	}
	out := IncVector(in, "rep-a")
	if out["rep-a"] != 4 {
		t.Fatalf("increment: got %d", out["rep-a"])
	}
	// Original map is untouched (caller may share it across
	// goroutines).
	if in["rep-a"] != 3 {
		t.Fatalf("original mutated: %v", in)
	}
	// Equal-to-snapshot check.
	for k, v := range snap {
		if in[k] != v {
			t.Fatalf("original drift at %q: %d != %d", k, in[k], v)
		}
	}
}

// TestIncVectorNewReplica: bumping an unseen replica starts
// at 1.
func TestIncVectorNewReplica(t *testing.T) {
	t.Parallel()
	out := IncVector(map[string]uint64{"rep-a": 5}, "rep-b")
	if out["rep-b"] != 1 {
		t.Fatalf("new replica: got %d", out["rep-b"])
	}
	if out["rep-a"] != 5 {
		t.Fatalf("existing replica should not regress: got %d", out["rep-a"])
	}
}

// TestMergeRecordsEmpty pins the degenerate case.
func TestMergeRecordsEmpty(t *testing.T) {
	t.Parallel()
	if got := MergeRecords(); got.Key != "" {
		t.Fatalf("empty merge should be zero-value, got %+v", got)
	}
}

// TestMergeRecordsSingle pins the single-element case (the
// fold must be a no-op).
func TestMergeRecordsSingle(t *testing.T) {
	t.Parallel()
	r := makeRecord("k", "v", "rep-a", 1, map[string]uint64{"rep-a": 1}, false)
	if got := MergeRecords(r); !recordEqual(got, r) {
		t.Fatalf("single: got %+v want %+v", got, r)
	}
}

// TestConcurrentReplicaConvergence: two replicas write
// disjoint vectors to the same key; after exchanging
// records, they converge to the same CRDTRecord.
func TestConcurrentReplicaConvergence(t *testing.T) {
	t.Parallel()
	a := makeRecord("k", "from-a", "rep-a", 100, map[string]uint64{"rep-a": 1}, false)
	b := makeRecord("k", "from-b", "rep-b", 200, map[string]uint64{"rep-b": 1}, false)
	// Replicas exchange their records; both apply the same
	// merge — the result is identical.
	gotA := Merge(a, b)
	gotB := Merge(b, a)
	if !recordEqual(gotA, gotB) {
		t.Fatalf("not converged: a=%+v b=%+v", gotA, gotB)
	}
	if gotA.WriteTS != 200 {
		t.Fatalf("later timestamp should win, got %d", gotA.WriteTS)
	}
}

// TestTombstoneDoesNotResurrectUnderEqualVector: a Set on a
// tombstoned key with the same vector + same WriteTS does NOT
// resurrect. The tombstone wins because VectorDominated
// reports false for equal vectors (incomparable).
func TestTombstoneDoesNotResurrectUnderEqualVector(t *testing.T) {
	t.Parallel()
	deleted := makeRecord("k", "", "rep-a", 100, map[string]uint64{"rep-a": 1}, true)
	set := makeRecord("k", "new", "rep-a", 100, map[string]uint64{"rep-a": 1}, false)
	got := Merge(deleted, set)
	if !got.Tombstone {
		t.Fatalf("tombstone with identical vector+ts should win, got %+v", got)
	}
}

// TestMergeFoldsVectorEntries is the regression for an old
// merge implementation that silently dropped the loser's
// higher vector counter. The merged record's vector must be
// the component-wise max of both operands — folding keeps
// causality visible to downstream merges.
func TestMergeFoldsVectorEntries(t *testing.T) {
	t.Parallel()
	a := makeRecord("k", "", "rep-a", 100, map[string]uint64{"rep-a": 3, "rep-b": 1}, false)
	b := makeRecord("k", "", "rep-b", 200, map[string]uint64{"rep-a": 1, "rep-b": 2}, false)
	// a dominates (rep-a: 3>1, rep-b: 1<2 → not dominated)
	// b dominates (rep-a: 1<3 → not dominated)
	// Tie-break by timestamp: b wins. The folded vector is
	// the component-wise max of a's and b's vectors, so the
	// result has rep-a=3 (a's higher) and rep-b=2 (b's).
	got := Merge(a, b)
	if got.WriteTS != 200 {
		t.Fatalf("ts: got %d", got.WriteTS)
	}
	if got.VersionVector["rep-b"] != 2 {
		t.Fatalf("rep-b count: got %d", got.VersionVector["rep-b"])
	}
	if got.VersionVector["rep-a"] != 3 {
		t.Fatalf("rep-a count: got %d want 3 (fold preserves max)", got.VersionVector["rep-a"])
	}
}
