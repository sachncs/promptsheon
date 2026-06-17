package llm

import (
	"context"
	"fmt"
	"math"
	"time"
)

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

// Retrying wraps a Provider with exponential-backoff retry logic.
type Retrying struct {
	inner Provider
	cfg   RetryConfig
}

// NewRetrying wraps a provider with retry support.
func NewRetrying(p Provider, cfg RetryConfig) *Retrying {
	return &Retrying{inner: p, cfg: cfg}
}

func (r *Retrying) Name() string { return r.inner.Name() }

func (r *Retrying) Complete(ctx context.Context, req *Request) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
		resp, err := r.inner.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

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

func (t *Timeouting) Name() string { return t.inner.Name() }

func (t *Timeouting) Complete(ctx context.Context, req *Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.inner.Complete(ctx, req)
}
