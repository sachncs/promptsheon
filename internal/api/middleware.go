package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/trace"
)

// Shared audit-detail keys. Handler packages reference these
// constants when populating s.audit() details; centralising them
// here keeps the audit vocabulary consistent across handlers.
const (
	auditKeyName    = "name"
	auditKeyStatus  = "status"
	auditKeyVersion = "version"
)

// Middleware is a function that wraps a Func with additional behavior.
type Middleware func(Func) Func

// ChainHTTP applies http.Handler middlewares in order.
func ChainHTTP(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// generateRequestID creates a cryptographically random request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Logging returns middleware that logs each request with trace_id, request_id, user_id.
//
// SECURITY: user_id is read AFTER the inner handler runs because the
// authenticator attaches the user to the request context inside the
// per-route requirePerm middleware. The previous implementation
// snapshotted user_id before the handler chain ran, which meant
// `user_id` was always empty in access logs.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := generateRequestID()
			if hdr := r.Header.Get("X-Request-ID"); hdr != "" {
				requestID = hdr
			}

			traceID := ""
			if span, ok := trace.SpanFromContext(r.Context()); ok {
				traceID = span.TraceID
			}
			if hdr := r.Header.Get("X-Trace-ID"); hdr != "" {
				traceID = hdr
			}

			ctx := trace.WithRequestID(r.Context(), requestID)
			if traceID != "" {
				ctx = trace.WithTraceID(ctx, traceID)
			}
			ctx = WithSlogContext(ctx, logger.With(
				slog.String("request_id", requestID),
				slog.String("trace_id", traceID),
			))
			// Attach the request so downstream helpers (notably
			// Server.audit) can enrich the audit entry with the
			// remote address and user-agent.
			ctx = WithRequest(ctx, r)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			// Re-read user_id after the handler chain. The
			// authenticator populates auth.UserFromContext inside
			// requirePerm, which runs as part of the handler.
			userID := ""
			if u, ok := auth.UserFromContext(r.Context()); ok && u != nil {
				userID = u.ID
			}

			// Log at DEBUG for high-traffic servers. The structured
			// logger is configured at the appropriate level by main.
			logger.Debug("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration", time.Since(start).String(),
				"remote", r.RemoteAddr,
				"request_id", requestID,
				"trace_id", traceID,
				"user_id", userID,
			)
		})
	}
}

// Recovery returns middleware that recovers from panics. The
// response uses the same JSON envelope as the rest of the API
// (writeError) so clients that send Accept: application/json
// don't get text/plain by surprise (SEC-8).
//
// BUG-9 fix: when a handler panics after it has already written
// the response status, recovering here and writing again would
// log a spurious "superfluous WriteHeader" warning and double-
// encode the body. Wrap the response writer with a once-only
// guard so the second writeError is a no-op.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &recoveryWriter{ResponseWriter: w}
			defer func() {
				if p := recover(); p != nil {
					requestID, _ := trace.RequestIDFromContext(r.Context())
					traceID, _ := trace.IDFromContext(r.Context())
					logger.Error("panic recovered",
						"err", p,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", requestID,
						"trace_id", traceID,
					)
					if rec.alive() {
						writeError(rec, &HTTPError{
							Status:  http.StatusInternalServerError,
							Message: "internal server error",
						})
					}
				}
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

// recoveryWriter wraps a ResponseWriter and records whether a
// write has already started. After a WriteHeader call the
// underlying connection is committed; subsequent writes are
// fatal in the stdlib (they log a warning). recoveryWriter
// makes the second write a silent no-op so the recover path
// does not corrupt the response when the original handler
// had already started streaming.
type recoveryWriter struct {
	http.ResponseWriter
	wrote bool
}

func (r *recoveryWriter) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *recoveryWriter) Write(b []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *recoveryWriter) alive() bool {
	return !r.wrote
}

// CORS returns middleware that handles CORS preflight requests.
// The allowedOrigins parameter controls Access-Control-Allow-Origin.
//
// Semantics:
//   - Empty list: deny all cross-origin requests. No CORS headers
//     are emitted.
//   - "*" as the single element: echo "*" back. This is insecure
//     and is rejected at config-load time when the bind is
//     non-loopback; the only legitimate use is local development.
//   - Multiple specific origins: the request's Origin is echoed
//     back only when it matches one of the configured values.
//     Otherwise no CORS header is set and the browser blocks the
//     response.
//
// Access-Control-Allow-Credentials is intentionally never emitted:
// the daemon uses Authorization headers, not cookies, so credentialed
// CORS is not needed.
func CORS(allowedOrigins ...string) func(http.Handler) http.Handler {
	wildcard := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allow := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			continue
		}
		allow[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestOrigin := r.Header.Get("Origin")
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Vary", "Origin")
			} else if requestOrigin != "" {
				if _, ok := allow[requestOrigin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
					w.Header().Set("Vary", "Origin")
				}
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Trace-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders returns middleware that adds security-related HTTP headers.
//
// BUG-24: Strict-Transport-Security is only useful over an
// encrypted channel; setting it on a plain HTTP deployment
// has no security benefit and trains operators to ignore the
// header. The middleware now inspects r.TLS (set by the
// stdlib server when ListenAndServeTLS is in play) and the
// X-Forwarded-Proto header (set by a trusted TLS-terminating
// proxy). HSTS is sent only when at least one of those
// signals a TLS connection.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// MaxBytesReader returns middleware that limits request body sizes.
func MaxBytesReader(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// slogContextKey is the context key for slog.Logger.
type slogContextKey struct{}

// WithSlogContext returns a new context with the logger attached.
func WithSlogContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, slogContextKey{}, logger)
}

// SlogFromContext returns the logger from the context. Falls back to
// slog.Default() so handlers can use it without an explicit check.
// Production code that wires the middleware should never see the
// fallback because the middleware always attaches a logger.
func SlogFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(slogContextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
