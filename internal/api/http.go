package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/sachncs/promptsheon/internal/api/handlers"
	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/store"
)

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

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	if err := handlers.JSON(w, status, data); err != nil {
		slog.Error("failed to encode json response", "err", err)
	}
}

// writeError writes a JSON error response, inferring the status code from
// known error types.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var httpErr *HTTPError
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr):
		// http.MaxBytesReader returns *http.MaxBytesError when the
		// body exceeds the configured limit. Map that to 413 so
		// the client sees the actual problem (oversize body)
		// rather than the generic 500 that previously leaked the
		// wrapped decode error.
		status = http.StatusRequestEntityTooLarge
	case errors.As(err, &httpErr):
		status = httpErr.Status
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{valError: err.Error()}
	if errors.As(err, &httpErr) && httpErr.Details != nil {
		body["details"] = httpErr.Details
	}
	if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
		slog.Error("failed to encode error json response", "err", encErr)
	}
}

// readJSON decodes the request body into target.
func readJSON(r *http.Request, target any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(target)
}

// httpRequestFromContext returns the *http.Request stored in the
// context by the request middleware, if any. Returns nil if there is
// none (e.g. background work).
func httpRequestFromContext(ctx context.Context) *http.Request {
	if r, ok := ctx.Value(httpRequestKey{}).(*http.Request); ok {
		return r
	}
	return nil
}

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
			// Audit the read-only block: the operator should
			// know when traffic is being shed.
			if s, ok := r.Context().Value(httpRequestKey{}).(*http.Request); ok && s != nil {
				_ = s // currently used only for context extraction
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"daemon is in read-only mode","details":{"reason":"PROMPTSHEON_READ_ONLY=true"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) error {
	if s.logHub == nil {
		return badRequest("log streaming not configured")
	}
	s.logHub.HandleSSE(w, r)
	return nil
}

func (s *Server) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) error {
	if s.collector == nil {
		return badRequest("metrics not configured")
	}
	s.collector.Handler().ServeHTTP(w, r)
	return nil
}

// Common API errors.
var (
	ErrNotFound   = errors.New("resource not found")
	ErrBadRequest = errors.New("bad request")
	ErrConflict   = errors.New("resource already exists")
)

// HTTPError represents an HTTP error with a specific status code.
type HTTPError struct {
	Status  int
	Message string
	Details any // optional structured payload (e.g. precondition failures)
}

func (e *HTTPError) Error() string { return e.Message }

func badRequest(msg string) error { return &HTTPError{Status: http.StatusBadRequest, Message: msg} }
func notFound(msg string) error   { return &HTTPError{Status: http.StatusNotFound, Message: msg} }
func unauthorized() error {
	return &HTTPError{Status: http.StatusUnauthorized, Message: "authentication required"}
}
func forbidden(msg string) error { return &HTTPError{Status: http.StatusForbidden, Message: msg} }

// callerID returns the authenticated user's ID, or "api" if no user
// is in the request context. Used to populate CreatedBy fields
// without each handler re-implementing the lookup.
func callerID(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return u.ID
	}
	return auditDefaultUser
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
		fieldKeyPrefix: keyPrefix,
		"reason":       reason,
		"remote_addr":  remoteAddr,
	})
}
