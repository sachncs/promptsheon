package testutil_test

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/testutil"
)

func TestDiscardLoggerReturnsLogger(t *testing.T) {
	t.Parallel()
	l := testutil.DiscardLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	l.Info("test", "k", "v")
}

func TestMemoryBusReturnsBus(t *testing.T) {
	t.Parallel()
	b := testutil.MemoryBus(t)
	if b == nil {
		t.Fatal("expected non-nil bus")
	}
	sub, err := b.Subscribe(func(_ capability.Event) {})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	sub.Cancel()
}

func TestContextWithTimeoutCancels(t *testing.T) {
	t.Parallel()
	ctx := testutil.ContextWithTimeout(t, 50*time.Millisecond)
	select {
	case <-ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context did not cancel within 500ms")
	}
}

func TestCounter(t *testing.T) {
	t.Parallel()
	c := &testutil.Counter{}
	c.Inc()
	c.Inc()
	c.Inc()
	if got := c.Value(); got != 3 {
		t.Errorf("Counter.Value = %d, want 3", got)
	}
}

func TestSpy(t *testing.T) {
	t.Parallel()
	s := &testutil.Spy[int]{}
	if _, hit := s.Last(); hit {
		t.Error("Spy.Last should report hit=false before Record")
	}
	s.Record(42)
	v, hit := s.Last()
	if !hit || v != 42 {
		t.Errorf("Spy.Last = (%d, %v), want (42, true)", v, hit)
	}
}

func TestSetenvAndUnsetenv(t *testing.T) {
	t.Setenv("PROMPTSHEON_TESTUTIL_KEY", "first")
	testutil.Setenv(t, "PROMPTSHEON_TESTUTIL_KEY", "second")
	if v := os.Getenv("PROMPTSHEON_TESTUTIL_KEY"); v != "second" {
		t.Errorf("Setenv failed to apply: got %q", v)
	}
	testutil.Unsetenv(t, "PROMPTSHEON_TESTUTIL_KEY")
	if v := os.Getenv("PROMPTSHEON_TESTUTIL_KEY"); v != "" {
		t.Errorf("Unsetenv failed to remove: got %q", v)
	}
}

func TestErrSentinel(t *testing.T) {
	t.Parallel()
	if testutil.ErrSentinel == nil {
		t.Error("expected non-nil sentinel")
	}
	if testutil.ErrSentinel.Error() == "" {
		t.Error("sentinel should have a message")
	}
}

func TestTempSQLiteOpens(t *testing.T) {
	t.Parallel()
	s := testutil.TempSQLite(t)
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestOpenTestSQLOpens(t *testing.T) {
	t.Parallel()
	db := testutil.OpenTestSQL(t, ":memory:")
	if db == nil {
		t.Fatal("expected non-nil DB")
	}
	_ = io.Discard
}
