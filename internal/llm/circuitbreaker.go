package llm

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// StateClosed is the normal operating state.
	StateClosed CircuitState = iota
	// StateOpen is the failure state where requests are rejected.
	StateOpen
	// StateHalfOpen is the testing state after cooldown.
	StateHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit.
	FailureThreshold int
	// SuccessThreshold is the number of successes to close the circuit from half-open.
	SuccessThreshold int
	// Cooldown is the time to wait before transitioning from open to half-open.
	Cooldown time.Duration
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         30 * time.Second,
	}
}

// CircuitBreaker tracks failures and prevents calls when open.
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           CircuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow checks if a request should be allowed through the circuit breaker.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if cooldown has elapsed
		if time.Since(cb.lastFailureTime) >= cb.config.Cooldown {
			cb.state = StateHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.successCount = 0
	}
}

// Reset resets the circuit breaker to its initial state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// CircuitBreakerMiddleware wraps a Provider with circuit breaker logic.
type CircuitBreakerMiddleware struct {
	provider Provider
	breaker  *CircuitBreaker
}

// NewCircuitBreakerMiddleware creates a new circuit breaker middleware.
func NewCircuitBreakerMiddleware(provider Provider, config CircuitBreakerConfig) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		provider: provider,
		breaker:  NewCircuitBreaker(config),
	}
}

// Complete implements Provider and wraps calls with circuit breaker logic.
func (cbm *CircuitBreakerMiddleware) Complete(ctx context.Context, req *Request) (*Response, error) {
	if !cbm.breaker.Allow() {
		return nil, ErrCircuitOpen
	}

	resp, err := cbm.provider.Complete(ctx, req)
	if err != nil {
		cbm.breaker.RecordFailure()
		return nil, err
	}

	cbm.breaker.RecordSuccess()
	return resp, nil
}

// Name returns the underlying provider name.
func (cbm *CircuitBreakerMiddleware) Name() string {
	return cbm.provider.Name()
}

// CircuitBreakerMiddleware satisfies the Provider interface.
var _ Provider = (*CircuitBreakerMiddleware)(nil)
