package llm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"promptsheon/internal/models"
)

func TestMockProvider(t *testing.T) {
	mock := NewMock("hello world")
	if mock.Name() != "mock" {
		t.Fatalf("expected name 'mock', got %q", mock.Name())
	}

	resp, err := mock.Complete(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Fatalf("expected 'hello world', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Fatalf("expected 30 total tokens, got %d", resp.Usage.TotalTokens)
	}
	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.CallCount())
	}
	if mock.LastCall().Model != "test-model" {
		t.Fatalf("expected model 'test-model', got %q", mock.LastCall().Model)
	}
}

func TestMockError(t *testing.T) {
	mock := NewMock("")
	mock.Error = errors.New("connection refused")

	_, err := mock.Complete(context.Background(), &Request{Model: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection refused" {
		t.Fatalf("expected 'connection refused', got %q", err.Error())
	}
	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call even on error, got %d", mock.CallCount())
	}
}

func TestRegistry(t *testing.T) {
	r := newRegistry()

	// Unknown provider
	_, err := r.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	// Register custom provider
	r.Register("custom", func(cfg ProviderConfig) Provider {
		return NewMock("custom-response")
	})
	r.Configure("custom", ProviderConfig{})

	p, err := r.Get("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "mock" {
		t.Fatalf("expected mock provider, got %q", p.Name())
	}

	// List providers
	names := r.Providers()
	found := false
	for _, n := range names {
		if n == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'custom' in providers list")
	}
}

func TestRegistryWithConfig(t *testing.T) {
	r := newRegistry()
	r.Configure("openai", ProviderConfig{APIKey: "test-key"})

	p, err := r.Get("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("expected 'openai', got %q", p.Name())
	}
}

func TestInstrumented(t *testing.T) {
	mock := NewMock("test")
	agg := NewAggregateMetrics()

	var collected []CallMetrics
	inst := NewInstrumented(mock, func(m CallMetrics) {
		collected = append(collected, m)
	}, nil)

	resp, err := inst.Complete(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "test" {
		t.Fatalf("expected 'test', got %q", resp.Content)
	}
	if len(collected) != 1 {
		t.Fatalf("expected 1 collected metric, got %d", len(collected))
	}
	if collected[0].Provider != "mock" {
		t.Fatalf("expected provider 'mock', got %q", collected[0].Provider)
	}
	if collected[0].Model != "gpt-4o" {
		t.Fatalf("expected model 'gpt-4o', got %q", collected[0].Model)
	}

	// Aggregate metrics
	agg.Collect()(collected[0])
	if agg.TotalCalls != 1 {
		t.Fatalf("expected 1 total call, got %d", agg.TotalCalls)
	}
}

func TestAggregateMetrics(t *testing.T) {
	agg := NewAggregateMetrics()
	collect := agg.Collect()

	collect(CallMetrics{Provider: "openai", Model: "gpt-4o", Usage: models.Usage{TotalTokens: 100}, CostUSD: 0.001, Latency: 100 * time.Millisecond})
	collect(CallMetrics{Provider: "openai", Model: "gpt-4o-mini", Usage: models.Usage{TotalTokens: 200}, CostUSD: 0.0005, Latency: 50 * time.Millisecond})
	collect(CallMetrics{Provider: "ollama", Model: "llama3", Usage: models.Usage{TotalTokens: 150}, CostUSD: 0, Latency: 200 * time.Millisecond})

	if agg.TotalCalls != 3 {
		t.Fatalf("expected 3 total calls, got %d", agg.TotalCalls)
	}
	if agg.TotalTokens != 450 {
		t.Fatalf("expected 450 total tokens, got %d", agg.TotalTokens)
	}
	if agg.ByProvider["openai"].Calls != 2 {
		t.Fatalf("expected 2 openai calls, got %d", agg.ByProvider["openai"].Calls)
	}
	if agg.ByProvider["ollama"].Calls != 1 {
		t.Fatalf("expected 1 ollama call, got %d", agg.ByProvider["ollama"].Calls)
	}
	if agg.ByModel["gpt-4o"].Tokens != 100 {
		t.Fatalf("expected 100 tokens for gpt-4o, got %d", agg.ByModel["gpt-4o"].Tokens)
	}
}

func TestCostCalculation(t *testing.T) {
	usage := models.Usage{PromptTokens: 1000, CompletionTokens: 500}
	cost := CalculateCost("gpt-4o", usage)
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}

	// Unknown model returns 0
	cost = CalculateCost("unknown-model", usage)
	if cost != 0 {
		t.Fatalf("expected 0 cost for unknown model, got %f", cost)
	}

	// Ollama is free
	cost = CalculateCost("llama3", usage)
	if cost != 0 {
		t.Fatalf("expected 0 cost for llama3, got %f", cost)
	}
}

func TestGetPricing(t *testing.T) {
	p := GetPricing("gpt-4o")
	if p == nil {
		t.Fatal("expected pricing for gpt-4o")
	}
	if p.PromptPerToken <= 0 {
		t.Fatalf("expected positive prompt price, got %f", p.PromptPerToken)
	}

	p = GetPricing("nonexistent")
	if p != nil {
		t.Fatal("expected nil for nonexistent model")
	}
}

func TestRetryExhaustion(t *testing.T) {
	calls := 0
	inner := &failAfterProvider{
		failUntil: 5,
		calls:     &calls,
	}
	retrying := NewRetrying(inner, RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond})

	_, err := retrying.Complete(context.Background(), &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
}

func TestRetrySuccess(t *testing.T) {
	calls := 0
	inner := &failAfterProvider{
		failUntil: 1,
		calls:     &calls,
	}
	retrying := NewRetrying(inner, RetryConfig{MaxRetries: 3, BaseDelay: time.Millisecond})

	resp, err := retrying.Complete(context.Background(), &Request{Model: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected 'ok', got %q", resp.Content)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (1 fail + 1 success), got %d", calls)
	}
}

func TestTimeout(t *testing.T) {
	slow := &slowProvider{delay: 5 * time.Millisecond}
	timeouting := NewTimeouting(slow, 1*time.Millisecond)

	_, err := retryingComplete(timeouting, context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRetryContextCancellation(t *testing.T) {
	inner := NewMock("")
	inner.Error = errors.New("retryable")
	retrying := NewRetrying(inner, RetryConfig{MaxRetries: 5, BaseDelay: time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := retrying.Complete(ctx, &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// --- helpers ---

func retryingComplete(p Provider, ctx context.Context) (*Response, error) {
	return p.Complete(ctx, &Request{Model: "test"})
}

type failAfterProvider struct {
	failUntil int
	calls     *int
}

func (f *failAfterProvider) Name() string { return "fail-after" }
func (f *failAfterProvider) Complete(_ context.Context, _ *Request) (*Response, error) {
	*f.calls++
	if *f.calls <= f.failUntil {
		return nil, errors.New("transient error")
	}
	return &Response{Content: "ok", Usage: models.Usage{TotalTokens: 1}}, nil
}

type slowProvider struct {
	delay time.Duration
}

func (s *slowProvider) Name() string { return "slow" }
func (s *slowProvider) Complete(ctx context.Context, _ *Request) (*Response, error) {
	select {
	case <-time.After(s.delay):
		return &Response{Content: "done"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "circuit breaker open",
			err:      ErrCircuitOpen,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: true,
		},
		{
			name:     "wrapped context error",
			err:      fmt.Errorf("wrapped: %w", context.Canceled),
			expected: false,
		},
		{
			name:     "wrapped circuit breaker error",
			err:      fmt.Errorf("wrapped: %w", ErrCircuitOpen),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.expected {
				t.Errorf("isRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}
