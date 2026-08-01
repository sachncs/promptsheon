//go:build tests_migration


package store_test

import (
	"github.com/sachncs/promptsheon/promptsheon/models"
	"context"
	"fmt"
	"sync"
	"testing"

)

// BenchmarkAppendAuditCASSerial measures AppendAudit throughput
// under no contention. Pin the per-call latency so we can detect
// regressions in the CAS retry loop (c0.11). Run with
//
//	go test -bench=BenchmarkAppendAuditCASSerial -benchtime=2s \
//	    -count=3 ./backend/tests/unit/store/
func BenchmarkAppendAuditCASSerial(b *testing.B) {
	s := newTestSQLite(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &models.AuditEntry{
			ID:       fmt.Sprintf("a-%d", i),
			UserID:   "u1",
			Action:   "create",
			Resource: fmt.Sprintf("capability/c-%d", i),
			Details:  map[string]any{"i": i},
		}
		if err := s.AppendAudit(ctx, entry); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAppendAuditCASContended measures AppendAudit under
// N goroutines. The audit chain's CAS retry loop (c0.11) bounds
// the worst case at appendAuditMaxAttempts (32); this benchmark
// pins that bound and detects any future regression.
func BenchmarkAppendAuditCASContended(b *testing.B) {
	s := newTestSQLite(b)
	ctx := context.Background()

	const goroutines = 8

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(goroutines)
		errs := make(chan error, goroutines)
		for g := 0; g < goroutines; g++ {
			g := g
			go func() {
				defer wg.Done()
				entry := &models.AuditEntry{
					ID:       fmt.Sprintf("a-c-%d-%d", i, g),
					UserID:   "u1",
					Action:   "create",
					Resource: fmt.Sprintf("capability/c-%d-%d", i, g),
					Details:  map[string]any{"i": i, "g": g},
				}
				if err := s.AppendAudit(ctx, entry); err != nil {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			b.Fatal(e)
		}
	}
}
