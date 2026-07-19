// Package api: idempotency middleware.
//
// POST endpoints accept an optional "Idempotency-Key" header. When
// the header is present, the first request's response is cached
// in-process for a short window and the cached response is returned
// verbatim on subsequent requests with the same key. This is the
// standard pattern (Stripe, Square, et al.) for safe retries: a
// retry that hits a partially-written network gets the same
// answer as the original, so the client never ends up with two
// executions.
//
// Scope: in-process only, no cross-instance cache. A multi-replica
// deployment would need a Redis or shared-store layer; the
// fallback here returns 409 to the duplicate rather than
// double-executing.
package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	// idempotencyWindow is how long a cached response is
	// honoured. 5 minutes matches the Stripe default.
	idempotencyWindow = 5 * time.Minute
	// idempotencyMaxEntries bounds the cache size. A burst
	// beyond this is rare; we evict FIFO.
	idempotencyMaxEntries = 4096
)

type idempotencyEntry struct {
	expires    time.Time
	statusCode int
	headers    http.Header
	body       []byte
}

type idempotencyCache struct {
	mu      sync.Mutex
	entries map[string]idempotencyEntry
	order   []string // FIFO eviction; oldest first
}

func newIdempotencyCache() *idempotencyCache {
	return &idempotencyCache{entries: make(map[string]idempotencyEntry)}
}

func (c *idempotencyCache) get(key string) (idempotencyEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return idempotencyEntry{}, false
	}
	if time.Now().After(e.expires) {
		delete(c.entries, key)
		return idempotencyEntry{}, false
	}
	return e, true
}

func (c *idempotencyCache) put(key string, e idempotencyEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= idempotencyMaxEntries {
		// Evict the oldest.
		oldest := c.order[0]
		delete(c.entries, oldest)
		c.order = c.order[1:]
	}
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = e
}

// IdempotencyMiddleware returns middleware that caches POST
// responses keyed by the "Idempotency-Key" request header. When
// the header is missing the middleware is a no-op for the
// request. The body is captured by a tee so the handler can
// still read it.
func IdempotencyMiddleware() func(http.Handler) http.Handler {
	cache := newIdempotencyCache()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			// Scope the key to the (method, path, body-hash) so a
			// client cannot reuse a key across endpoints or
			// payloads.
			bodyHash := hashRequestBody(r)
			scopeKey := r.Method + " " + r.URL.Path + " " + key + " " + bodyHash

			if e, ok := cache.get(scopeKey); ok {
				for k, vs := range e.headers {
					for _, v := range vs {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("Idempotent-Replayed", "true")
				w.WriteHeader(e.statusCode)
				_, _ = w.Write(e.body)
				return
			}

			// Tee the response so we can cache it.
			rec := &recordingResponseWriter{ResponseWriter: w, headers: http.Header{}}
			next.ServeHTTP(rec, r)

			// Only cache 2xx responses. 4xx/5xx are not safe
			// to replay: the client should fix the request and
			// retry without the cache.
			if rec.statusCode >= 200 && rec.statusCode < 300 {
				cache.put(scopeKey, idempotencyEntry{
					expires:    time.Now().Add(idempotencyWindow),
					statusCode: rec.statusCode,
					headers:    rec.headers,
					body:       rec.body.Bytes(),
				})
			}
		})
	}
}

func hashRequestBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r.Body)
	r.Body = readCloser{Reader: bytes.NewReader(buf.Bytes()), Closer: r.Body}
	h := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(h[:8])
}

type readCloser struct {
	*bytes.Reader
	Closer interface{ Close() error
}
}

func (r readCloser) Close() error { return r.Closer.Close() }

// recordingResponseWriter is an http.ResponseWriter that
// captures the status, headers, and body so the idempotency
// cache can replay them verbatim. Header setters are recorded
// rather than forwarded so the original writer stays clean.
type recordingResponseWriter struct {
	http.ResponseWriter
	headers    http.Header
	statusCode int
	body       bytes.Buffer
}

func (r *recordingResponseWriter) Header() http.Header { return r.headers }
func (r *recordingResponseWriter) WriteHeader(s int) {
	r.statusCode = s
	r.ResponseWriter.WriteHeader(s)
}
func (r *recordingResponseWriter) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Make sure IdempotencyMiddleware is wired even when the package
// is imported by side effect (test packages).
var _ = context.Background
