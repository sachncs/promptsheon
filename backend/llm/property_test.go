// Property-based tests for the LLM gateway. The library used
// (testing/quick) is the standard-library property harness; this
// file pins the behaviour of the registry, the instrumented
// middleware, the retry loop, and the fallback chain against
// arbitrary inputs.
//
// All fixtures are local: the existing Mock + flakyProvider +
// callableProvider mocks plus the scriptedTransport in
// testinfra_test.go. No external HTTP pulls, no SDK dependency.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	rand "math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"testing/quick"
	"time"
)

// arbClampedInt returns a printable generator limited to [0, N)
// so the per-iteration wall time stays bounded across the
// retry/fallback properties. The Generator interface uses
// math/rand (v1), so we match its signature.
type arbClampedInt struct {
	N int
}

func (a arbClampedInt) Generate(rng *rand.Rand, _ int) reflect.Value {
	bound := a.N
	if bound <= 0 {
		bound = 64
	}
	return reflect.ValueOf(arbClampedInt{N: rng.Intn(bound) + 1})
}

func TestArbClampedIntGeneratesPositiveValues(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		if got := (arbClampedInt{}).Generate(rng, 0).Interface().(arbClampedInt).N; got <= 0 {
			t.Fatalf("sample %d = %d, want positive", i, got)
		}
	}
}

// ---- Registry -------------------------------------------------------------

// TestProperty_RegistryGetReturnsSameInstance asserts that the
// cache returns the same provider instance for repeated calls
// with the same name. The cache is what makes repeated
// constructions cheap (no per-call HTTP client); an
// always-rebuilds implementation would be a regression.
func TestProperty_RegistryGetReturnsSameInstance(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		r := NewRegistry()
		r.Configure("openai", ProviderConfig{APIKey: "sk-test"})
		p1, err := r.Get("openai")
		if err != nil {
			t.Logf("Get: %v", err)
			return false
		}
		p2, err := r.Get("openai")
		if err != nil {
			t.Logf("Get: %v", err)
			return false
		}
		return p1 == p2
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RegistryConfigureInvalidatesInstance asserts that
// re-Configuring a provider yields a fresh instance. The cache
// must drop the old instantiation so the new config takes
// effect.
func TestProperty_RegistryConfigureInvalidatesInstance(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		r := NewRegistry()
		r.Configure("openai", ProviderConfig{APIKey: "sk-old"})
		old, err := r.Get("openai")
		if err != nil {
			t.Logf("first Get: %v", err)
			return false
		}
		r.Configure("openai", ProviderConfig{APIKey: "sk-new"})
		neu, err := r.Get("openai")
		if err != nil {
			t.Logf("second Get: %v", err)
			return false
		}
		return old != neu
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RegistryUnknownProviderRejected asserts that a
// provider that has neither factory nor configuration returns
// an error. This is the contract callers rely on for fast
// startup validation.
func TestProperty_RegistryUnknownProviderRejected(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		r := NewRegistry()
		if _, err := r.Get("nope"); err == nil {
			t.Log("Get(unknown) returned nil error")
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RegistryUnconfiguredProviderRejected asserts that
// a registered factory must be configured before Get returns a
// provider. The factory exists; the config does not.
func TestProperty_RegistryUnconfiguredProviderRejected(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		r := NewRegistry()
		// openai is pre-registered by NewRegistry but not
		// configured.
		if _, err := r.Get("openai"); err == nil {
			t.Log("Get(unconfigured) returned nil error")
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RegistryCacheSurvivesRepeatedGet asserts that
// repeated Get calls produce the same instance across many
// accesses. The cache is what makes repeated constructions
// cheap, and the property test runs enough iterations to catch
// any race-window bug that the existing concurrent test would
// also catch. We deliberately do not spawn goroutines here:
// the existing TestRegistryConcurrentGet covers the concurrent
// path, and a property test that spawns goroutines under
// quick.Check tends to overload the scheduler on small
// machines.
func TestProperty_RegistryCacheSurvivesRepeatedGet(t *testing.T) {
	f := func(reps arbClampedInt) bool {
		r := NewRegistry()
		r.Register("mock", func(_ ProviderConfig) Provider {
			return &mockProvider{name: "mock"}
		})
		r.Configure("mock", ProviderConfig{APIKey: "sk-conc"})
		first, err := r.Get("mock")
		if err != nil {
			t.Logf("Get: %v", err)
			return false
		}
		if first == nil {
			t.Log("first Get returned nil")
			return false
		}
		for i := 0; i < reps.N; i++ {
			again, err := r.Get("mock")
			if err != nil {
				t.Logf("Get: %v", err)
				return false
			}
			if again != first {
				t.Logf("rep %d: %p vs %p", i, again, first)
				return false
			}
		}
		return true
	}
	cfg := &quick.Config{MaxCount: 50}
	if err := quick.Check(f, cfg); err != nil {
		t.Fatal(err)
	}
}

// ---- Instrumented middleware ---------------------------------------------

// TestProperty_InstrumentedPreservesResponse asserts that the
// Instrumented wrapper returns the inner Response unchanged. The
// wrapper must not mutate the response; any field it changes
// would silently corrupt clients.
func TestProperty_InstrumentedPreservesResponse(t *testing.T) {
	t.Parallel()
	f := func(content string) bool {
		expected := &Response{
			Content:    content,
			Usage:      Usage{TotalTokens: 42},
			Model:      "unit",
			StopReason: "stop",
		}
		inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			return expected, nil
		}}
		var got CallMetrics
		inst := NewInstrumented(inner, func(m CallMetrics) { got = m }, nil, nil)
		gotResp, err := inst.Complete(context.Background(), &Request{Model: "unit"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		if gotResp != expected {
			t.Log("response pointer was rewritten by middleware")
			return false
		}
		if got.Error != "" {
			t.Logf("metrics recorded error on success: %q", got.Error)
			return false
		}
		if got.Usage.TotalTokens != 42 {
			t.Logf("metrics.Usage.TotalTokens = %d, want 42", got.Usage.TotalTokens)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_InstrumentedPreservesError asserts that the
// Instrumented wrapper returns the inner error unchanged. The
// error is the only signal a caller has to retry or fall back;
// re-wrapping it would break error chain inspection.
func TestProperty_InstrumentedPreservesError(t *testing.T) {
	t.Parallel()
	sentinel := &ErrPermanent{Cause: errors.New("sentinel")}
	f := func(_ arbClampedInt) bool {
		inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			return nil, sentinel
		}}
		var got CallMetrics
		inst := NewInstrumented(inner, func(m CallMetrics) { got = m }, nil, nil)
		_, err := inst.Complete(context.Background(), &Request{Model: "unit"})
		if !errors.Is(err, sentinel) {
			t.Logf("error chain lost: %v", err)
			return false
		}
		if !errors.Is(err, sentinel) {
			t.Log("errors.Is returned false on the returned error")
			return false
		}
		if got.Error == "" {
			t.Log("metrics did not record the error message")
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_InstrumentedEmitsExactlyOneMetric asserts that
// the wrapper emits exactly one CallMetrics per Complete call,
// regardless of success or failure. The metrics aggregator
// relies on this to count calls; double-emit would inflate the
// rate.
func TestProperty_InstrumentedEmitsExactlyOneMetric(t *testing.T) {
	t.Parallel()
	f := func(fail bool) bool {
		inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			if fail {
				return nil, errors.New("boom")
			}
			return &Response{Content: "ok", Usage: Usage{TotalTokens: 1}}, nil
		}}
		var n atomic.Int64
		inst := NewInstrumented(inner, func(CallMetrics) { n.Add(1) }, nil, nil)
		_, _ = inst.Complete(context.Background(), &Request{Model: "unit"})
		return n.Load() == 1
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// ---- Retry ----------------------------------------------------------------

// TestProperty_RetrySucceedsAfterTransientFailures asserts that
// the retry loop resolves a transient failure within MaxRetries
// and stops immediately on success. attempts = failures + 1.
func TestProperty_RetrySucceedsAfterTransientFailures(t *testing.T) {
	t.Parallel()
	f := func(failures arbClampedInt) bool {
		k := failures.N
		budget := k + 4 // 4 should be plenty for any k in [0, 63]
		inner := &flakyProvider{
			name:     "flaky",
			failures: k,
			lastResp: &Response{Content: "ok"},
		}
		r := NewRetrying(inner, RetryConfig{
			MaxRetries: budget,
			BaseDelay:  10 * time.Microsecond,
			MaxDelay:   10 * time.Microsecond,
		})
		resp, err := r.Complete(context.Background(), &Request{Model: "test"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		if resp.Content != "ok" {
			t.Logf("content = %q", resp.Content)
			return false
		}
		if inner.attempts != k+1 {
			t.Logf("attempts = %d, want %d", inner.attempts, k+1)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RetryBoundsAttempts asserts that the retry loop
// never makes more than MaxRetries + 1 attempts. Total attempts
// is the budget the wrapper exposes; exceeding it would breach
// the contract.
func TestProperty_RetryBoundsAttempts(t *testing.T) {
	t.Parallel()
	f := func(c arbClampedInt) bool {
		budget := c.N
		if budget < 0 {
			return true
		}
		inner := &flakyProvider{name: "down", failures: 1000}
		r := NewRetrying(inner, RetryConfig{
			MaxRetries: budget,
			BaseDelay:  10 * time.Microsecond,
			MaxDelay:   10 * time.Microsecond,
		})
		_, _ = r.Complete(context.Background(), &Request{Model: "test"})
		return inner.attempts == budget+1
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RetryPermanentErrorSingleAttempt asserts that a
// permanent error short-circuits the retry loop after exactly
// one attempt. The middleware's documentation contract says
// "never retry 4xx"; the property pins that.
func TestProperty_RetryPermanentErrorSingleAttempt(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		var calls atomic.Int64
		inner := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			calls.Add(1)
			return nil, &ErrPermanent{Cause: errors.New("bad key")}
		}}
		r := NewRetrying(inner, RetryConfig{
			MaxRetries: 5,
			BaseDelay:  10 * time.Microsecond,
			MaxDelay:   10 * time.Microsecond,
		})
		_, err := r.Complete(context.Background(), &Request{Model: "test"})
		if err == nil {
			t.Log("expected error")
			return false
		}
		return calls.Load() == 1
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_RetryContextCancellationStops asserts that a
// cancelled context stops the retry loop after at most
// MaxRetries + 1 attempts. The retry loop honours ctx.Done()
// during back-off.
func TestProperty_RetryContextCancellationStops(t *testing.T) {
	t.Parallel()
	f := func(c arbClampedInt) bool {
		budget := c.N
		inner := &flakyProvider{name: "slow", failures: 1000}
		r := NewRetrying(inner, RetryConfig{
			MaxRetries: budget,
			BaseDelay:  50 * time.Microsecond,
			MaxDelay:   50 * time.Microsecond,
		})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Microsecond)
			cancel()
		}()
		_, err := r.Complete(ctx, &Request{Model: "test"})
		if err == nil {
			t.Log("expected error from cancelled context")
			return false
		}
		// Attempts must be bounded; the exact number depends on
		// timing, so we just assert it is in [1, budget+1].
		if inner.attempts < 1 || inner.attempts > budget+1 {
			t.Logf("attempts = %d, want in [1, %d]", inner.attempts, budget+1)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 20}); err != nil {
		t.Fatal(err)
	}
}

// ---- Fallback -------------------------------------------------------------

// arbFallbackShape returns a generator that produces a non-empty
// list of fallback providers distinct from the primary. The
// shape is a sequence of (response behaviour, name) flags used
// by the Fallback contract tests.
type arbFallbackShape struct {
	PrimaryFails bool
	NumFallbacks int
	NumFailures  int // number of fallbacks that fail (in order)
}

func (arbFallbackShape) Generate(rng *rand.Rand, _ int) reflect.Value {
	primary := rng.Intn(2) == 0
	n := 1 + rng.Intn(4) // 1..4 fallbacks
	fails := rng.Intn(n + 1)
	return reflect.ValueOf(arbFallbackShape{
		PrimaryFails: primary,
		NumFallbacks: n,
		NumFailures:  fails,
	})
}

// TestProperty_FallbackPrimarySuccessSkipsOthers asserts that
// when the primary succeeds, no fallback is ever called. The
// primary-success path is the steady state; an accidentally-
// fired fallback would double the cost.
func TestProperty_FallbackPrimarySuccessSkipsOthers(t *testing.T) {
	t.Parallel()
	f := func(_ arbFallbackShape) bool {
		var calls atomic.Int64
		primary := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			calls.Add(1)
			return &Response{Content: "ok"}, nil
		}}
		fb := &callableProvider{fn: func(_ context.Context, _ *Request) (*Response, error) {
			calls.Add(1)
			return nil, errors.New("fallback")
		}}
		chain := NewFallback(primary, []Provider{fb}, nil)
		_, err := chain.Complete(context.Background(), &Request{Model: "t"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		if calls.Load() != 1 {
			t.Logf("calls = %d, want 1", calls.Load())
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_FallbackUsesFirstSuccessfulFallback asserts that
// the fallback chain stops at the first provider that returns
// a response, even if later providers would also succeed.
func TestProperty_FallbackUsesFirstSuccessfulFallback(t *testing.T) {
	t.Parallel()
	f := func(s arbFallbackShape) bool {
		// Skip the no-fallback-needed case (primary succeeds).
		// Skip the all-fail case (covered by
		// TestProperty_FallbackAllFailReturnsError).
		if !s.PrimaryFails {
			return true
		}
		if s.NumFailures >= s.NumFallbacks {
			return true
		}
		primary := &mockProvider{
			name: "primary",
			completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
				return nil, errors.New("primary down")
			},
		}
		var (
			callsMu sync.Mutex
			calls   []string
		)
		var fallbacks []Provider
		for i := 0; i < s.NumFallbacks; i++ {
			fail := i < s.NumFailures
			name := string([]byte{byte('a' + i), 'b'})
			n := name
			f := fail
			fallbacks = append(fallbacks, &mockProvider{
				name: n,
				completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
					callsMu.Lock()
					calls = append(calls, n)
					callsMu.Unlock()
					if f {
						return nil, errors.New(n + " fails")
					}
					return &Response{Content: "ok-from-" + n}, nil
				},
			})
		}
		chain := NewFallback(primary, fallbacks, nil)
		resp, err := chain.Complete(context.Background(), &Request{Model: "t"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		// The chain should stop at the first successful fallback.
		wantCalls := s.NumFailures + 1
		if len(calls) != wantCalls {
			t.Logf("calls = %v, want %d", calls, wantCalls)
			return false
		}
		if resp == nil || resp.Content != "ok-from-"+calls[len(calls)-1] {
			t.Logf("response.Content = %v, last call = %s", resp, calls[len(calls)-1])
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_FallbackAllFailReturnsError asserts that when
// every provider in the chain fails (primary + every fallback),
// Complete returns a non-nil error wrapping the last failure.
func TestProperty_FallbackAllFailReturnsError(t *testing.T) {
	t.Parallel()
	f := func(s arbFallbackShape) bool {
		primary := &mockProvider{
			name: "primary",
			completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
				return nil, errors.New("primary down")
			},
		}
		var fallbacks []Provider
		for i := 0; i < s.NumFallbacks; i++ {
			n := string([]byte{byte('a' + i), 'b'})
			nn := n
			fallbacks = append(fallbacks, &mockProvider{
				name: nn,
				completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
					return nil, errors.New(nn + " down")
				},
			})
		}
		chain := NewFallback(primary, fallbacks, nil)
		_, err := chain.Complete(context.Background(), &Request{Model: "t"})
		return err != nil
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_FallbackDuplicateNameSkipped asserts that a
// fallback whose name matches the primary is skipped. The chain
// must not silently degrade to "primary called twice".
func TestProperty_FallbackDuplicateNameSkipped(t *testing.T) {
	t.Parallel()
	f := func(_ arbClampedInt) bool {
		var calls atomic.Int64
		primary := &mockProvider{
			name: "shared",
			completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
				calls.Add(1)
				return nil, errors.New("primary down")
			},
		}
		dup := &mockProvider{
			name: "shared",
			completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
				calls.Add(1)
				return &Response{Content: "ok"}, nil
			},
		}
		chain := NewFallback(primary, []Provider{dup}, nil)
		_, err := chain.Complete(context.Background(), &Request{Model: "t"})
		if err == nil {
			t.Log("expected error when primary fails and fallback is duplicate")
			return false
		}
		if calls.Load() != 1 {
			t.Logf("primary invoked %d times, want 1", calls.Load())
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_FallbackNameContainsAllProviders asserts that the
// composed Name lists every provider in the chain in order. The
// name is what dashboards see in the metrics; a missing
// provider would hide traffic.
func TestProperty_FallbackNameContainsAllProviders(t *testing.T) {
	t.Parallel()
	f := func(s arbFallbackShape) bool {
		primary := &mockProvider{
			name: "primary",
			completeFunc: func(_ context.Context, _ *Request) (*Response, error) {
				return &Response{Content: "ok"}, nil
			},
		}
		var fallbacks []Provider
		for i := 0; i < s.NumFallbacks; i++ {
			name := string([]byte{byte('a' + i), 'b'})
			fallbacks = append(fallbacks, &mockProvider{
				name:         name,
				completeFunc: func(_ context.Context, _ *Request) (*Response, error) { return &Response{Content: "ok"}, nil },
			})
		}
		chain := NewFallback(primary, fallbacks, nil)
		got := chain.Name()
		if !contains(got, "primary") {
			t.Logf("Name = %q, missing primary", got)
			return false
		}
		for _, fb := range fallbacks {
			if !contains(got, fb.Name()) {
				t.Logf("Name = %q, missing %q", got, fb.Name())
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 30}); err != nil {
		t.Fatal(err)
	}
}

// ---- httptest end-to-end -------------------------------------------------

// TestProperty_HttptestRoundtripPreservesContent asserts that
// the scriptedTransport property carries the request through a
// real HTTP round-trip and that the response Content survives.
// This is the integration test that proves the LLM stack
// composes over a real HTTP transport, not just in-memory mocks.
func TestProperty_HttptestRoundtripPreservesContent(t *testing.T) {
	t.Parallel()
	f := func(content string) bool {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req Request
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resp := Response{
				Content: content,
				Usage:   Usage{TotalTokens: 1},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()
		p := &scriptedTransport{name: "echo", server: srv}
		// Wire through the Registry so the cache path is part
		// of the round-trip too.
		r := NewRegistry()
		r.Register("echo", func(_ ProviderConfig) Provider { return p })
		r.Configure("echo", ProviderConfig{APIKey: "k"})
		got, err := r.Get("echo")
		if err != nil {
			t.Logf("Get: %v", err)
			return false
		}
		resp, err := got.Complete(context.Background(), &Request{Model: "x"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		return resp.Content == content
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_HttptestRetryRoundtripBounded asserts that the
// retry loop composed with the scriptedTransport respects the
// MaxRetries budget. The scripted server returns a transient
// error for the first k calls and a success on the (k+1)th.
func TestProperty_HttptestRetryRoundtripBounded(t *testing.T) {
	t.Parallel()
	f := func(failures arbClampedInt) bool {
		k := failures.N
		budget := k + 4
		var calls atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := calls.Add(1)
			if int(n) <= k {
				http.Error(w, "transient", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(Response{Content: "ok"})
		}))
		defer srv.Close()
		p := &scriptedTransport{name: "flaky", server: srv}
		r := NewRetrying(p, RetryConfig{
			MaxRetries: budget,
			BaseDelay:  10 * time.Microsecond,
			MaxDelay:   10 * time.Microsecond,
		})
		resp, err := r.Complete(context.Background(), &Request{Model: "x"})
		if err != nil {
			t.Logf("Complete: %v", err)
			return false
		}
		if resp.Content != "ok" {
			t.Logf("content = %q", resp.Content)
			return false
		}
		got := calls.Load()
		if got != int64(k+1) {
			t.Logf("calls = %d, want %d", got, k+1)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 20}); err != nil {
		t.Fatal(err)
	}
}

// contains is a tiny string-in-string helper that avoids the
// strings import for one usage.
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
