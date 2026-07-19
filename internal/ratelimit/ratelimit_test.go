package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
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

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	t.Cleanup(l.Stop)
	l.Allow("key")
	if l.Allow("key") {
		t.Fatal("expected deny")
	}
	l.Reset()
	if !l.Allow("key") {
		t.Fatal("expected allow after reset")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Rate != 60 {
		t.Fatalf("expected rate 60, got %d", cfg.Rate)
	}
	if cfg.Interval != time.Minute {
		t.Fatalf("expected interval 1m, got %v", cfg.Interval)
	}
	if cfg.Burst != 10 {
		t.Fatalf("expected burst 10, got %d", cfg.Burst)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Run("defaults when no env set", func(t *testing.T) {
		_ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT")
		_ = os.Unsetenv("PROMPTSHEON_RATE_BURST")
		cfg := LoadConfigFromEnv()
		if cfg.Rate != 60 || cfg.Burst != 10 {
			t.Fatalf("expected 60/10, got %d/%d", cfg.Rate, cfg.Burst)
		}
	})

	t.Run("custom rate", func(t *testing.T) {
		_ = os.Setenv("PROMPTSHEON_RATE_LIMIT", "30")
		t.Cleanup(func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT") })
		_ = os.Unsetenv("PROMPTSHEON_RATE_BURST")
		cfg := LoadConfigFromEnv()
		if cfg.Rate != 30 {
			t.Fatalf("expected rate 30, got %d", cfg.Rate)
		}
	})

	t.Run("rate zero disables", func(t *testing.T) {
		_ = os.Setenv("PROMPTSHEON_RATE_LIMIT", "0")
		t.Cleanup(func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT") })
		cfg := LoadConfigFromEnv()
		if cfg.Rate != 0 {
			t.Fatalf("expected rate 0, got %d", cfg.Rate)
		}
		if cfg.Burst != 1_000_000 {
			t.Fatalf("expected burst 1000000 when disabled, got %d", cfg.Burst)
		}
	})

	t.Run("custom burst", func(t *testing.T) {
		_ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT")
		_ = os.Setenv("PROMPTSHEON_RATE_BURST", "25")
		t.Cleanup(func() { _ = os.Unsetenv("PROMPTSHEON_RATE_BURST") })
		cfg := LoadConfigFromEnv()
		if cfg.Burst != 25 {
			t.Fatalf("expected burst 25, got %d", cfg.Burst)
		}
	})

	t.Run("invalid rate uses default", func(t *testing.T) {
		_ = os.Setenv("PROMPTSHEON_RATE_LIMIT", "not-a-number")
		t.Cleanup(func() { _ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT") })
		_ = os.Unsetenv("PROMPTSHEON_RATE_BURST")
		cfg := LoadConfigFromEnv()
		if cfg.Rate != 60 {
			t.Fatalf("expected rate 60, got %d", cfg.Rate)
		}
	})

	t.Run("invalid burst uses default", func(t *testing.T) {
		_ = os.Unsetenv("PROMPTSHEON_RATE_LIMIT")
		_ = os.Setenv("PROMPTSHEON_RATE_BURST", "-5")
		t.Cleanup(func() { _ = os.Unsetenv("PROMPTSHEON_RATE_BURST") })
		cfg := LoadConfigFromEnv()
		if cfg.Burst != 10 {
			t.Fatalf("expected burst 10, got %d", cfg.Burst)
		}
	})
}

func TestStop(_ *testing.T) {
	l := NewLimiter(Config{Rate: 10, Interval: time.Second, Burst: 10})
	l.Stop()
	// Should be safe to use after stop
	l.Allow("key")
	l.Reset()
}

func TestStopTwice(_ *testing.T) {
	l := NewLimiter(Config{Rate: 10, Interval: time.Second, Burst: 10})
	l.Stop()
	l.Stop()
}

func TestCleanupExit(_ *testing.T) {
	l := NewLimiter(Config{Rate: 10, Interval: time.Second, Burst: 10})
	// Give the goroutine a moment to start, then stop
	time.Sleep(10 * time.Millisecond)
	l.Stop()
}

func TestExtractKeyUserContext(t *testing.T) {
	u := &auth.User{ID: "user-42"}
	ctx := auth.WithUserContext(context.Background(), u)
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(ctx)
	got := extractKey(req)
	if got != "user:user-42" {
		t.Fatalf("expected user:user-42, got %s", got)
	}
}

func TestExtractKeyUserContextEmptyID(t *testing.T) {
	u := &auth.User{ID: ""}
	ctx := auth.WithUserContext(context.Background(), u)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	req = req.WithContext(ctx)
	got := extractKey(req)
	if got != "ip:10.0.0.1" {
		t.Fatalf("expected ip:10.0.0.1, got %s", got)
	}
}

func TestExtractKeyUserContextNilUser(t *testing.T) {
	// WithUserContext with nil stores (*User)(nil) in context.
	// UserFromContext returns (nil, true) and the nil check in extractKey
	// prevents a nil pointer dereference.
	ctx := auth.WithUserContext(context.Background(), nil)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.2:5678"
	req = req.WithContext(ctx)
	got := extractKey(req)
	if got != "ip:10.0.0.2" {
		t.Fatalf("expected ip:10.0.0.2, got %s", got)
	}
}

func TestExtractKeyXForwardedForComma(t *testing.T) {
	ConfigureTrustedProxies("192.0.2.0/24")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	got := extractKey(req)
	if got != "ip:1.2.3.4" {
		t.Fatalf("expected ip:1.2.3.4, got %s", got)
	}
}

func TestExtractKeyXForwardedFor(t *testing.T) {
	ConfigureTrustedProxies("192.0.2.0/24")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	got := extractKey(req)
	if got != "ip:1.2.3.4" {
		t.Fatalf("expected ip:1.2.3.4, got %s", got)
	}
}

func TestExtractKeyXRealIP(t *testing.T) {
	ConfigureTrustedProxies("192.0.2.0/24")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Real-IP", "5.6.7.8")
	got := extractKey(req)
	if got != "ip:5.6.7.8" {
		t.Fatalf("expected ip:5.6.7.8, got %s", got)
	}
}

func TestExtractKeyRemoteAddr(t *testing.T) {
	ConfigureTrustedProxies("")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "9.10.11.12:8080"
	got := extractKey(req)
	if got != "ip:9.10.11.12" {
		t.Fatalf("expected ip:9.10.11.12, got %s", got)
	}
}

func TestExtractKeyXForwardedForPreferred(t *testing.T) {
	ConfigureTrustedProxies("192.0.2.0/24")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	got := extractKey(req)
	if got != "ip:1.2.3.4" {
		t.Fatalf("expected ip:1.2.3.4, got %s", got)
	}
}

func TestExtractKeyXRealIPFallback(t *testing.T) {
	ConfigureTrustedProxies("192.0.2.0/24")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Real-IP", "5.6.7.8")
	got := extractKey(req)
	if got != "ip:5.6.7.8" {
		t.Fatalf("expected ip:5.6.7.8, got %s", got)
	}
}
