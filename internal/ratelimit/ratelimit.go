// Package ratelimit provides per-key rate limiting using a token bucket algorithm.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Limiter enforces rate limits per API key using a token bucket.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens (bucket capacity)
}

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// Config controls rate limiter behavior.
type Config struct {
	Rate     int           // requests per interval
	Interval time.Duration // refill interval
	Burst    int           // max burst size
}

// DefaultConfig returns 60 requests/minute with burst of 10.
func DefaultConfig() Config {
	return Config{
		Rate:     60,
		Interval: time.Minute,
		Burst:    10,
	}
}

// NewLimiter creates a rate limiter with the given config.
func NewLimiter(cfg Config) *Limiter {
	return &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     cfg.Rate,
		interval: cfg.Interval,
		burst:    cfg.Burst,
	}
}

// Allow checks if a request from the given key is allowed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.burst), lastFill: time.Now()}
		l.buckets[key] = b
	}

	// Refill tokens
	elapsed := time.Since(b.lastFill)
	tokensToAdd := elapsed.Seconds() * float64(l.rate) / l.interval.Seconds()
	b.tokens += tokensToAdd
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.lastFill = time.Now()

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware returns HTTP middleware that enforces rate limiting per API key.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := extractKey(r)
		if !l.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`)) //nolint:errcheck
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractKey(r *http.Request) string {
	// Try Authorization header
	if auth := r.Header.Get("Authorization"); auth != "" {
		if len(auth) > 7 && auth[:7] == "Bearer " {
			return auth[7:]
		}
	}
	// Fall back to query param
	if key := r.URL.Query().Get("api_key"); key != "" {
		return key
	}
	// Fall back to remote addr
	return r.RemoteAddr
}

// Reset clears all buckets.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}
