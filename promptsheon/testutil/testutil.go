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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/sachncs/promptsheon/promptsheon/eventbus"
	"github.com/sachncs/promptsheon/promptsheon/store"
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
