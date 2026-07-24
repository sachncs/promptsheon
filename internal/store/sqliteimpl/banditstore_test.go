package sqliteimpl

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
	"github.com/sachncs/promptsheon/internal/banditstore"
	"github.com/sachncs/promptsheon/internal/store"
)

func TestBanditStoreSQLite_ObservePersists(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if err := bs.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := bs.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := bs.Observe(context.Background(), "rep-a", "arm-1", false); err != nil {
		t.Fatal(err)
	}
	st, err := bs.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st["arm-1"] != (bandit.Counter{Successes: 2, Failures: 1}) {
		t.Fatalf("expected {2,1}, got %+v", st["arm-1"])
	}
}

func TestBanditStoreSQLite_MergeConverges(t *testing.T) {
	db := openTestDB(t)
	// Two replicas observe disjoint arms; both end up at the
	// expected SUM aggregate after they exchange per-replica
	// snapshots. The per-replica Merge is still component-wise
	// max so duplicates from a single replica are no-ops.
	storeA, err := banditstore.NewStoreWithReplica(NewBanditStore(db.DB()), "rep-a")
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := banditstore.NewStoreWithReplica(NewBanditStore(db.DB()), "rep-b")
	if err != nil {
		t.Fatal(err)
	}
	if err := storeA.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := storeA.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := storeB.Observe(context.Background(), "arm-1", false); err != nil {
		t.Fatal(err)
	}
	if err := storeB.Observe(context.Background(), "arm-2", true); err != nil {
		t.Fatal(err)
	}
	// Replicas exchange per-replica raw state (not the Load
	// aggregate) so the SUM-on-Load accounts every replica's
	// observations exactly once.
	stB := bandit.State{
		"arm-1": {Successes: 0, Failures: 1},
		"arm-2": {Successes: 1, Failures: 0},
	}
	if err := storeA.Merge(context.Background(), "rep-b", stB); err != nil {
		t.Fatal(err)
	}
	stA := bandit.State{
		"arm-1": {Successes: 2, Failures: 0},
	}
	if err := storeB.Merge(context.Background(), "rep-a", stA); err != nil {
		t.Fatal(err)
	}
	want := bandit.State{
		"arm-1": {Successes: 2, Failures: 1},
		"arm-2": {Successes: 1, Failures: 0},
	}
	gotA, _ := storeA.Load(context.Background())
	gotB, _ := storeB.Load(context.Background())
	if !stateEqual(gotA, want) {
		t.Fatalf("rep-a: got %+v want %+v", gotA, want)
	}
	if !stateEqual(gotB, want) {
		t.Fatalf("rep-b: got %+v want %+v", gotB, want)
	}
}

// TestBanditStoreSQLite_LoadSumsAcrossReplicas pins the
// cross-replica aggregation contract on SQLite: two replicas
// each holding their own (successes, failures) for the same
// arm produce an effective bucket of (sum, sum), not the
// component-wise max.
func TestBanditStoreSQLite_LoadSumsAcrossReplicas(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if _, err := db.DB().ExecContext(context.Background(),
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES ('arm-1', 'rep-a', 2, 1)`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(context.Background(),
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES ('arm-1', 'rep-b', 3, 4)`,
	); err != nil {
		t.Fatal(err)
	}
	st, err := bs.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := bandit.Counter{Successes: 5, Failures: 5}
	if got := st["arm-1"]; got != want {
		t.Fatalf("effective arm-1: got %+v want %+v (per-replica sum, not max)", got, want)
	}
}

func TestBanditStoreSQLite_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	open := func() *BanditStore {
		t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
		s, err := store.NewSQLite(dir + "/restart.db")
		if err != nil {
			t.Fatal(err)
		}
		return NewBanditStore(s.DB())
	}
	bs1 := open()
	if err := bs1.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := bs1.Observe(context.Background(), "rep-a", "arm-2", false); err != nil {
		t.Fatal(err)
	}
	bs2 := open()
	st, err := bs2.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st["arm-1"] != (bandit.Counter{Successes: 1}) {
		t.Fatalf("arm-1: got %+v", st["arm-1"])
	}
	if st["arm-2"] != (bandit.Counter{Failures: 1}) {
		t.Fatalf("arm-2: got %+v", st["arm-2"])
	}
}

func TestBanditStoreSQLite_ObserveConflictSafe(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	// Seed a row directly with a higher count, then Observe
	// a lower-bump. The MAX() update must preserve the seed.
	if _, err := db.DB().ExecContext(context.Background(),
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES ('arm-1', 'rep-a', 10, 10)`,
	); err != nil {
		t.Fatal(err)
	}
	if err := bs.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	st, _ := bs.Load(context.Background())
	// The MAX() formula in Observe protects the existing row:
	// max(incoming(1), existing(10)+1) = 11.
	if st["arm-1"].Successes != 11 {
		t.Fatalf("successes: got %d want 11", st["arm-1"].Successes)
	}
	if st["arm-1"].Failures != 10 {
		t.Fatalf("failures: got %d want 10", st["arm-1"].Failures)
	}
}

func TestBanditStoreSQLite_MergeRejectsEmptyReplica(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if err := bs.Merge(context.Background(), "", bandit.State{"arm-1": {}}); err == nil {
		t.Fatal("expected error for empty replica id")
	}
}

func TestBanditStoreSQLite_ObserveRejectsEmptyID(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if err := bs.Observe(context.Background(), "", "arm-1", true); err == nil {
		t.Fatal("expected error for empty replica id")
	}
	if err := bs.Observe(context.Background(), "rep-a", "", true); err == nil {
		t.Fatal("expected error for empty arm id")
	}
}

func TestBanditStoreSQLite_RegisterArmsDoesNotInventObservations(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if err := bs.Merge(context.Background(), "rep-a", bandit.State{"arm-1": {}}); err != nil {
		t.Fatal(err)
	}
	st, _ := bs.Load(context.Background())
	if c := st["arm-1"]; c.Successes != 0 || c.Failures != 0 {
		t.Fatalf("register invented observations: %+v", c)
	}
}

func TestBanditStoreSQLite_PerReplicaIsolation(t *testing.T) {
	db := openTestDB(t)
	bs := NewBanditStore(db.DB())
	if err := bs.Observe(context.Background(), "rep-a", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := bs.Observe(context.Background(), "rep-b", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := bs.Observe(context.Background(), "rep-b", "arm-1", true); err != nil {
		t.Fatal(err)
	}
	// Per-replica counters: rep-a {1,0}, rep-b {2,0}.
	// The merged view is the SUM: {3, 0}.
	st, _ := bs.Load(context.Background())
	if st["arm-1"] != (bandit.Counter{Successes: 3}) {
		t.Fatalf("expected merged successes=3, got %+v", st["arm-1"])
	}
	// Query the per-replica rows directly to confirm partition.
	var aSuccesses, bSuccesses int64
	if err := db.DB().QueryRowContext(context.Background(),
		`SELECT successes FROM bandit_arm_counters WHERE arm_id='arm-1' AND replica_id='rep-a'`,
	).Scan(&aSuccesses); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRowContext(context.Background(),
		`SELECT successes FROM bandit_arm_counters WHERE arm_id='arm-1' AND replica_id='rep-b'`,
	).Scan(&bSuccesses); err != nil {
		t.Fatal(err)
	}
	if aSuccesses != 1 || bSuccesses != 2 {
		t.Fatalf("per-replica: rep-a=%d rep-b=%d", aSuccesses, bSuccesses)
	}
}

func stateEqual(a, b bandit.State) bool {
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
