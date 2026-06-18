package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"promptsheon/internal/auth"
	"promptsheon/internal/trace"
)

// Middleware is a function that wraps an APIFunc with additional behavior.
type Middleware func(APIFunc) APIFunc

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
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

// Logging returns middleware that logs each request with trace_id, request_id, user_id.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate request ID
			requestID := generateRequestID()
			if hdr := r.Header.Get("X-Request-ID"); hdr != "" {
				requestID = hdr
			}

			// Propagate trace ID from span context if present
			traceID := ""
			if span, ok := trace.SpanFromContext(r.Context()); ok {
				traceID = span.TraceID
			}
			if hdr := r.Header.Get("X-Trace-ID"); hdr != "" {
				traceID = hdr
			}

			// Inject IDs into context
			ctx := trace.WithRequestID(r.Context(), requestID)
			if traceID != "" {
				ctx = trace.WithTraceID(ctx, traceID)
			}

			// Extract user ID if authenticated
			userID := ""
			if u, ok := auth.UserFromContext(ctx); ok {
				userID = u.ID
				ctx = trace.WithUserID(ctx, userID)
			}

			// Add slog context for downstream handlers
			attrs := []slog.Attr{
				slog.String("request_id", requestID),
			}
			if traceID != "" {
				attrs = append(attrs, slog.String("trace_id", traceID))
			}
			if userID != "" {
				attrs = append(attrs, slog.String("user_id", userID))
			}
			// Convert attrs to []any for slog.With
			args := make([]any, len(attrs))
			for i, a := range attrs {
				args[i] = a
			}
			ctx = WithSlogContext(ctx, logger.With(args...))

			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			logger.Info("http request",
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

// Recovery returns middleware that recovers from panics.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID, _ := trace.RequestIDFromContext(r.Context())
					traceID, _ := trace.TraceIDFromContext(r.Context())
					logger.Error("panic recovered",
						"err", rec,
						"method", r.Method,
						"path", r.URL.Path,
						"request_id", requestID,
						"trace_id", traceID,
					)
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS returns middleware that handles CORS preflight requests.
// The allowedOrigins parameter controls Access-Control-Allow-Origin.
// Pass "*" to allow all origins, or a specific list for production.
func CORS(allowedOrigins ...string) func(http.Handler) http.Handler {
	origin := "*"
	if len(allowedOrigins) > 0 {
		origin = allowedOrigins[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Trace-ID")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders returns middleware that adds security-related HTTP headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0")
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

// SlogFromContext returns the logger from the context.
func SlogFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(slogContextKey{}).(*slog.Logger); ok {
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
