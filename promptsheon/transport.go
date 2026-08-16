package promptsheon

import (
	"context"
	"net/http"
	"os"

	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/ratelimit"
	"github.com/sachncs/promptsheon/promptsheon/store"
)

// isRequestTLS reports whether the inbound request arrived over an
// encrypted channel. It checks r.TLS (set by ListenAndServeTLS) and
// X-Forwarded-Proto (set by a trusted TLS-terminating proxy). Used to
// set Secure on cookies and HSTS on the response — both useless on

// requirePerm returns middleware that requires a specific permission.
func (s *Server) requirePerm(perm auth.Permission) func(Func) Func {
	return func(fn Func) Func {
		return func(w http.ResponseWriter, r *http.Request) error {
			if s.requireAuth && s.authn != nil {
				user, err := s.authn.Authenticate(r)
				if err != nil {
					return &HTTPError{Status: http.StatusUnauthorized, Message: "unauthorized: " + err.Error()}
				}
				r = r.WithContext(auth.WithUserContext(r.Context(), user))
			}
			user, ok := auth.UserFromContext(r.Context())
			if !ok && s.requireAuth {
				return &HTTPError{Status: http.StatusUnauthorized, Message: "no user in context"}
			}
			if ok && !auth.HasPermission(user.Role, perm) {
				return &HTTPError{Status: http.StatusForbidden, Message: "insufficient permissions"}
			}
			return fn(w, r)
		}
	}
}

// wrapHandler wraps a Func into an http.HandlerFunc with error handling.
func (s *Server) wrapHandler(fn Func) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			s.logger.Error("handler error",
				"err", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			writeError(w, err)
		}
	}
}

// writeError writes a JSON error response, inferring the status code from

// httpRequestFromContext returns the *http.Request stored in the
// context by the request middleware, if any. Returns nil if there is

// ReadOnlyMiddleware returns 503 Service Unavailable for any
// non-GET request when the daemon is in read-only mode. Used
// during canary / blue-green rollouts so the new code can run
// against a live workload before writes are enabled. Set via
// PROMPTSHEON_READ_ONLY=true.
//
// Read-only mode is intentional and fail-closed: a single
// misconfigured toggle blocks the entire write surface, not
// the read surface. Operators get a clear 503 with a
// `reason` field so log dashboards can alert on accidental
// lockouts.
func ReadOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("PROMPTSHEON_READ_ONLY") == "true" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"daemon is in read-only mode","details":{"reason":"PROMPTSHEON_READ_ONLY=true"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleLogsStream streams backend log entries over Server-Sent
// Events. The stream is open-ended until the caller disconnects.
// LogsStream handles the request.
func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) error {
	if s.logHub == nil {
		return badRequest("log streaming not configured")
	}
	s.logHub.HandleSSE(w, r)
	return nil
}

// handleMetricsPrometheus returns the metrics collector in the
// Prometheus text exposition format.
// MetricsPrometheus handles the request.
func (s *Server) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) error {
	if s.collector == nil {
		return badRequest("metrics not configured")
	}
	s.collector.Handler().ServeHTTP(w, r)
	return nil
}

// callerID returns the authenticated user's ID, or "api" if no user
// is in the request context. Used to populate CreatedBy fields
// without each handler re-implementing the lookup.
func callerID(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return u.ID
	}
	return AnonUser
}

// --- Rate Limiting ---

// rateLimit wraps a Func with rate limiting. The bucket key is
// derived from the authenticated user when auth has populated
// the context, and from the client IP otherwise (see
// ratelimit.extractKey for the trusted-proxy rules). SEC-RL-1:
// per-user-or-IP keying so a single attacker IP cannot exhaust
// a global bucket shared across every tenant.
func (s *Server) rateLimit(next Func) Func {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.rateLimiter != nil && !s.rateLimiter.Allow(ratelimit.ExtractKey(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return nil
		}
		return next(w, r)
	}
}

// --- Auth Adapter ---

// storeAuthAdapter adapts API key persistence to auth.APIKeyStore.
type storeAuthAdapter struct {
	db store.APIKeys
}

func (a *storeAuthAdapter) GetAPIKeyByHash(ctx context.Context, keyHash string) (*auth.APIKeyRecord, error) {
	key, err := a.db.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}
	return &auth.APIKeyRecord{
		ID:        key.ID,
		UserID:    key.UserID,
		Role:      key.Role,
		KeyPrefix: key.KeyPrefix,
		ExpiresAt: key.ExpiresAt,
		Revoked:   key.Revoked,
	}, nil
}

func (a *storeAuthAdapter) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	return a.db.UpdateAPIKeyLastUsed(ctx, id)
}

// --- Auth Audit Logger ---

// authAuditLogger adapts the server's audit method to auth.logger.
type authAuditLogger struct {
	server *Server
}

func (l *authAuditLogger) LogAuthFailure(ctx context.Context, keyPrefix, reason, remoteAddr string) {
	l.server.audit(ctx, "auth_failure", "api_key", map[string]any{
		FieldKeyPref:  keyPrefix,
		"reason":      reason,
		"remote_addr": remoteAddr,
	})
}
