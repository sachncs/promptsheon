package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowBasic(t *testing.T) {
	l := NewLimiter(Config{Rate: 5, Interval: time.Second, Burst: 5})

	// Should allow burst
	for i := 0; i < 5; i++ {
		if !l.Allow("key-1") {
			t.Fatalf("expected allow on attempt %d", i+1)
		}
	}
	// 6th should be denied
	if l.Allow("key-1") {
		t.Fatal("expected deny after burst")
	}
}

func TestRefill(t *testing.T) {
	l := NewLimiter(Config{Rate: 100, Interval: time.Second, Burst: 1})

	if !l.Allow("key-refill") {
		t.Fatal("expected allow")
	}
	if l.Allow("key-refill") {
		t.Fatal("expected deny")
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond)
	if !l.Allow("key-refill") {
		t.Fatal("expected allow after refill")
	}
}

func TestSeparateKeys(t *testing.T) {
	l := NewLimiter(Config{Rate: 1, Interval: time.Second, Burst: 1})

	if !l.Allow("a") {
		t.Fatal("expected allow for a")
	}
	if l.Allow("a") {
		t.Fatal("expected deny for a")
	}
	// Different key should still work
	if !l.Allow("b") {
		t.Fatal("expected allow for b")
	}
}

func TestMiddleware(t *testing.T) {
	l := NewLimiter(Config{Rate: 2, Interval: time.Second, Burst: 2})

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	}))

	// First two should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 on attempt %d, got %d", i+1, w.Code)
		}
	}

	// Third should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMiddlewareQueryParam(t *testing.T) {
	l := NewLimiter(Config{Rate: 1, Interval: time.Second, Burst: 1})

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test?api_key=qs-key", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Second with same key should be denied
	req = httptest.NewRequest("GET", "/test?api_key=qs-key", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestReset(t *testing.T) {
	l := NewLimiter(Config{Rate: 1, Interval: time.Second, Burst: 1})
	l.Allow("key")
	if l.Allow("key") {
		t.Fatal("expected deny")
	}
	l.Reset()
	if !l.Allow("key") {
		t.Fatal("expected allow after reset")
	}
}
