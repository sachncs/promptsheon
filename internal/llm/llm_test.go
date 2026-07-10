package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
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
	r := NewRegistry()

	// Unknown provider
	_, err := r.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	// Register custom provider
	r.Register("custom", func(_ ProviderConfig) Provider {
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
	r := NewRegistry()
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

	collect(CallMetrics{Provider: "openai", Model: "gpt-4o", Usage: Usage{TotalTokens: 100}, CostUSD: 0.001, Latency: 100 * time.Millisecond})
	collect(CallMetrics{Provider: "openai", Model: "gpt-4o-mini", Usage: Usage{TotalTokens: 200}, CostUSD: 0.0005, Latency: 50 * time.Millisecond})
	collect(CallMetrics{Provider: "ollama", Model: "llama3", Usage: Usage{TotalTokens: 150}, CostUSD: 0, Latency: 200 * time.Millisecond})

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
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500}
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

	_, err := retryingComplete(context.Background(), timeouting)
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

func retryingComplete(ctx context.Context, p Provider) (*Response, error) {
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
	return &Response{Content: "ok", Usage: Usage{TotalTokens: 1}}, nil
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

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PROMPTSHEON_OPENAI_API_KEY", "sk-test")
	t.Setenv("PROMPTSHEON_ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("PROMPTSHEON_OLLAMA_BASE_URL", "http://localhost:11434")
	t.Setenv("PROMPTSHEON_AZURE_API_KEY", "sk-azure-test")
	t.Setenv("PROMPTSHEON_NVIDIA_API_KEY", "nv-test")
	t.Setenv("PROMPTSHEON_LLM_PROVIDER", "openai")

	r := NewRegistry()
	provider := r.LoadFromEnv()
	if provider != "openai" {
		t.Fatalf("expected 'openai', got %q", provider)
	}

	for _, name := range []string{"openai", "anthropic", "ollama", "azure", "nvidia"} {
		p, err := r.Get(name)
		if err != nil {
			t.Fatalf("r.Get(%q): %v", name, err)
		}
		if p.Name() != name {
			t.Errorf("r.Get(%q).Name() = %q, want %q", name, p.Name(), name)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty string: got %d, want 0", got)
	}
	if got := EstimateTokens("hello"); got <= 0 {
		t.Errorf("short string: got %d, want > 0", got)
	}
	long := strings.Repeat("hello world ", 100)
	if got := EstimateTokens(long); got <= 0 {
		t.Errorf("long string: got %d, want > 0", got)
	}
	punct := "... !!! ??? ,,, ;;;"
	if got := EstimateTokens(punct); got <= 0 {
		t.Errorf("punctuation-heavy: got %d, want > 0", got)
	}
}

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost(100, 50, "gpt-4o")
	if cost <= 0 {
		t.Errorf("expected cost > 0 for gpt-4o, got %f", cost)
	}
	cost = EstimateCost(100, 50, "unknown-model")
	if cost != 0 {
		t.Errorf("expected cost 0 for unknown model, got %f", cost)
	}
}

func TestCircuitStateString(t *testing.T) {
	if StateClosed.String() != "closed" {
		t.Errorf("StateClosed: got %q", StateClosed.String())
	}
	if StateOpen.String() != "open" {
		t.Errorf("StateOpen: got %q", StateOpen.String())
	}
	if StateHalfOpen.String() != "half-open" {
		t.Errorf("StateHalfOpen: got %q", StateHalfOpen.String())
	}
	if CircuitState(99).String() != "unknown" {
		t.Errorf("CircuitState(99): got %q", CircuitState(99).String())
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("FailureThreshold: got %d, want 5", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 3 {
		t.Errorf("SuccessThreshold: got %d, want 3", cfg.SuccessThreshold)
	}
	if cfg.Cooldown != 30*time.Second {
		t.Errorf("Cooldown: got %v, want 30s", cfg.Cooldown)
	}
}

func TestInstrumentedName(t *testing.T) {
	mock := NewMock("test")
	inst := NewInstrumented(mock, nil, nil)
	if inst.Name() != "mock" {
		t.Errorf("Instrumented.Name() = %q, want %q", inst.Name(), "mock")
	}
}

func TestCircuitBreakerMiddlewareName(t *testing.T) {
	mock := NewMock("test")
	cbm := NewCircuitBreakerMiddleware(mock, DefaultCircuitBreakerConfig())
	if cbm.Name() != "mock" {
		t.Errorf("CircuitBreakerMiddleware.Name() = %q, want %q", cbm.Name(), "mock")
	}
}

func TestMockLastCallEmpty(t *testing.T) {
	mock := NewMock("test")
	if last := mock.LastCall(); last != nil {
		t.Fatal("expected nil LastCall on mock with no calls")
	}
}

func TestInstrumentedError(t *testing.T) {
	mock := NewMock("")
	mock.Error = errors.New("provider failure")
	var collected []CallMetrics
	inst := NewInstrumented(mock, func(m CallMetrics) {
		collected = append(collected, m)
	}, nil)

	_, err := inst.Complete(context.Background(), &Request{Model: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(collected) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(collected))
	}
	if collected[0].Error != "provider failure" {
		t.Errorf("expected error metric, got %v", collected[0].Error)
	}
}

func TestInstrumentedWithLogger(t *testing.T) {
	mock := NewMock("test")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	inst := NewInstrumented(mock, nil, logger)

	resp, err := inst.Complete(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "test" {
		t.Errorf("expected 'test', got %q", resp.Content)
	}
}

func TestCircuitBreakerAllowEdgeCases(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Cooldown:         time.Hour,
	})

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatal("expected open state")
	}
	if cb.Allow() {
		t.Error("expected Allow to return false when open and cooldown not elapsed")
	}

	cb.state = StateHalfOpen
	if !cb.Allow() {
		t.Error("expected Allow to return true in half-open state")
	}

	cb.state = CircuitState(99)
	if cb.Allow() {
		t.Error("expected Allow to return false for invalid state")
	}
}
