package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Authenticator validates API keys and attaches user info to requests.
type Authenticator struct {
	store      APIKeyStore
	authLogger AuthLogger

	// lastUsedCh is a buffered channel of API key IDs whose
	// last_used_at timestamp should be updated. H-5 fix: the
	// previous implementation called UpdateAPIKeyLastUsed
	// synchronously with context.Background() and //nolint:errcheck,
	// which could stall the request goroutine indefinitely if the
	// DB was slow. The update is now fire-and-forget on a
	// background goroutine.
	lastUsedCh chan string
	wg         sync.WaitGroup
	stopCh     chan struct{}
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
	a := &Authenticator{
		store:      store,
		authLogger: &noopLogger{},
		lastUsedCh: make(chan string, 1024),
		stopCh:     make(chan struct{}),
	}
	a.wg.Add(1)
	go a.lastUsedWorker()
	return a
}

// NewAuthenticatorWithLogger creates a new Authenticator with an audit logger.
func NewAuthenticatorWithLogger(store APIKeyStore, logger AuthLogger) *Authenticator {
	a := &Authenticator{
		store:      store,
		authLogger: logger,
		lastUsedCh: make(chan string, 1024),
		stopCh:     make(chan struct{}),
	}
	a.wg.Add(1)
	go a.lastUsedWorker()
	return a
}

// lastUsedWorker drains the lastUsedCh channel and applies updates on
// a background goroutine. Errors are logged via slog and never
// bubble up to the request path. The previous design called
// UpdateAPIKeyLastUsed synchronously inside the request hot path.
func (a *Authenticator) lastUsedWorker() {
	defer a.wg.Done()
	for {
		select {
		case <-a.stopCh:
			return
		case id, ok := <-a.lastUsedCh:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := a.store.UpdateAPIKeyLastUsed(ctx, id); err != nil {
				slog.Error("auth: update last_used failed", "err", err, "api_key_id", id)
			}
			cancel()
		}
	}
}

// Stop signals the lastUsedWorker to exit and waits for it. Safe to
// call multiple times.
func (a *Authenticator) Stop() {
	select {
	case <-a.stopCh:
		return
	default:
		close(a.stopCh)
	}
	a.wg.Wait()
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

	// H-5 fix: queue the last-used update on a background
	// goroutine. The request hot path is now non-blocking — a
	// slow or stalled DB can no longer freeze the response.
	select {
	case a.lastUsedCh <- rec.ID:
	default:
		// Channel full: drop the update. The last_used column is
		// for observability, not for security, so it is safe to
		// lose an occasional write under heavy load.
	}

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
