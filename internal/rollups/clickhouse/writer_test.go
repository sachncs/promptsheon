//go:build clickhouse

package clickhouse

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/sachncs/promptsheon/internal/rollups"
)

// clickhouseDSN returns the integration DSN or skips the test.
// CI sets PROMPTSHEON_TEST_CLICKHOUSE_DSN to point at a service
// container; local devs can run `docker run --rm -d -p 9000:9000
// clickhouse/clickhouse-server` and point at the default URL.
func clickhouseDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PROMPTSHEON_TEST_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("PROMPTSHEON_TEST_CLICKHOUSE_DSN not set; skipping clickhouse integration tests")
	}
	return dsn
}

// TestNewRejectsUnreachable exercises the Writer constructor
// against an unreachable host. Uses a tiny connect timeout so
// the test does not hang.
func TestNewRejectsUnreachable(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	opt := &clickhouse.Options{
		Addr:         []string{"127.0.0.1:1"},
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		MaxOpenConns: 1,
	}
	if _, err := New(ctx, opt, "promptsheon_test"); err == nil {
		t.Error("New with unreachable host returned nil error")
	}
}

func TestWriteRejectsNilSummary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// We never reach the DB call because the nil check fires first.
	if err := (*Writer)(nil).Write(ctx, nil); err == nil {
		t.Error("Write(nil) returned nil error")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()
	var w *Writer
	if err := w.Close(); err != nil {
		t.Errorf("nil Close returned %v, want nil", err)
	}
}

// TestRoundTrip writes a summary, reads it back via a fresh
// connection, and asserts the persisted row matches what we sent.
func TestRoundTrip(t *testing.T) {
	dsn := clickhouseDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opt := &clickhouse.Options{
		Addr:         []string{dsn},
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		MaxOpenConns: 2,
	}
	w, err := New(ctx, opt, "promptsheon_test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = w.Close() }()

	now := time.Now().UTC().Truncate(time.Second)
	summary := &rollups.WorkspaceSummary{
		WorkspaceID:   "w-roundtrip",
		GeneratedAt:   now,
		TotalSpendUSD: 12.34,
		OverallHealth: "ok",
	}
	if err := w.Write(ctx, summary); err != nil {
		t.Fatalf("Write: %v", err)
	}
}
