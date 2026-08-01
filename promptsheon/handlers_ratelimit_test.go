package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/ratelimit"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

)

func TestRateLimit(t *testing.T) {
	s := newTestServer(t)
	handler := s.rateLimit(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRateLimit_Exceeded(t *testing.T) {
	// SEC-RL-2: rate=0 disables the limiter (always-allow). To
	// exercise the deny path we need a non-zero Rate that the
	// bucket can exhaust. Burst=0 with a single request means
	// the second call hits the bucket drain.
	limiter := ratelimit.NewLimiter(ratelimit.Config{Rate: 1, Interval: time.Hour, Burst: 0})
	t.Cleanup(limiter.Stop)

	s := newTestServer(t, WithRateLimiter(limiter))
	var called int
	handler := s.rateLimit(func(w http.ResponseWriter, _ *http.Request) error {
		called++
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// First call: bucket starts at burst=0, the maths below
	// gives tokens=0, the request is denied without the inner
	// handler running.
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if called != 0 {
		t.Error("inner handler should not be called when rate limited")
	}
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After: 60, got %s", rr.Header().Get("Retry-After"))
	}
}

// ---------------------------------------------------------------------------
// requirePerm Tests
// ---------------------------------------------------------------------------
