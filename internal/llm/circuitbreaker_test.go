package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerStateMachine(t *testing.T) {
	tests := []struct {
		name          string
		config        CircuitBreakerConfig
		actions       []string // "success", "failure", "wait"
		expectedState CircuitState
		expectedError error
	}{
		{
			name: "starts closed",
			config: CircuitBreakerConfig{
				FailureThreshold: 3,
				SuccessThreshold: 2,
				Cooldown:         100 * time.Millisecond,
			},
			actions:       []string{},
			expectedState: StateClosed,
		},
		{
			name: "opens after failures",
			config: CircuitBreakerConfig{
				FailureThreshold: 3,
				SuccessThreshold: 2,
				Cooldown:         100 * time.Millisecond,
			},
			actions:       []string{"failure", "failure", "failure"},
			expectedState: StateOpen,
		},
		{
			name: "transitions to half-open after cooldown",
			config: CircuitBreakerConfig{
				FailureThreshold: 2,
				SuccessThreshold: 2,
				Cooldown:         100 * time.Millisecond,
			},
			actions:       []string{"failure", "failure", "wait", "allow"},
			expectedState: StateHalfOpen,
		},
		{
			name: "closes from half-open after successes",
			config: CircuitBreakerConfig{
				FailureThreshold: 2,
				SuccessThreshold: 2,
				Cooldown:         100 * time.Millisecond,
			},
			actions:       []string{"failure", "failure", "wait", "allow", "success", "success"},
			expectedState: StateClosed,
		},
		{
			name: "reopens from half-open on failure",
			config: CircuitBreakerConfig{
				FailureThreshold: 2,
				SuccessThreshold: 2,
				Cooldown:         100 * time.Millisecond,
			},
			actions:       []string{"failure", "failure", "wait", "allow", "failure"},
			expectedState: StateOpen,
		},
		{
			name: "rejects when open",
			config: CircuitBreakerConfig{
				FailureThreshold: 1,
				SuccessThreshold: 1,
				Cooldown:         1 * time.Hour, // Very long cooldown
			},
			actions:       []string{"failure", "allow"},
			expectedState: StateOpen,
			expectedError: ErrCircuitOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.config)

			for _, action := range tt.actions {
				switch action {
				case "success":
					cb.RecordSuccess()
				case "failure":
					cb.RecordFailure()
				case "wait":
					time.Sleep(tt.config.Cooldown + 50*time.Millisecond)
				case "allow":
					allowed := cb.Allow()
					if tt.expectedError != nil && allowed {
						t.Error("expected request to be rejected")
					}
				}
			}

			if cb.State() != tt.expectedState {
				t.Errorf("state = %v, want %v", cb.State(), tt.expectedState)
			}
		})
	}
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	mock := &mockProvider{
		name: "mock",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "ok"}, nil
		},
	}

	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Cooldown:         50 * time.Millisecond,
	}

	middleware := NewCircuitBreakerMiddleware(mock, config)

	// Test successful call
	resp, err := middleware.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("content = %q, want %q", resp.Content, "ok")
	}

	// Test circuit opens after failures
	mock.completeFunc = func(ctx context.Context, req *Request) (*Response, error) {
		return nil, errors.New("provider error")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		middleware.Complete(context.Background(), &Request{})
	}

	// Circuit should be open now
	_, err = middleware.Complete(context.Background(), &Request{})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}

	// Wait for cooldown
	time.Sleep(config.Cooldown + 10*time.Millisecond)

	// Circuit should be half-open, allow one request
	mock.completeFunc = func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{Content: "recovered"}, nil
	}

	resp, err = middleware.Complete(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("unexpected error after recovery: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("content = %q, want %q", resp.Content, "recovered")
	}

	// Circuit should be closed now
	if middleware.breaker.State() != StateClosed {
		t.Errorf("state = %v, want %v", middleware.breaker.State(), StateClosed)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Cooldown:         time.Hour,
	})

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("state = %v, want %v", cb.State(), StateOpen)
	}

	cb.Reset()
	if cb.State() != StateClosed {
		t.Errorf("state after reset = %v, want %v", cb.State(), StateClosed)
	}
}

type mockProvider struct {
	name         string
	completeFunc func(ctx context.Context, req *Request) (*Response, error)
}

func (m *mockProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
	return m.completeFunc(ctx, req)
}

func (m *mockProvider) Name() string {
	return m.name
}
