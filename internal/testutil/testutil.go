// Package testutil provides shared test helpers used by every
// package's _test.go files. The goal is to keep test boilerplate
// out of production packages and to give every test file the
// same conventions for setup, cleanup, and fakes.
//
// Conventions:
//   - All helpers take a *testing.T first.
//   - Cleanup is registered with t.Cleanup so the test ends in a
//     known state even on panic.
//   - Fakes are concurrency-safe.
package testutil

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/store"
)

// DiscardLogger returns a *slog.Logger that writes to io.Discard.
// Use for tests that need a logger but do not assert on output.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TempSQLite opens a fresh on-disk SQLite database in t.TempDir(),
// runs migrations, and registers cleanup. Returns nil and skips
// the test on open error (e.g., environment without sqlite).
func TempSQLite(t *testing.T) *store.SQLite {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Skipf("TempSQLite open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// MemoryBus returns an in-memory event bus registered for cleanup.
func MemoryBus(t *testing.T) *eventbus.Memory {
	t.Helper()
	b := eventbus.NewMemory()
	t.Cleanup(func() { b.Close() })
	return b
}

// ContextWithTimeout returns a context that is cancelled at d
// from now and is cancelled automatically at test cleanup.
func ContextWithTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// OpenTestSQL is a thin helper that opens a *sql.DB against the
// same driver the production store uses, useful for tests that
// need to issue raw SQL (e.g., migration tests).
func OpenTestSQL(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("OpenTestSQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Setenv sets an environment variable and registers cleanup
// that restores the prior value (or removes the var if it was
// unset). Use to scope env mutations to a single test.
func Setenv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// Unsetenv removes an environment variable and registers cleanup
// that restores the prior value (if any).
func Unsetenv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		}
	})
}

// NewTestDB returns a fresh, migrated SQLite database for one
// test. It's the canonical entry point for any test that needs
// the store layer; the migration order is the production order,
// the path is in t.TempDir(), and t.Cleanup closes the DB so
// parallel tests don't leak. TEST-INFRA-1.
//
// The returned *store.SQLite can be used directly OR via the
// *sql.DB returned by (*store.SQLite).DB(). It's an alias for
// TempSQLite kept here so the new-testutil test layer is
// discoverable in one place.
func NewTestDB(t *testing.T) *store.SQLite {
	t.Helper()
	return TempSQLite(t)
}

// ClockFunc is the TEST-INFRA-3 test seam: tests that need a
// deterministic time use the supplied func instead of time.Now().
// Production code uses time.Now directly; the seam only takes
// effect when a test substitutes the package-level var.
var ClockFunc = time.Now

// Now is a drop-in replacement for time.Now that honours the
// ClockFunc test seam. Tests that need deterministic time set
// ClockFunc; production code calls Now() through this seam so
// the substitution is transparent.
func Now() time.Time {
	return ClockFunc()
}

// Counter is a concurrency-safe int counter useful for verifying
// "callback fired N times" assertions.
type Counter struct {
	mu sync.Mutex
	n  int
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

// Value returns the current count.
func (c *Counter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// Spy captures the most recent value passed to Record. Use to
// assert on the last argument a callback received.
type Spy[T any] struct {
	mu  sync.Mutex
	val T
	hit bool
}

// Record stashes the supplied value as the most-recent.
func (s *Spy[T]) Record(v T) {
	s.mu.Lock()
	s.val = v
	s.hit = true
	s.mu.Unlock()
}

// Last returns the most-recently recorded value and whether
// anything was ever recorded.
func (s *Spy[T]) Last() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val, s.hit
}

// ErrSentinel is a stub error useful for tests that need a
// distinguishable error but want to avoid leaking stdlib sentinels.
var ErrSentinel = errors.New("testutil: sentinel")
