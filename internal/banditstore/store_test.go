package banditstore

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
)

func TestNewStoreRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewStore(nil); err == nil {
		t.Fatalf("expected error for nil backend")
	}
}

func TestNewStoreWithReplicaRejectsNilBackend(t *testing.T) {
	t.Parallel()
	if _, err := NewStoreWithReplica(nil, "rep-1"); err == nil {
		t.Fatalf("expected error for nil backend")
	}
}

func TestNewStoreWithReplicaRejectsEmptyReplicaID(t *testing.T) {
	t.Parallel()
	if _, err := NewStoreWithReplica(NewInMemory(), ""); err == nil {
		t.Fatalf("expected error for empty replica id")
	}
}

func TestInMemoryLoadEmpty(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	st, err := im.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st) != 0 {
		t.Fatalf("expected empty State, got %d", len(st))
	}
}

func TestInMemoryObserveRoundTrip(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	if err := im.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := im.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := im.Observe(context.Background(), "rep-a", "arm-1", false); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	st, err := im.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := bandit.Counter{Successes: 2, Failures: 1}
	if got := st["arm-1"]; got != want {
		t.Fatalf("arm-1: got %+v want %+v", got, want)
	}
}

func TestInMemoryMergeComponentWise(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	if err := im.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := im.Merge(context.Background(), "rep-b", bandit.State{
		"arm-1": {Successes: 5, Failures: 2},
		"arm-2": {Successes: 0, Failures: 1},
	}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Per-replica Merge is component-wise max: rep-b's arm-1
	// row goes from {0,0} to {5, 2}; rep-a's arm-1 row stays
	// at {1, 0}. The Load is then the SUM across replicas.
	// arm-1: rep-a {1,0} + rep-b {5,2} = {6, 2}
	if st, _ := im.Load(context.Background()); st["arm-1"] != (bandit.Counter{Successes: 6, Failures: 2}) {
		t.Fatalf("arm-1: got %+v", st["arm-1"])
	}
	// arm-2 only has rep-b: stays at {0,1}
	if st, _ := im.Load(context.Background()); st["arm-2"] != (bandit.Counter{Successes: 0, Failures: 1}) {
		t.Fatalf("arm-2: got %+v", st["arm-2"])
	}
}

// TestInMemoryLoadSumsAcrossReplicas pins the cross-replica
// aggregation contract: two replicas each holding (2, 1) and
// (3, 4) for the same arm produce an effective bucket of
// (5, 5), not the component-wise max of (3, 4). The per-replica
// Merge stays MAX (no-op for duplicates), but Load SUMs every
// replica's contribution into one bucket per arm.
func TestInMemoryLoadSumsAcrossReplicas(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	im.Seed("rep-a", "arm-1", bandit.Counter{Successes: 2, Failures: 1})
	im.Seed("rep-b", "arm-1", bandit.Counter{Successes: 3, Failures: 4})
	st, err := im.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := bandit.Counter{Successes: 5, Failures: 5}
	if got := st["arm-1"]; got != want {
		t.Fatalf("effective arm-1: got %+v want %+v (per-replica sum, not max)", got, want)
	}
}

func TestStoreLoadEmpty(t *testing.T) {
	t.Parallel()
	s, err := NewStoreWithReplica(NewInMemory(), "rep-1")
	if err != nil {
		t.Fatalf("NewStoreWithReplica: %v", err)
	}
	st, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st) != 0 {
		t.Fatalf("expected empty State, got %d", len(st))
	}
}

func TestStoreObservePersistsToBackend(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	s, err := NewStoreWithReplica(im, "rep-1")
	if err != nil {
		t.Fatalf("NewStoreWithReplica: %v", err)
	}
	err = s.Observe(context.Background(), "arm-1", true)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	st, err := im.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st["arm-1"].Successes != 1 {
		t.Fatalf("expected successes=1, got %+v", st["arm-1"])
	}
}

func TestStoreRegisterArmsDoesNotInventObservations(t *testing.T) {
	t.Parallel()
	s, err := NewStoreWithReplica(NewInMemory(), "rep-1")
	if err != nil {
		t.Fatalf("NewStoreWithReplica: %v", err)
	}
	err = s.RegisterArms(context.Background(), []string{"arm-1", "arm-2"})
	if err != nil {
		t.Fatalf("RegisterArms: %v", err)
	}
	st, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for id, c := range st {
		if c.Successes != 0 || c.Failures != 0 {
			t.Fatalf("arm %s: register should not invent observations, got %+v", id, c)
		}
	}
}

func TestStoreGetRoundTrip(t *testing.T) {
	t.Parallel()
	s, _ := NewStoreWithReplica(NewInMemory(), "rep-1")
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatal(err)
	}
	c, ok := s.Get(context.Background(), "arm-1")
	if !ok || c.Successes != 0 || c.Failures != 0 {
		t.Fatalf("expected empty counter for registered arm, got %+v ok=%v", c, ok)
	}
}

func TestStoreMergeConverges(t *testing.T) {
	t.Parallel()
	// Two replicas accumulate disjoint observations; after a
	// round-trip merge they converge to the same effective
	// State. Cross-replica Load is SUM, so both replicas
	// agree on the global observation total once they have
	// each other's per-replica observations.
	a, _ := NewStoreWithReplica(NewInMemory(), "rep-a")
	b, _ := NewStoreWithReplica(NewInMemory(), "rep-b")
	if err := a.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := a.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := b.Observe(context.Background(), "arm-1", false); err != nil {
		t.Fatal(err)
	}
	if err := b.Observe(context.Background(), "arm-2", true); err != nil {
		t.Fatal(err)
	}
	// Replicas exchange per-replica raw state (not the Load
	// aggregate) so the SUM-on-Load accounts every replica's
	// observations exactly once. The transport here is the
	// Store.Merge call with the peer's view; in production
	// the per-replica accessor is the bandit's wire format.
	stB := bandit.State{
		"arm-1": {Successes: 0, Failures: 1},
		"arm-2": {Successes: 1, Failures: 0},
	}
	if err := a.Merge(context.Background(), "rep-b", stB); err != nil {
		t.Fatal(err)
	}
	stA := bandit.State{
		"arm-1": {Successes: 2, Failures: 0},
	}
	if err := b.Merge(context.Background(), "rep-a", stA); err != nil {
		t.Fatal(err)
	}
	want := bandit.State{
		"arm-1": {Successes: 2, Failures: 1},
		"arm-2": {Successes: 1, Failures: 0},
	}
	gotA, _ := a.Load(context.Background())
	gotB, _ := b.Load(context.Background())
	if !countersEqual(gotA, want) {
		t.Fatalf("rep-a post-merge: got %+v want %+v", gotA, want)
	}
	if !countersEqual(gotB, want) {
		t.Fatalf("rep-b post-merge: got %+v want %+v", gotB, want)
	}
}

func TestStoreNewEntropyBackedReplicaID(t *testing.T) {
	t.Parallel()
	s, err := NewStore(NewInMemory())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if s.ReplicaID() == "" {
		t.Fatal("expected entropy-seeded replica id")
	}
	if len(s.ReplicaID()) != 32 {
		t.Fatalf("expected 32-char hex replica id, got %d", len(s.ReplicaID()))
	}
	// Two NewStore calls produce different replica ids.
	s2, err2 := NewStore(NewInMemory())
	if err2 != nil {
		t.Fatalf("NewStore (s2): %v", err2)
	}
	if s.ReplicaID() == s2.ReplicaID() {
		t.Fatal("expected distinct replica ids")
	}
}

func TestInMemorySeedVisibleAfterLoad(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	im.Seed("rep-a", "arm-1", bandit.Counter{Successes: 3, Failures: 1})
	st, _ := im.Load(context.Background())
	if st["arm-1"] != (bandit.Counter{Successes: 3, Failures: 1}) {
		t.Fatalf("seed: got %+v", st["arm-1"])
	}
}

func TestStoreArmIDs(t *testing.T) {
	t.Parallel()
	s, _ := NewStoreWithReplica(NewInMemory(), "rep-1")
	if err := s.RegisterArms(context.Background(), []string{"arm-1", "arm-2"}); err != nil {
		t.Fatal(err)
	}
	ids, err := s.ArmIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 arm ids, got %v", ids)
	}
}

func TestStoreFlushNoOp(t *testing.T) {
	t.Parallel()
	s, _ := NewStoreWithReplica(NewInMemory(), "rep-1")
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func countersEqual(a, b bandit.State) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			return false
		}
	}
	return true
}
