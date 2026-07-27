package election

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "leader.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEnsureTableIdempotent(t *testing.T) {
	db := openTestDB(t)
	e := New(db, "pod-a", 5*time.Second)
	for i := 0; i < 3; i++ {
		if err := e.EnsureTable(context.Background()); err != nil {
			t.Fatalf("ensure %d: %v", i, err)
		}
	}
}

func TestAcquireFirstCallerWins(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", 5*time.Second)
	b := New(db, "pod-b", 5*time.Second)
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatalf("pod-a acquire: %v", err)
	}
	if !a.IsLeader() {
		t.Fatal("pod-a should be leader")
	}
	if err := b.Acquire(context.Background()); err != ErrNotLeader {
		t.Fatalf("pod-b acquire: want ErrNotLeader, got %v", err)
	}
	if b.IsLeader() {
		t.Fatal("pod-b should not be leader")
	}
}

func TestRenewDoesNotRelease(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", 2*time.Second)
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Multiple renews should be idempotent.
	for i := 0; i < 3; i++ {
		if err := a.Acquire(context.Background()); err != nil {
			t.Fatalf("renew %d: %v", i, err)
		}
	}
	if !a.IsLeader() {
		t.Fatal("pod-a should still be leader")
	}
}

func TestStaleLeaseIsStolen(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", time.Hour) // long TTL
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Simulate pod-a going away by force-expiring its lease.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE leader SET expires_at = ? WHERE name = 'promptsheon'`,
		time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("force expire: %v", err)
	}

	b := New(db, "pod-b", 5*time.Second)
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("pod-b steal: %v", err)
	}
	if !b.IsLeader() {
		t.Fatal("pod-b should be leader after stealing stale lease")
	}
}

func TestRelease(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", 5*time.Second)
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Release(context.Background()); err != nil {
		t.Fatalf("release: %v", err)
	}
	if a.IsLeader() {
		t.Fatal("pod-a should not be leader after Release")
	}
	// pod-b can now acquire immediately.
	b := New(db, "pod-b", 5*time.Second)
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("pod-b acquire after release: %v", err)
	}
}

func TestCurrentReportsIdentity(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", 5*time.Second)
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := a.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Identity != "pod-a" {
		t.Errorf("want pod-a, got %s", got.Identity)
	}
	if !got.IsLeader {
		t.Error("IsLeader should be true on the leader")
	}
}

func TestRunRenewLoop(t *testing.T) {
	db := openTestDB(t)
	a := New(db, "pod-a", 100*time.Millisecond)
	if err := a.EnsureTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.Run(ctx, errs)
	}()
	time.Sleep(250 * time.Millisecond)
	cancel()
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("unexpected error: %v", e)
	}
	if a.IsLeader() {
		t.Error("pod-a should have stepped down after cancel")
	}
}
