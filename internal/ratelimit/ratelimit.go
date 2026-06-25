// Package ratelimit provides per-key rate limiting using a token bucket algorithm.
package ratelimit

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"promptsheon/internal/auth"
)

// Limiter enforces rate limits per API key using a token bucket.
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	rate       int           // tokens per interval
	interval   time.Duration // refill interval
	burst      int           // max tokens (bucket capacity)
	stop       chan struct{}
	cleanupDone chan struct{}
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

// LoadConfigFromEnv reads rate limit settings from environment variables.
// PROMPTSHEON_RATE_LIMIT=0 disables rate limiting entirely.
// PROMPTSHEON_RATE_BURST overrides the burst size (default 10).
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("PROMPTSHEON_RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			cfg.Rate = n
		}
	}
	if v := os.Getenv("PROMPTSHEON_RATE_BURST"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			cfg.Burst = n
		}
	}
	// If rate is 0, set burst to a large value so the bucket never blocks.
	if cfg.Rate == 0 {
		cfg.Burst = 1_000_000
	}
	return cfg
}

// NewLimiter creates a rate limiter with the given config.
func NewLimiter(cfg Config) *Limiter {
	l := &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     cfg.Rate,
		interval: cfg.Interval,
		burst:    cfg.Burst,
		stop:     make(chan struct{}),
	}
	// Start background cleanup of stale buckets.
	go l.cleanup()
	return l
}

// Stop terminates the background cleanup goroutine. Safe to call more
// than once.
func (l *Limiter) Stop() {
	l.mu.Lock()
	select {
	case <-l.stop:
		// already stopped
	default:
		close(l.stop)
	}
	l.mu.Unlock()
	<-l.cleanupDone
}

// cleanup periodically removes stale rate limit buckets to prevent memory leaks.
func (l *Limiter) cleanup() {
	defer close(l.cleanupDone)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for key, b := range l.buckets {
				if b.lastFill.Before(cutoff) {
					delete(l.buckets, key)
				}
			}
			l.mu.Unlock()
		}
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

// extractKey derives the rate-limit bucket key from the request.
//
// SECURITY: the previous implementation keyed off the raw bearer token
// value, which meant an attacker spamming with random tokens could grow
// the bucket map to millions of entries between sweeps. We now key off
// the authenticated user ID when auth has run, and the client IP
// otherwise. Using the validated user ID (not the raw token) means
// changing tokens for the same user still hits the same bucket.
func extractKey(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return "user:" + u.ID
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return "ip:" + strings.TrimSpace(xff[:i])
		}
		return "ip:" + strings.TrimSpace(xff)
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return "ip:" + strings.TrimSpace(rip)
	}
	return "ip:" + r.RemoteAddr
}

// Reset clears all buckets.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}
