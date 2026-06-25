package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestIsRetryable_TypedErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ErrTransient", &ErrTransient{Cause: errors.New("x")}, true},
		{"ErrPermanent", &ErrPermanent{Cause: errors.New("x")}, false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"ErrCircuitOpen", ErrCircuitOpen, false},
		{"net.Timeout", &timeoutErr{}, true},
		{"wrapped ErrTransient", fmt.Errorf("wrap: %w", &ErrTransient{Cause: errors.New("x")}), true},
		{"wrapped ErrPermanent", fmt.Errorf("wrap: %w", &ErrPermanent{Cause: errors.New("x")}), false},
		{"unknown error (default retry)", errors.New("random failure"), true},
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
