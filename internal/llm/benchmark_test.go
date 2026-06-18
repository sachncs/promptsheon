package llm

import (
	"context"
	"testing"
)

func BenchmarkCircuitBreakerAllow(b *testing.B) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         30,
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Allow()
	}
}

func BenchmarkCircuitBreakerRecordSuccess(b *testing.B) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         30,
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordSuccess()
	}
}

func BenchmarkCircuitBreakerRecordFailure(b *testing.B) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         30,
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordFailure()
	}
}

func BenchmarkCircuitBreakerMiddleware(b *testing.B) {
	mock := &mockProvider{
		name: "mock",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "ok"}, nil
		},
	}

	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Cooldown:         30,
	}

	middleware := NewCircuitBreakerMiddleware(mock, config)
	ctx := context.Background()
	req := &Request{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.Complete(ctx, req)
	}
}

func BenchmarkRetryingComplete(b *testing.B) {
	mock := &mockProvider{
		name: "mock",
		completeFunc: func(ctx context.Context, req *Request) (*Response, error) {
			return &Response{Content: "ok"}, nil
		},
	}

	cfg := DefaultRetryConfig()
	retrying := NewRetrying(mock, cfg)
	ctx := context.Background()
	req := &Request{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retrying.Complete(ctx, req)
	}
}
