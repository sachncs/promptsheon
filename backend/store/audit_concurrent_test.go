// Concurrent audit-append tests. Lives in a separate file so
// the long-running store_test.go fixtures don't grow, and so the
// regression for the audit-tail cache is grouped with the cache
// logic.
package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/store"
)

// TestAppendAuditConcurrentChainPreserved hammers AppendAudit
// from N goroutines and asserts that:
//   - every append succeeds
//   - the audit chain still verifies (no rowid/previous_hash
//     corruption from concurrent reads of the cached tail)
//   - the cached (rowid, hash) in the SQLite struct matches the
//     persisted audit_chain_state row.
//
// This is the regression test for the per-append tail SELECT:
// the old path did N independent SELECTs and could observe a
// stale previous_hash under contention; the cache path must
// serialise through a mutex + SQLite's serialisable transaction
// so the chain stays intact.
func TestAppendAuditConcurrentChainPreserved(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	const goroutines = 8
	const perG = 25
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines*perG)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				id := idFor(g, i)
				entry := &models.AuditEntry{
					ID:       id,
					UserID:   "u1",
					Action:   "create",
					Resource: "capability/c-" + id,
					Details:  map[string]any{"g": g, "i": i},
				}
				if err := s.AppendAudit(ctx, entry); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("AppendAudit (concurrent): %v", e)
	}

	res, err := s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !res.Ok {
		t.Fatalf("audit chain did not verify after concurrent appends: %s", res.Reason)
	}
	if res.LastRowID <= 0 {
		t.Fatalf("expected positive LastRowID, got %d", res.LastRowID)
	}

	// The cached tail must agree with the persisted state.
	cached := readAuditTailForTest(t, s)
	if cached.rowID != uint64(res.LastRowID) || cached.hash != res.LastHash {
		t.Fatalf("cached tail diverged from DB: cached=(%d,%s) db=(%d,%s)",
			cached.rowID, cached.hash, res.LastRowID, res.LastHash)
	}

	// All entries we wrote must be present.
	wantIDs := make([]string, 0, goroutines*perG)
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perG; i++ {
			wantIDs = append(wantIDs, idFor(g, i))
		}
	}
	listed, err := s.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	seen := make(map[string]bool, len(listed))
	for _, e := range listed {
		seen[e.ID] = true
		if e.EntryHash == "" {
			t.Fatalf("entry %s missing EntryHash", e.ID)
		}
		// PreviousHash wiring is asserted by VerifyAuditChain
		// above (res.Ok). The chain links by rowid, not by
		// caller-supplied ID, so a per-entry PreviousHash check
		// here would conflate goroutine scheduling with chain
		// integrity.
	}
	for _, id := range wantIDs {
		if !seen[id] {
			t.Fatalf("missing audit entry %s after concurrent appends", id)
		}
	}
}

// TestAppendAuditCacheReadThrough verifies the read-through
// path: open a fresh SQLite, write two entries via AppendAudit,
// open a second SQLite handle to the same file, write one more
// entry, and confirm the new handle's first AppendAudit picks up
// the previous_hash from the DB rather than from an empty cache.
func TestAppendAuditCacheReadThrough(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/readthrough.db"

	first, err := store.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	seedDefaultUsers(t, first)

	ctx := context.Background()
	err = first.AppendAudit(ctx, &models.AuditEntry{
		ID: "rt-1", UserID: "u1", Action: "create", Resource: "x/y",
	})
	if err != nil {
		t.Fatalf("AppendAudit 1: %v", err)
	}
	err = first.AppendAudit(ctx, &models.AuditEntry{
		ID: "rt-2", UserID: "u1", Action: "create", Resource: "x/y",
	})
	if err != nil {
		t.Fatalf("AppendAudit 2: %v", err)
	}
	res, err := first.VerifyAuditChain(ctx)
	if err != nil || !res.Ok {
		t.Fatalf("first verify: res=%+v err=%v", res, err)
	}
	firstTail := res.LastHash

	// Second handle shares the file but starts with an empty
	// cache. Its first AppendAudit must read the persisted tail
	// and produce a chain that still verifies.
	second, err := store.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	err = second.AppendAudit(ctx, &models.AuditEntry{
		ID: "rt-3", UserID: "u1", Action: "create", Resource: "x/y",
	})
	if err != nil {
		t.Fatalf("AppendAudit 3: %v", err)
	}
	res2, err := second.VerifyAuditChain(ctx)
	if err != nil || !res2.Ok {
		t.Fatalf("second verify: res=%+v err=%v", res2, err)
	}
	if res2.LastHash == firstTail {
		t.Fatalf("expected third entry to extend the chain, but tail hash unchanged")
	}
	if res2.LastRowID != res.LastRowID+1 {
		t.Fatalf("expected LastRowID %d, got %d", res.LastRowID+1, res2.LastRowID)
	}
}

func TestAppendAuditInterleavedHandles(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/interleaved.db"

	first, err := store.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	seedDefaultUsers(t, first)

	second, err := store.NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	ctx := context.Background()
	firstEntry := &models.AuditEntry{ID: "interleaved-a1", UserID: "u1", Action: "first", Resource: "audit/1"}
	secondEntry := &models.AuditEntry{ID: "interleaved-b1", UserID: "u1", Action: "second", Resource: "audit/2"}
	timestamp := time.Date(2026, time.July, 24, 12, 34, 56, 789, time.UTC)
	retriedEntry := &models.AuditEntry{
		ID:        "interleaved-a2",
		UserID:    "u1",
		Action:    "retried",
		Resource:  "audit/3",
		Details:   map[string]any{"value": "preserved"},
		Timestamp: timestamp,
	}

	if appendErr := first.AppendAudit(ctx, firstEntry); appendErr != nil {
		t.Fatalf("AppendAudit first/A: %v", appendErr)
	}
	if appendErr := second.AppendAudit(ctx, secondEntry); appendErr != nil {
		t.Fatalf("AppendAudit second/B: %v", appendErr)
	}
	if appendErr := first.AppendAudit(ctx, retriedEntry); appendErr != nil {
		t.Fatalf("AppendAudit third/A: %v", appendErr)
	}

	res, err := first.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !res.Ok {
		t.Fatalf("audit chain did not verify after interleaved appends: %s", res.Reason)
	}
	if retriedEntry.PreviousHash != secondEntry.EntryHash {
		t.Fatalf("retried entry previous hash = %q, want %q", retriedEntry.PreviousHash, secondEntry.EntryHash)
	}

	listed, err := first.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("ListAudit count = %d, want 3", len(listed))
	}
	seen := make(map[string]*models.AuditEntry, len(listed))
	for _, entry := range listed {
		seen[entry.ID] = entry
	}
	for _, id := range []string{firstEntry.ID, secondEntry.ID, retriedEntry.ID} {
		if seen[id] == nil {
			t.Fatalf("missing audit entry %s", id)
		}
	}
	got := seen[retriedEntry.ID]
	if got.Action != retriedEntry.Action || got.Resource != retriedEntry.Resource || got.Details["value"] != "preserved" || !got.Timestamp.Equal(timestamp) {
		t.Fatalf("retried entry fields changed: got=%+v", got)
	}
}

type auditTailSnapshot struct {
	rowID uint64
	hash  string
}

func readAuditTailForTest(t *testing.T, s *store.SQLite) auditTailSnapshot {
	t.Helper()
	row := s.DB().QueryRow(`SELECT last_rowid, last_hash FROM audit_chain_state WHERE id = 0`)
	var snap auditTailSnapshot
	var hash string
	var rowID int64
	if err := row.Scan(&rowID, &hash); err != nil {
		t.Fatalf("read audit_chain_state: %v", err)
	}
	snap.rowID = uint64(rowID)
	snap.hash = hash
	return snap
}

func idFor(g, i int) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 8)
	n := g*1000 + i
	for j := 0; j < 8; j++ {
		out = append(out, hex[(n>>(j*4))&0xf])
	}
	return "a-" + string(out)
}
