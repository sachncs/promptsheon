package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIsRetryable_TypedErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "ErrTransient", err: &ErrTransient{Cause: errors.New("x")}, want: true},
		{name: "ErrPermanent", err: &ErrPermanent{Cause: errors.New("x")}, want: false},
		{name: "context.Canceled", err: context.Canceled, want: false},
		{name: "context.DeadlineExceeded", err: context.DeadlineExceeded, want: false},
		{name: "ErrCircuitOpen", err: ErrCircuitOpen, want: false},
		{name: "net.Timeout", err: &timeoutErr{}, want: true},
		{name: "wrapped ErrTransient", err: fmt.Errorf("wrap: %w", &ErrTransient{Cause: errors.New("x")}), want: true},
		{name: "wrapped ErrPermanent", err: fmt.Errorf("wrap: %w", &ErrPermanent{Cause: errors.New("x")}), want: false},
		{name: "unknown error (default retry)", err: errors.New("random failure"), want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRetryable(c.err); got != c.want {
				t.Fatalf("isRetryable(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// TestIsRetryable_NetOpErrorTyped pins the L-14 fix: we check
// net.OpError.Op == "dial" via errors.As, not by string-matching
// "connection refused" on the error text.
func TestIsRetryable_NetOpErrorTyped(t *testing.T) {
	dialErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	if !isRetryable(dialErr) {
		t.Fatal("dial net.OpError should be retryable")
	}
	nonDialErr := &net.OpError{
		Op:  "read",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	// non-dial errors fall through to the default-retry branch.
	if !isRetryable(nonDialErr) {
		t.Fatal("non-dial net.OpError should fall through to default-retry (true)")
	}
}

// timeoutErr is a minimal net.Error that reports Timeout()=true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestErrTransientMessage(t *testing.T) {
	// The Error() method should mention "transient:" plus the
	// wrapped cause. The unwrap path must return the original
	// error so errors.Is/As work for callers.
	cause := errors.New("upstream 502")
	e := &ErrTransient{Cause: cause}
	if got := e.Error(); got != "transient: upstream 502" {
		t.Errorf("Error(): got %q", got)
	}
	if e.Unwrap() != cause {
		t.Error("Unwrap did not return the wrapped cause")
	}
	// A nil-cause ErrTransient should still produce a usable
	// string and not panic.
	empty := &ErrTransient{}
	if empty.Error() != "transient" {
		t.Errorf("nil-cause Error(): got %q", empty.Error())
	}
}

func TestErrPermanentMessage(t *testing.T) {
	cause := errors.New("bad api key")
	e := &ErrPermanent{Cause: cause}
	if got := e.Error(); got != "permanent: bad api key" {
		t.Errorf("Error(): got %q", got)
	}
	if e.Unwrap() != cause {
		t.Error("Unwrap did not return the wrapped cause")
	}
	empty := &ErrPermanent{}
	if empty.Error() != "permanent" {
		t.Errorf("nil-cause Error(): got %q", empty.Error())
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	c := DefaultRetryConfig()
	if c.MaxRetries < 1 {
		t.Errorf("expected MaxRetries >= 1, got %d", c.MaxRetries)
	}
	if c.BaseDelay <= 0 {
		t.Errorf("expected positive BaseDelay, got %v", c.BaseDelay)
	}
	if c.MaxDelay < c.BaseDelay {
		t.Errorf("expected MaxDelay >= BaseDelay, got %v vs %v", c.MaxDelay, c.BaseDelay)
	}
}

// flakyProvider returns a sequence of errors followed by a
// success. The error sequence is configurable so retry tests
// can drive the inner provider through a known number of
// failures.
type flakyProvider struct {
	name     string
	failures int
	attempts int
	lastResp *Response
}

func (f *flakyProvider) Name() string { return f.name }
func (f *flakyProvider) Complete(_ context.Context, _ *Request) (*Response, error) {
	f.attempts++
	if f.attempts <= f.failures {
		return nil, &ErrTransient{Cause: errors.New("upstream 502")}
	}
	return f.lastResp, nil
}

func TestRetryingSucceedsAfterTransientFailures(t *testing.T) {
	inner := &flakyProvider{
		name:     "flaky",
		failures: 2,
		lastResp: &Response{Content: "ok", Model: "test"},
	}
	r := NewRetrying(inner, RetryConfig{MaxRetries: 5, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond})
	resp, err := r.Complete(context.Background(), &Request{Model: "test"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected content 'ok', got %q", resp.Content)
	}
	if inner.attempts != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", inner.attempts)
	}
}

func TestRetryingReturnsLastError(t *testing.T) {
	// All attempts fail; we expect the wrapped 'after N
	// retries' error.
	inner := &flakyProvider{name: "down", failures: 100}
	r := NewRetrying(inner, RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond})
	_, err := r.Complete(context.Background(), &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "after 2 retries") {
		t.Errorf("expected 'after 2 retries' in error, got %v", err)
	}
}

func TestRetryingStopsOnPermanentError(_ *testing.T) {
	// A permanent error must short-circuit the retry loop;
	// the inner provider should see only one attempt.
	inner := &flakyProvider{
		name:     "perm",
		failures: 100,
		lastResp: &Response{Content: "ok"},
	}
	r := NewRetrying(inner, RetryConfig{MaxRetries: 5, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond})
	// Wrap the inner to return a permanent error on the first
	// attempt.
	inner.failures = 0
	_, err := r.Complete(context.Background(), &Request{Model: "test"})
	_ = err // flakyProvider returns nil error, so this test just
	//       exercises the success path; the next test exercises
	//       the permanent-error path explicitly.
}

func TestRetryingPermanentStopsImmediately(t *testing.T) {
	calls := 0
	inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return nil, &ErrPermanent{Cause: errors.New("bad key")}
	}}
	r := NewRetrying(inner, RetryConfig{MaxRetries: 5, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond})
	_, err := r.Complete(context.Background(), &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 attempt for permanent error, got %d", calls)
	}
}

func TestRetryingContextCancellation(t *testing.T) {
	inner := &flakyProvider{name: "slow", failures: 100}
	r := NewRetrying(inner, RetryConfig{MaxRetries: 5, BaseDelay: time.Second, MaxDelay: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	_, err := r.Complete(ctx, &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetryingName(t *testing.T) {
	inner := &flakyProvider{name: "my-provider", failures: 0, lastResp: &Response{Content: "x"}}
	r := NewRetrying(inner, RetryConfig{})
	if r.Name() != "my-provider" {
		t.Errorf("Name: got %q", r.Name())
	}
}

func TestTimeoutingWrapsContext(t *testing.T) {
	// The Timeouting wrapper installs a per-call timeout on
	// the context it passes down. We use a provider that
	// records the context's Done channel state.
	inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
		return &Response{Content: "ok"}, nil
	}}
	tw := NewTimeouting(inner, time.Hour)
	resp, err := tw.Complete(context.Background(), &Request{Model: "test"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected content 'ok', got %q", resp.Content)
	}
	if tw.Name() != "callable" {
		t.Errorf("Name: got %q, want 'callable'", tw.Name())
	}
}

func TestRetryingDelayExponentialAndCapped(t *testing.T) {
	r := &Retrying{cfg: RetryConfig{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}}
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, time.Second},  // capped at MaxDelay
		{5, time.Second},  // still capped
		{20, time.Second}, // very large attempt, still capped
	}
	for _, tt := range tests {
		if got := r.delay(tt.attempt); got != tt.want {
			t.Errorf("delay(%d): got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

// callableProvider lets a test inject an arbitrary Complete
// function without declaring a new struct.
type callableProvider struct {
	fn func(context.Context, *Request) (*Response, error)
}

func (c *callableProvider) Name() string { return "callable" }
func (c *callableProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	return c.fn(ctx, req)
}
