// Package ratelimit provides per-key rate limiting using a token bucket algorithm.
package ratelimit

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/auth"
)

// trustedProxies is the set of CIDRs that may set X-Forwarded-For
// or X-Real-IP. Configured once at process start from
// PROMPTSHEON_TRUSTED_PROXIES. An empty slice disables trust
// entirely.
var trustedProxies []*net.IPNet

// ConfigureTrustedProxies parses a comma-separated CIDR list and
// installs it as the trusted-proxy set. Exposed so tests can build
// their own configuration without re-running init().
//
// 1.6: the previous implementation kept only the CIDR with the
// smallest mask (largest range) and silently dropped invalid
// entries. With 10.0.0.0/8 + 192.168.0.0/16 only 10.0.0.0/8
// survived; the second CIDR was lost. Now keeps the union and
// logs invalid CIDRs at warn level.
func ConfigureTrustedProxies(raw string) {
	if raw == "" {
		trustedProxies = nil
		return
	}
	var out []*net.IPNet
	for _, c := range strings.Split(raw, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			slog.Warn("trusted_proxies: ignoring invalid CIDR",
				"raw", c, "err", err)
			continue
		}
		out = append(out, n)
	}
	trustedProxies = out
}

// isTrustedProxy reports whether ip falls in any of the configured
// trusted CIDRs. Replaces the previous single-IPNet lookup that
// only kept the largest range and silently dropped the rest.
func isTrustedProxy(ip net.IP) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	for _, n := range trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func init() {
	ConfigureTrustedProxies(os.Getenv("PROMPTSHEON_TRUSTED_PROXIES"))
}

func realRemoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// PERF-RL-1: the limiter is sharded by the first byte of the
// bucket key (fnv-1a hash modulo numShards). Each shard owns its
// own mutex; Allow() takes one lock per call instead of one
// process-wide lock. The TRIZ review flagged the previous single-
// mutex design as a 8x concurrency bottleneck under heavy load.
//
// numShards is a power of two so the modulo is a cheap bitmask.
// 16 shards gives 16-way concurrent Allow() calls at the cost
// of 16 small mutexes.
const numShards = 16

func shardFor(key string) uint64 {
	// FNV-1a 64-bit; cheap and well-distributed for short
	// ASCII keys (API key prefixes, user IDs, IPs).
	h := uint64(1469598103934665603)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return h & (numShards - 1)
}

// shard is one partition of the bucket map. Its mutex is the
// only contention point for keys that hash into it.
type shard struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newShard() *shard {
	return &shard{buckets: map[string]*bucket{}}
}

// Limiter enforces rate limits per API key using a token bucket.
//
// Concurrency: the limiter is sharded into numShards partitions.
// Allow() takes one shard mutex per call, not one process-wide
// mutex. Cleanup walks every shard in turn.
type Limiter struct {
	shards      [numShards]*shard
	rate        int           // tokens per interval
	interval    time.Duration // refill interval
	burst       int           // max tokens (bucket capacity)
	stop        chan struct{}
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
	// SEC-RL-2: rate=0 means rate limiting is disabled. Reset
	// Burst to 0 so the bucket is fully open; callers wanting a
	// large burst must set PROMPTSHEON_RATE_BURST explicitly.
	if cfg.Rate == 0 {
		cfg.Burst = 0
	}
	return cfg
}

// NewLimiter creates a rate limiter with the given config.
func NewLimiter(cfg Config) *Limiter {
	l := &Limiter{
		rate:        cfg.Rate,
		interval:    cfg.Interval,
		burst:       cfg.Burst,
		stop:        make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	for i := range l.shards {
		l.shards[i] = newShard()
	}
	// Start background cleanup of stale buckets.
	go l.cleanup()
	return l
}

// Stop terminates the background cleanup goroutine. Safe to call more
// than once.
func (l *Limiter) Stop() {
	l.shards[0].mu.Lock()
	select {
	case <-l.stop:
		l.shards[0].mu.Unlock()
		// already stopped
	default:
		close(l.stop)
		l.shards[0].mu.Unlock()
	}
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
			cutoff := time.Now().Add(-10 * time.Minute)
			for _, s := range l.shards {
				s.mu.Lock()
				for key, b := range s.buckets {
					if b.lastFill.Before(cutoff) {
						delete(s.buckets, key)
					}
				}
				s.mu.Unlock()
			}
		}
	}
}

// Allow checks if a request from the given key is allowed.
//
// PERF-RL-1: the limiter is sharded by key-hash modulo
// numShards. Two callers on different keys hit different
// shards in 14/16 of calls; concurrency scales linearly
// with the shard count.
func (l *Limiter) Allow(key string) bool {
	// SEC-RL-2: rate=0 means rate limiting is disabled. Without
	// this short-circuit the bucket maths below sees
	// burst=0, tokens=0, and refuses every request — the
	// opposite of what an operator setting PROMPTSHEON_RATE_LIMIT=0
	// expects.
	if l.rate == 0 {
		return true
	}
	s := l.shards[shardFor(key)]
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.burst), lastFill: time.Now()}
		s.buckets[key] = b
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
//
// Probes (/health, /ready) and the Prometheus scrape path
// (/metrics) are exempt: a busy deployment would otherwise
// 429 its own health checks under load and Kubernetes would
// restart the pod.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/ready", "/metrics":
			next.ServeHTTP(w, r)
			return
		}
		key := ExtractKey(r)
		if !l.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ExtractKey derives the rate-limit bucket key from the request.
// Exposed so the api server's rateLimit wrapper can call it
// directly without re-implementing the user/IP precedence.
//
// SECURITY: the previous implementation keyed off the raw bearer token
// value, which meant an attacker spamming with random tokens could grow
// the bucket map to millions of entries between sweeps. We now key off
// the authenticated user ID when auth has run, and the client IP
// otherwise. Using the validated user ID (not the raw token) means
// changing tokens for the same user still hits the same bucket.
//
// PROMPTSHEON_TRUSTED_PROXIES is a comma-separated list of CIDRs.
// When set, X-Forwarded-For and X-Real-IP are honoured only when
// the request's RemoteAddr falls inside one of the configured
// networks. Without a configured list, forwarded headers are
// ignored (the request's direct RemoteAddr is used) so an exposed
// daemon cannot be tricked into per-attacker buckets.
func ExtractKey(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return "user:" + u.ID
	}
	remote := realRemoteAddr(r)
	if ip := net.ParseIP(remote); ip != nil && isTrustedProxy(ip) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return "ip:" + strings.TrimSpace(xff[:i])
			}
			return "ip:" + strings.TrimSpace(xff)
		}
		if rip := r.Header.Get("X-Real-IP"); rip != "" {
			return "ip:" + strings.TrimSpace(rip)
		}
	}
	return "ip:" + remote
}

// Reset clears all buckets.
func (l *Limiter) Reset() {
	for _, s := range l.shards {
		s.mu.Lock()
		s.buckets = make(map[string]*bucket)
		s.mu.Unlock()
	}
}
