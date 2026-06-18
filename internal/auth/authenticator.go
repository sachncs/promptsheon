package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Authenticator validates API keys and attaches user info to requests.
type Authenticator struct {
	store      APIKeyStore
	authLogger AuthLogger
}

// AuthLogger logs authentication failures for audit purposes.
type AuthLogger interface {
	LogAuthFailure(ctx context.Context, keyPrefix, reason, remoteAddr string)
}

// noopLogger is a default that does nothing.
type noopLogger struct{}

func (n *noopLogger) LogAuthFailure(_ context.Context, _, _, _ string) {}

// APIKeyStore is the subset of the repository needed by the authenticator.
type APIKeyStore interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKeyRecord, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error
}

// APIKeyRecord is the data returned when looking up an API key.
type APIKeyRecord struct {
	ID        string
	UserID    string
	Role      string
	KeyPrefix string
	ExpiresAt *time.Time
	Revoked   bool
}

// User holds the authenticated user's identity.
type User struct {
	ID   string
	Role Role
}

// contextKey is the unexported type for context keys in this package.
type contextKey string

const userContextKey contextKey = "user"

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok
}

// WithUserContext returns a new context with the user attached.
func WithUserContext(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// NewAuthenticator creates a new Authenticator.
func NewAuthenticator(store APIKeyStore) *Authenticator {
	return &Authenticator{store: store, authLogger: &noopLogger{}}
}

// NewAuthenticatorWithLogger creates a new Authenticator with an audit logger.
func NewAuthenticatorWithLogger(store APIKeyStore, logger AuthLogger) *Authenticator {
	return &Authenticator{store: store, authLogger: logger}
}

// Authenticate extracts and validates the API key from the request.
// It returns an error if the key is missing, invalid, or expired.
func (a *Authenticator) Authenticate(r *http.Request) (*User, error) {
	key := extractAPIKey(r)
	if key == "" {
		a.authLogger.LogAuthFailure(r.Context(), "", "missing api key", r.RemoteAddr)
		return nil, fmt.Errorf("missing api key")
	}

	hash := HashAPIKey(key)
	rec, err := a.store.GetAPIKeyByHash(r.Context(), hash)
	if err != nil {
		a.authLogger.LogAuthFailure(r.Context(), key[:min(8, len(key))], "lookup error", r.RemoteAddr)
		return nil, fmt.Errorf("lookup api key: %w", err)
	}
	if rec == nil || rec.Revoked {
		prefix := ""
		if len(key) > 8 {
			prefix = key[:8]
		}
		a.authLogger.LogAuthFailure(r.Context(), prefix, "invalid or revoked", r.RemoteAddr)
		return nil, fmt.Errorf("invalid api key")
	}
	if rec.ExpiresAt != nil && rec.ExpiresAt.Before(time.Now()) {
		a.authLogger.LogAuthFailure(r.Context(), rec.KeyPrefix, "expired", r.RemoteAddr)
		return nil, fmt.Errorf("api key expired")
	}

	// Update last used (best effort, don't fail request).
	a.store.UpdateAPIKeyLastUsed(context.Background(), rec.ID) //nolint:errcheck

	return &User{
		ID:   rec.UserID,
		Role: Role(rec.Role),
	}, nil
}

// AuthenticateMiddleware returns middleware that authenticates requests and
// attaches the user to the context.
func (a *Authenticator) AuthenticateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.Authenticate(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"`+err.Error()+`"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAPIKey gets the API key from the Authorization header.
// Query parameter support is deprecated for security reasons (key appears in logs/referer).
func extractAPIKey(r *http.Request) string {
	// Check Authorization header: "Bearer ps_..."
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return ""
}
