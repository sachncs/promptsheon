package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"time"
)

// ErrTransient is a marker that an error is worth retrying. Providers
// should wrap retryable failures (5xx, timeouts, connection reset,
// connection refused) with this so the retry classifier can decide
// without resorting to fragile string matching. L-14 replaces the
// previous strings.Contains logic with typed-error checks.
type ErrTransient struct {
	Cause error
}

func (e *ErrTransient) Error() string {
	if e.Cause != nil {
		return "transient: " + e.Cause.Error()
	}
	return "transient"
}

func (e *ErrTransient) Unwrap() error { return e.Cause }

// ErrPermanent is a marker that an error must NOT be retried. Wrap
// permanent failures (4xx auth/validation) with this.
type ErrPermanent struct {
	Cause error
}

func (e *ErrPermanent) Error() string {
	if e.Cause != nil {
		return "permanent: " + e.Cause.Error()
	}
	return "permanent"
}

func (e *ErrPermanent) Unwrap() error { return e.Cause }

// RetryConfig controls retry behavior.
type RetryConfig struct {
	MaxRetries int           `json:"max_retries"`
	BaseDelay  time.Duration `json:"base_delay"`
	MaxDelay   time.Duration `json:"max_delay"`
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   10 * time.Second,
	}
}

// isRetryable determines if an error should be retried. Returns
// false for permanent errors (auth, validation, context cancellation,
// circuit-breaker open) and true for transient errors. L-14: the
// logic now switches on typed errors (ErrTransient, ErrPermanent) and
// on known stdlib types (net.Error.Timeout, *net.OpError with
// Op=="dial"), instead of strings.Contains on the error text. The
// fallback for un-typed errors remains "retry" because the previous
// behaviour was permissive.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Explicit permanent: never retry.
	var perm *ErrPermanent
	if errors.As(err, &perm) {
		return false
	}
	// Explicit transient: always retry.
	var trans *ErrTransient
	if errors.As(err, &trans) {
		return true
	}

	// Don't retry context cancellation.
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Circuit breaker is open: never retry (avoid pile-on).
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}

	// net.Error.Timeout: retry.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// *net.OpError with Op=="dial" (connection refused / DNS):
	// retry. We check the typed op rather than scanning the
	// message text.
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}

	// Default: retry. The previous behaviour was to retry on any
	// unclassified error, which is the right default for a
	// permissive LLM gateway — the cost of one extra attempt is
	// small compared to failing a customer request.
	return true
}

// Retrying wraps a Provider with exponential-backoff retry logic.
type Retrying struct {
	inner Provider
	cfg   RetryConfig
}

// NewRetrying wraps a provider with retry support.
func NewRetrying(p Provider, cfg RetryConfig) *Retrying {
	return &Retrying{inner: p, cfg: cfg}
}

// Name returns the wrapped provider name.
func (r *Retrying) Name() string { return r.inner.Name() }

// Retrying satisfies the Provider interface.
var _ Provider = (*Retrying)(nil)

// Timeouting satisfies the Provider interface.
var _ Provider = (*Timeouting)(nil)

// Complete sends a request with exponential-backoff retry logic.
func (r *Retrying) Complete(ctx context.Context, req *Request) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		resp, err := r.inner.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Don't retry if error is not retryable
		if !isRetryable(err) {
			return nil, err
		}

		if attempt == r.cfg.MaxRetries {
			break
		}

		delay := r.delay(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("after %d retries: %w", r.cfg.MaxRetries, lastErr)
}

func (r *Retrying) delay(attempt int) time.Duration {
	delay := float64(r.cfg.BaseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(r.cfg.MaxDelay) {
		delay = float64(r.cfg.MaxDelay)
	}
	return time.Duration(delay)
}

// Timeouting wraps a Provider with a per-call timeout.
type Timeouting struct {
	inner   Provider
	timeout time.Duration
}

// NewTimeouting wraps a provider with a per-call timeout.
func NewTimeouting(p Provider, timeout time.Duration) *Timeouting {
	return &Timeouting{inner: p, timeout: timeout}
}

// Name returns the wrapped provider name.
func (t *Timeouting) Name() string { return t.inner.Name() }

// Complete sends a request with a per-call timeout.
func (t *Timeouting) Complete(ctx context.Context, req *Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.inner.Complete(ctx, req)
}
