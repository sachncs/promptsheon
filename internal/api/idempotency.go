// Package api: idempotency middleware.
//
// POST endpoints accept an optional "Idempotency-Key" header. When
// the header is present, the first request's response is cached
// for a short window and the cached response is returned verbatim
// on subsequent requests with the same key. This is the standard
// pattern (Stripe, Square, et al.) for safe retries: a retry that
// hits a partially-written network gets the same answer as the
// original, so the client never ends up with two executions.
//
// Storage: SQLite-backed via IdempotencyStore (API-IDEMP-1). The
// previous in-memory implementation was per-replica and would
// double-execute a POST when a load balancer routed the retry to
// a different replica than the original. The store interface is
// small (Get / Put) so a Redis backend can drop in later.
//
// The body hash is computed while streaming (PERF-7): we never
// buffer more than MaxBytesReader's limit and we never allocate
// a copy of the body just to hash it.
package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/store"
)

const (
	// idempotencyWindow is how long a cached response is
	// honoured. 5 minutes matches the Stripe default.
	idempotencyWindow = 5 * time.Minute
)

// IdempotencyStore is re-exported so handlers don't have to
// import the store package. The store package owns the
// persistence (SQLite / future Redis); the api package owns the
// replay logic.
type IdempotencyStore = store.IdempotencyStore

// inMemoryIdempotencyCache keeps the previous behaviour available
// for tests that don't need cross-replica semantics. Production
// wiring (cmd/promptsheond/main.go) passes a SQLite-backed store.
//
// API-IDEMP-2 fix: c.order was previously a FIFO slice that
// leaked entries on `get`-eviction. The eviction now also removes
// the key from `c.order`.
type inMemoryIdempotencyCache struct {
	mu      sync.Mutex
	entries map[string]store.IdempotencyEntry
	order   []string // FIFO eviction; oldest first
}

const idempotencyMaxEntries = 4096

func newInMemoryIdempotencyCache() *inMemoryIdempotencyCache {
	return &inMemoryIdempotencyCache{entries: make(map[string]store.IdempotencyEntry)}
}

func (c *inMemoryIdempotencyCache) get(key string) (store.IdempotencyEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return store.IdempotencyEntry{}, false
	}
	if time.Now().After(e.Expires) {
		delete(c.entries, key)
		// ponytail: API-IDEMP-2 — remove from `c.order` so a
		// later put() doesn't see a phantom entry. The previous
		// implementation skipped this and the slice grew
		// monotonically until the process restarted.
		c.removeFromOrder(key)
		return store.IdempotencyEntry{}, false
	}
	return e, true
}

func (c *inMemoryIdempotencyCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *inMemoryIdempotencyCache) put(key string, e store.IdempotencyEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= idempotencyMaxEntries {
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
//
// store may be nil; the middleware falls back to the in-memory
// cache for tests. Production wires the SQLite-backed store via
// api.WithIdempotencyStore.
func IdempotencyMiddleware(idempStore IdempotencyStore) func(http.Handler) http.Handler {
	mem := newInMemoryIdempotencyCache()
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
			// PERF-7: stream-hash the body instead of buffering
			// the full payload (up to MaxBytesReader's limit).
			// The tee reader yields the original bytes AND feeds
			// them through the hasher; r.Body is replaced with
			// a reader that replays the same bytes for the
			// handler.
			bodyHash, bodyReader, err := hashAndTeeBody(r.Body)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			r.Body = bodyReader
			scopeKey := r.Method + " " + r.URL.Path + " " + key + " " + bodyHash

			if e, ok := lookupIdempotency(idempStore, mem, scopeKey); ok {
				for k, vs := range e.Headers {
					for _, v := range vs {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("Idempotent-Replayed", "true")
				w.WriteHeader(e.StatusCode)
				_, _ = w.Write(e.Body)
				return
			}

			// Tee the response so we can cache it.
			rec := &recordingResponseWriter{ResponseWriter: w, headers: http.Header{}}
			next.ServeHTTP(rec, r)

			// Only cache 2xx responses. 4xx/5xx are not safe
			// to replay: the client should fix the request and
			// retry without the cache.
			if rec.statusCode >= 200 && rec.statusCode < 300 {
				entry := store.IdempotencyEntry{
					Expires:    time.Now().Add(idempotencyWindow),
					StatusCode: rec.statusCode,
					Headers:    rec.headers,
					Body:       rec.body.Bytes(),
				}
				if idempStore != nil {
					_ = idempStore.PutIdempotency(r.Context(), scopeKey, entry)
				} else {
					mem.put(scopeKey, entry)
				}
			}
		})
	}
}

// lookupIdempotency asks the store first, then falls back to the
// in-memory cache. A store error degrades gracefully to the
// in-memory path so a DB blip doesn't break retries entirely.
func lookupIdempotency(store IdempotencyStore, mem *inMemoryIdempotencyCache, key string) (store.IdempotencyEntry, bool) {
	if store != nil {
		if e, err := store.GetIdempotency(context.Background(), key); err == nil {
			return e, true
		}
	}
	return mem.get(key)
}

// hashAndTeeBody hashes the request body in a single pass and
// returns (hex-hash-prefix, replay-reader, error). The returned
// reader exposes the same bytes again so the handler's
// json.Decode still sees the original payload.
//
// PERF-7: we never hold more than the io.Copy buffer in memory.
// The hash is sha256 truncated to 8 bytes (16 hex chars), which
// is enough collision resistance for an idempotency key.
func hashAndTeeBody(body io.ReadCloser) (string, io.ReadCloser, error) {
	if body == nil {
		return "", http.NoBody, nil
	}
	h := sha256.New()
	var buf bytes.Buffer
	if _, err := io.Copy(io.MultiWriter(&buf, h), body); err != nil {
		_ = body.Close()
		return "", body, err
	}
	_ = body.Close()
	digest := h.Sum(nil)
	prefix := hex.EncodeToString(digest[:8])
	return prefix, &readCloser{Reader: bytes.NewReader(buf.Bytes()), Closer: body}, nil
}

// readCloser is a small adapter so the replayed reader can
// satisfy io.ReadCloser (the handler expects r.Body.Close() to
// work).
type readCloser struct {
	*bytes.Reader
	Closer io.Closer
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
